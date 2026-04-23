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

var stateStore StateStore

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

func observeRunMetrics(
	details runMetricDetails,
	statusCounter *prometheus.CounterVec,
	queuedGauge *prometheus.GaugeVec,
	inProgressGauge *prometheus.GaugeVec,
	completedGauge *prometheus.GaugeVec,
	durationHistogram *prometheus.HistogramVec,
) {
	statusCounter.WithLabelValues(
		details.repository,
		details.branch,
		details.name,
		details.status,
		details.conclusion,
	).Inc()

	switch strings.ToLower(details.status) {
	case "queued":
		queuedGauge.WithLabelValues(
			details.repository,
			details.branch,
			details.name,
		).Inc()
	case "in_progress":
		inProgressGauge.WithLabelValues(
			details.repository,
			details.branch,
			details.name,
		).Inc()
		queuedGauge.WithLabelValues(
			details.repository,
			details.branch,
			details.name,
		).Dec()
	case "completed":
		completedGauge.WithLabelValues(
			details.repository,
			details.branch,
			details.conclusion,
			details.name,
		).Inc()
		inProgressGauge.WithLabelValues(
			details.repository,
			details.branch,
			details.name,
		).Dec()

		startedAt, err1 := time.Parse(time.RFC3339, details.startedAt)
		endedAt, err2 := time.Parse(time.RFC3339, details.endedAt)
		if err1 == nil && err2 == nil {
			duration := endedAt.Sub(startedAt).Seconds()
			durationHistogram.WithLabelValues(
				details.repository,
				details.branch,
				details.name,
				details.status,
				details.conclusion,
			).Observe(duration)
		}
	}
}

func updateRunState(ctx context.Context, id int, details runMetricDetails, updateFn func(context.Context, int, RunState) error, entityName string) {
	if stateStore == nil {
		return
	}

	if err := updateFn(ctx, id, RunState{
		Repository: details.repository,
		Branch:     details.branch,
		Name:       details.name,
		Status:     details.status,
		Conclusion: details.conclusion,
		StartedAt:  details.startedAt,
		EndedAt:    details.endedAt,
	}); err != nil {
		logger.Error("Failed to update run state in redis", zap.String("entity", entityName), zap.Int("id", id), zap.Error(err))
	}
}

func updateWorkflowMetrics(ctx context.Context, body []byte) {
	var payload GithubWorkflow

	if err := json.Unmarshal(body, &payload); err != nil {
		logger.Error("Failed to unmarshal workflow_run payload", zap.Error(err))
		return
	}

	details := runMetricDetails{
		repository: payload.Workflow.Repository.FullName,
		branch:     payload.Workflow.Branch,
		name:       payload.Workflow.Name,
		status:     payload.Workflow.Status,
		conclusion: payload.Workflow.Conclusion,
		startedAt:  payload.Workflow.CreatedAt,
		endedAt:    payload.Workflow.UpdatedAt,
	}

	if stateStore != nil {
		updateRunState(ctx, payload.Workflow.RunID, details, stateStore.UpdateWorkflowRun, "workflow_run")
	}

	observeRunMetrics(
		details,
		workflowStatusCounter,
		workflowQueuedGauge,
		workflowInProgressGauge,
		workflowCompletedGauge,
		workflowDurationHistogram,
	)
}

func updateJobMetrics(ctx context.Context, body []byte) {
	var payload GithubJob

	if err := json.Unmarshal(body, &payload); err != nil {
		logger.Error("Failed to unmarshal workflow_job payload", zap.Error(err))
		return
	}

	details := runMetricDetails{
		repository: payload.Job.Repository.FullName,
		branch:     payload.Job.Branch,
		name:       payload.Job.WorkflowName,
		status:     payload.Job.Status,
		conclusion: payload.Job.Conclusion,
		startedAt:  payload.Job.StartedAt,
		endedAt:    payload.Job.CompletedAt,
	}

	if stateStore != nil {
		updateRunState(ctx, payload.Job.ID, details, stateStore.UpdateWorkflowJob, "workflow_job")
	}

	observeRunMetrics(
		details,
		jobStatusCounter,
		jobQueuedGauge,
		jobInProgressGauge,
		jobCompletedGauge,
		jobDurationHistogram,
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
		payload.PullRequest.Base.Ref,
		payload.Action,
	).Inc()
}
