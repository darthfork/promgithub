// Package main provides GitHub webhook handling and Prometheus metrics collection.
package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

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

const (
	statusQueued     = "queued"
	statusInProgress = "in_progress"
	statusCompleted  = "completed"
)

var (
	deliveryStateStore    deliveryStateBackend
	workflowRunStateStore workflowRunStateBackend
	workflowJobStateStore workflowJobStateBackend
	eventProcessor        *asyncEventProcessor
)

func validateHMAC(body []byte, signature string, secret []byte) bool {
	h := hmac.New(sha256.New, secret)
	h.Write(body)
	computedSignature := "sha256=" + hex.EncodeToString(h.Sum(nil))
	return hmac.Equal([]byte(computedSignature), []byte(signature))
}

var deliveryDeduperCache = newDeliveryDeduper(defaultDeliveryRetention, defaultDeliveryCacheEntries)

func githubEventsHandler(w http.ResponseWriter, r *http.Request) {
	webhookHTTPHandler(newDefaultWebhookIngestion(), logger).ServeHTTP(w, r)
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

func updateTrackedRunMetrics(
	ctx context.Context,
	id int,
	details runMetricDetails,
	store runTransitionStore,
	entityName string,
	metricKind runMetricKind,
) {
	processor := &runTransitionProcessor{
		store:      store,
		recorder:   metricRunTransitionRecorder{kind: metricKind, recorder: defaultMetricRecorder},
		logger:     logger,
		entityName: entityName,
	}
	processor.Apply(ctx, id, details)
}

func workflowRunTransitionStore() runTransitionStore {
	if workflowRunStateStore == nil {
		return nil
	}

	return workflowRunStateAdapter{store: workflowRunStateStore}
}

func workflowJobTransitionStore() runTransitionStore {
	if workflowJobStateStore == nil {
		return nil
	}

	return workflowJobStateAdapter{store: workflowJobStateStore}
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
		workflowRunTransitionStore(),
		githubEventWorkflowRun,
		runMetricKindWorkflow,
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
		workflowJobTransitionStore(),
		githubEventWorkflowJob,
		runMetricKindJob,
	)
}

func updateCommitMetrics(body []byte) {
	var payload GithubCommit

	if err := json.Unmarshal(body, &payload); err != nil {
		logger.Error("Failed to unmarshal push payload", zap.Error(err))
		return
	}

	for range payload.Commits {
		defaultMetricRecorder.RecordCommitPushed(payload.Repository.FullName)
	}
}

func updatePullRequestMetrics(body []byte) {
	var payload GithubPullRequest

	if err := json.Unmarshal(body, &payload); err != nil {
		logger.Error("Failed to unmarshal pull_request payload", zap.Error(err))
		return
	}

	defaultMetricRecorder.RecordPullRequest(
		payload.Repository.FullName,
		payload.PullRequest.Base.Ref,
		payload.Action,
	)
}
