// Package main provides GitHub webhook handling and Prometheus metrics collection.
package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

type GithubRepo struct {
	FullName string `json:"full_name"`
}

type GithubWorkflow struct {
	Workflow struct {
		ID         int        `json:"id"`
		Status     string     `json:"status"`
		RunID      int        `json:"run_id"`
		Name       string     `json:"name"`
		Branch     string     `json:"head_branch"`
		Repository GithubRepo `json:"repository"`
		Conclusion string     `json:"conclusion"`
		CreatedAt  string     `json:"created_at"`
		UpdatedAt  string     `json:"updated_at"`
		HTMLURL    string     `json:"html_url"`
	} `json:"workflow_run"`
}

type GithubJob struct {
	Job struct {
		ID           int        `json:"id"`
		Status       string     `json:"status"`
		Name         string     `json:"name"`
		Branch       string     `json:"head_branch"`
		Repository   GithubRepo `json:"repository"`
		RunnerName   string     `json:"runner_name"`
		Conclusion   string     `json:"conclusion"`
		StartedAt    string     `json:"started_at"`
		CompletedAt  string     `json:"completed_at"`
		WorkflowName string     `json:"workflow_name"`
		HTMLURL      string     `json:"html_url"`
	} `json:"workflow_job"`
}

type GithubCommit struct {
	Repository GithubRepo `json:"repository"`
	Commits    []struct {
		ID     string `json:"id"`
		Author struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"author"`
	} `json:"commits"`
	Ref string `json:"ref"`
}

type GithubPullRequest struct {
	Action      string `json:"action"`
	PullRequest struct {
		ID    int    `json:"id"`
		State string `json:"state"`
		Title string `json:"title"`
		Base  struct {
			Ref string `json:"ref"`
		} `json:"base"`
		Head struct {
			Ref string `json:"ref"`
		} `json:"head"`
		User struct {
			Login string `json:"login"`
			Email string `json:"email"`
		} `json:"user"`
	} `json:"pull_request"`
	Repository GithubRepo `json:"repository"`
}

type runMetricDetails struct {
	repository string
	branch     string
	name       string
	status     string
	conclusion string
	startedAt  string
	endedAt    string
}

type runMetricSet struct {
	statusCounter     *prometheus.CounterVec
	queuedGauge       *prometheus.GaugeVec
	inProgressGauge   *prometheus.GaugeVec
	completedGauge    *prometheus.GaugeVec
	durationHistogram *prometheus.HistogramVec
}

type runMetricSets struct {
	core     runMetricSet
	detailed runMetricSet
}

type runStoreMethods struct {
	get    func(context.Context, int) (RunState, bool, error)
	update func(context.Context, int, RunState) error
}

const (
	statusQueued     = "queued"
	statusInProgress = "in_progress"
	statusCompleted  = "completed"
)

var (
	stateStore     StateStore
	eventProcessor *asyncEventProcessor
)

func validateHMAC(body []byte, signature string, secret []byte) bool {
	h := hmac.New(sha256.New, secret)
	h.Write(body)
	computedSignature := "sha256=" + hex.EncodeToString(h.Sum(nil))
	return hmac.Equal([]byte(computedSignature), []byte(signature))
}

func githubEventsHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Unable to read request body", http.StatusInternalServerError)
		logger.Error("Unable to read request body", zap.Error(err))
		return
	}

	signature := r.Header.Get("X-Hub-Signature-256")
	if !validateHMAC(body, signature, githubWebhookSecret) {
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		logger.Error("Invalid signature")
		return
	}

	ctx := r.Context()
	deliveryID := strings.TrimSpace(r.Header.Get("X-GitHub-Delivery"))
	if stateStore != nil && deliveryID != "" {
		processed, storeErr := stateStore.MarkDeliveryProcessed(ctx, deliveryID)
		if storeErr != nil {
			http.Error(w, "Unable to record webhook delivery", http.StatusInternalServerError)
			logger.Error("Unable to record webhook delivery", zap.String("deliveryID", deliveryID), zap.Error(storeErr))
			return
		}
		if !processed {
			logger.Info("Skipping duplicate GitHub delivery", zap.String("deliveryID", deliveryID))
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	eventType := r.Header.Get("X-GitHub-Event")
	if eventProcessor != nil {
		if err := eventProcessor.Enqueue(ctx, eventType, body); err != nil {
			http.Error(w, "Webhook queue is full", http.StatusServiceUnavailable)
			logger.Warn("Dropping webhook event because queue is full", zap.String("eventType", eventType), zap.Error(err))
			return
		}
		w.WriteHeader(http.StatusAccepted)
		return
	}

	switch eventType {
	case "workflow_run":
		updateWorkflowMetrics(ctx, body)
	case "workflow_job":
		updateJobMetrics(ctx, body)
	case "push":
		updateCommitMetrics(body)
	case "pull_request":
		updatePullRequestMetrics(body)
	default:
		logger.Warn("Invalid GitHub event type", zap.String("eventType", eventType))
	}

	w.WriteHeader(http.StatusOK)
}

func normalizeRunState(details runMetricDetails) RunState {
	return RunState{
		Repository: details.repository,
		Branch:     details.branch,
		Name:       details.name,
		Status:     normalizeStatus(details.status),
		Conclusion: normalizeConclusion(details.conclusion),
		StartedAt:  details.startedAt,
		EndedAt:    details.endedAt,
	}
}

func normalizeStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}

func normalizeConclusion(conclusion string) string {
	return strings.ToLower(strings.TrimSpace(conclusion))
}

func stateRank(status string) int {
	switch normalizeStatus(status) {
	case statusQueued:
		return 1
	case statusInProgress:
		return 2
	case statusCompleted:
		return 3
	default:
		return 0
	}
}

func parseMetricTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, false
	}

	return parsed, true
}

func shouldApplyStateTransition(previous, next RunState) bool {
	previousRank := stateRank(previous.Status)
	nextRank := stateRank(next.Status)
	if nextRank < previousRank {
		return false
	}

	if nextRank == previousRank {
		if next.Status == previous.Status && next.Conclusion == previous.Conclusion {
			return false
		}

		previousEndedAt, previousHasEndedAt := parseMetricTime(previous.EndedAt)
		nextEndedAt, nextHasEndedAt := parseMetricTime(next.EndedAt)
		if previousHasEndedAt && nextHasEndedAt && nextEndedAt.Before(previousEndedAt) {
			return false
		}

		if previousHasEndedAt && !nextHasEndedAt {
			return false
		}
	}

	return true
}

func applyCoreGaugeDelta(details RunState, delta float64, queuedGauge, inProgressGauge, completedGauge *prometheus.GaugeVec) {
	switch normalizeStatus(details.Status) {
	case statusQueued:
		queuedGauge.WithLabelValues(details.Repository).Add(delta)
	case statusInProgress:
		inProgressGauge.WithLabelValues(details.Repository).Add(delta)
	case statusCompleted:
		completedGauge.WithLabelValues(details.Repository, details.Conclusion).Add(delta)
	}
}

func applyDetailedGaugeDelta(details RunState, delta float64, queuedGauge, inProgressGauge, completedGauge *prometheus.GaugeVec) {
	switch normalizeStatus(details.Status) {
	case statusQueued:
		queuedGauge.WithLabelValues(details.Repository, details.Branch, details.Name).Add(delta)
	case statusInProgress:
		inProgressGauge.WithLabelValues(details.Repository, details.Branch, details.Name).Add(delta)
	case statusCompleted:
		completedGauge.WithLabelValues(details.Repository, details.Branch, details.Conclusion, details.Name).Add(delta)
	}
}

func observeCoreDuration(details RunState, durationHistogram *prometheus.HistogramVec) {
	if normalizeStatus(details.Status) != statusCompleted {
		return
	}

	startedAt, startedOK := parseMetricTime(details.StartedAt)
	endedAt, endedOK := parseMetricTime(details.EndedAt)
	if !startedOK || !endedOK || endedAt.Before(startedAt) {
		return
	}

	durationHistogram.WithLabelValues(
		details.Repository,
		details.Status,
		details.Conclusion,
	).Observe(endedAt.Sub(startedAt).Seconds())
}

func observeDetailedDuration(details RunState, durationHistogram *prometheus.HistogramVec) {
	if normalizeStatus(details.Status) != statusCompleted {
		return
	}

	startedAt, startedOK := parseMetricTime(details.StartedAt)
	endedAt, endedOK := parseMetricTime(details.EndedAt)
	if !startedOK || !endedOK || endedAt.Before(startedAt) {
		return
	}

	durationHistogram.WithLabelValues(
		details.Repository,
		details.Branch,
		details.Name,
		details.Status,
		details.Conclusion,
	).Observe(endedAt.Sub(startedAt).Seconds())
}

func applyCoreStatefulMetrics(details RunState, previous *RunState, metrics runMetricSet) {
	metrics.statusCounter.WithLabelValues(
		details.Repository,
		details.Status,
		details.Conclusion,
	).Inc()

	if previous != nil {
		applyCoreGaugeDelta(*previous, -1, metrics.queuedGauge, metrics.inProgressGauge, metrics.completedGauge)
	}
	applyCoreGaugeDelta(details, 1, metrics.queuedGauge, metrics.inProgressGauge, metrics.completedGauge)

	if previous == nil || normalizeStatus(previous.Status) != statusCompleted {
		observeCoreDuration(details, metrics.durationHistogram)
	}
}

func applyDetailedStatefulMetrics(details RunState, previous *RunState, metrics runMetricSet) {
	metrics.statusCounter.WithLabelValues(
		details.Repository,
		details.Branch,
		details.Name,
		details.Status,
		details.Conclusion,
	).Inc()

	if previous != nil {
		applyDetailedGaugeDelta(*previous, -1, metrics.queuedGauge, metrics.inProgressGauge, metrics.completedGauge)
	}
	applyDetailedGaugeDelta(details, 1, metrics.queuedGauge, metrics.inProgressGauge, metrics.completedGauge)

	if previous == nil || normalizeStatus(previous.Status) != statusCompleted {
		observeDetailedDuration(details, metrics.durationHistogram)
	}
}

func getPreviousState(ctx context.Context, id int, getFn func(context.Context, int) (RunState, bool, error), entityName string) (*RunState, bool) {
	if stateStore == nil {
		return nil, true
	}

	previous, found, err := getFn(ctx, id)
	if err != nil {
		logger.Error("Failed to load run state from redis", zap.String("entity", entityName), zap.Int("id", id), zap.Error(err))
		return nil, false
	}
	if !found {
		return nil, true
	}

	return &previous, true
}

func persistRunState(ctx context.Context, id int, next RunState, updateFn func(context.Context, int, RunState) error, entityName string) bool {
	if stateStore == nil {
		return true
	}

	if err := updateFn(ctx, id, next); err != nil {
		logger.Error("Failed to update run state in redis", zap.String("entity", entityName), zap.Int("id", id), zap.Error(err))
		return false
	}

	return true
}

func updateTrackedRunMetrics(
	ctx context.Context,
	id int,
	details runMetricDetails,
	store runStoreMethods,
	entityName string,
	metrics runMetricSets,
) {
	nextState := normalizeRunState(details)

	if stateStore == nil {
		applyCoreStatefulMetrics(nextState, nil, metrics.core)
		if enableDetailedMetrics {
			applyDetailedStatefulMetrics(nextState, nil, metrics.detailed)
		}
		return
	}

	previousState, ok := getPreviousState(ctx, id, store.get, entityName)
	if !ok {
		return
	}
	if previousState != nil && !shouldApplyStateTransition(*previousState, nextState) {
		logger.Debug("Skipping stale or duplicate run transition", zap.String("entity", entityName), zap.Int("id", id), zap.String("status", nextState.Status), zap.String("conclusion", nextState.Conclusion))
		return
	}
	if !persistRunState(ctx, id, nextState, store.update, entityName) {
		return
	}

	applyCoreStatefulMetrics(nextState, previousState, metrics.core)
	if enableDetailedMetrics {
		applyDetailedStatefulMetrics(nextState, previousState, metrics.detailed)
	}
}

func workflowRunStoreMethods() runStoreMethods {
	return runStoreMethods{
		get: func(ctx context.Context, id int) (RunState, bool, error) {
			return stateStore.GetWorkflowRun(ctx, id)
		},
		update: func(ctx context.Context, id int, state RunState) error {
			return stateStore.UpdateWorkflowRun(ctx, id, state)
		},
	}
}

func workflowJobStoreMethods() runStoreMethods {
	return runStoreMethods{
		get: func(ctx context.Context, id int) (RunState, bool, error) {
			return stateStore.GetWorkflowJob(ctx, id)
		},
		update: func(ctx context.Context, id int, state RunState) error {
			return stateStore.UpdateWorkflowJob(ctx, id, state)
		},
	}
}

func updateWorkflowMetrics(ctx context.Context, body []byte) {
	var payload GithubWorkflow

	if err := json.Unmarshal(body, &payload); err != nil {
		logger.Error("Failed to unmarshal workflow_run payload", zap.Error(err))
		return
	}

	updateTrackedRunMetrics(
		ctx,
		payload.Workflow.RunID,
		runMetricDetails{
			repository: payload.Workflow.Repository.FullName,
			branch:     payload.Workflow.Branch,
			name:       payload.Workflow.Name,
			status:     payload.Workflow.Status,
			conclusion: payload.Workflow.Conclusion,
			startedAt:  payload.Workflow.CreatedAt,
			endedAt:    payload.Workflow.UpdatedAt,
		},
		workflowRunStoreMethods(),
		"workflow_run",
		runMetricSets{
			core: runMetricSet{
				statusCounter:     workflowStatusCounter,
				queuedGauge:       workflowQueuedGauge,
				inProgressGauge:   workflowInProgressGauge,
				completedGauge:    workflowCompletedGauge,
				durationHistogram: workflowDurationHistogram,
			},
			detailed: runMetricSet{
				statusCounter:     workflowStatusDetailedCounter,
				queuedGauge:       workflowQueuedDetailedGauge,
				inProgressGauge:   workflowInProgressDetailedGauge,
				completedGauge:    workflowCompletedDetailedGauge,
				durationHistogram: workflowDurationDetailedHistogram,
			},
		},
	)
}

func updateJobMetrics(ctx context.Context, body []byte) {
	var payload GithubJob

	if err := json.Unmarshal(body, &payload); err != nil {
		logger.Error("Failed to unmarshal workflow_job payload", zap.Error(err))
		return
	}

	updateTrackedRunMetrics(
		ctx,
		payload.Job.ID,
		runMetricDetails{
			repository: payload.Job.Repository.FullName,
			branch:     payload.Job.Branch,
			name:       payload.Job.WorkflowName,
			status:     payload.Job.Status,
			conclusion: payload.Job.Conclusion,
			startedAt:  payload.Job.StartedAt,
			endedAt:    payload.Job.CompletedAt,
		},
		workflowJobStoreMethods(),
		"workflow_job",
		runMetricSets{
			core: runMetricSet{
				statusCounter:     jobStatusCounter,
				queuedGauge:       jobQueuedGauge,
				inProgressGauge:   jobInProgressGauge,
				completedGauge:    jobCompletedGauge,
				durationHistogram: jobDurationHistogram,
			},
			detailed: runMetricSet{
				statusCounter:     jobStatusDetailedCounter,
				queuedGauge:       jobQueuedDetailedGauge,
				inProgressGauge:   jobInProgressDetailedGauge,
				completedGauge:    jobCompletedDetailedGauge,
				durationHistogram: jobDurationDetailedHistogram,
			},
		},
	)
}

func updateCommitMetrics(body []byte) {
	var payload GithubCommit

	if err := json.Unmarshal(body, &payload); err != nil {
		logger.Error("Failed to unmarshal push payload", zap.Error(err))
		return
	}

	for range payload.Commits {
		commitPushedCounter.WithLabelValues(payload.Repository.FullName).Inc()
	}
}

func updatePullRequestMetrics(body []byte) {
	var payload GithubPullRequest

	if err := json.Unmarshal(body, &payload); err != nil {
		logger.Error("Failed to unmarshal pull_request payload", zap.Error(err))
		return
	}

	pullRequestCounter.WithLabelValues(
		payload.Repository.FullName,
		payload.Action,
	).Inc()

	if enableDetailedMetrics {
		pullRequestDetailedCounter.WithLabelValues(
			payload.Repository.FullName,
			payload.PullRequest.Base.Ref,
			payload.Action,
		).Inc()
	}
}
