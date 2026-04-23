// Package main provides GitHub webhook handling and Prometheus metrics collection.
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
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

	eventType := r.Header.Get("X-GitHub-Event")
	switch eventType {
	case "workflow_run":
		updateWorkflowMetrics(body)
	case "workflow_job":
		updateJobMetrics(body)
	case "push":
		updateCommitMetrics(body)
	case "pull_request":
		updatePullRequestMetrics(body)
	default:
		logger.Warn("Invalid GitHub event type", zap.String("eventType", eventType))
	}

	w.WriteHeader(http.StatusOK)
}

func updateWorkflowMetrics(body []byte) {
	var payload GithubWorkflow

	if err := json.Unmarshal(body, &payload); err != nil {
		logger.Error("Failed to unmarshal workflow_run payload", zap.Error(err))
		return
	}

	workflowStatusCounter.WithLabelValues(
		payload.Workflow.Repository.FullName,
		payload.Workflow.Name,
		payload.Workflow.Status,
		payload.Workflow.Conclusion,
	).Inc()

	switch strings.ToLower(payload.Workflow.Status) {
	case "queued":
		workflowQueuedGauge.WithLabelValues(
			payload.Workflow.Repository.FullName,
			payload.Workflow.Name,
		).Inc()
	case "in_progress":
		workflowInProgressGauge.WithLabelValues(
			payload.Workflow.Repository.FullName,
			payload.Workflow.Name,
		).Inc()
		workflowQueuedGauge.WithLabelValues(
			payload.Workflow.Repository.FullName,
			payload.Workflow.Name,
		).Dec()
	case "completed":
		workflowCompletedGauge.WithLabelValues(
			payload.Workflow.Repository.FullName,
			payload.Workflow.Conclusion,
			payload.Workflow.Name,
		).Inc()
		workflowInProgressGauge.WithLabelValues(
			payload.Workflow.Repository.FullName,
			payload.Workflow.Name,
		).Dec()

		createdAt, err1 := time.Parse(time.RFC3339, payload.Workflow.CreatedAt)
		updatedAt, err2 := time.Parse(time.RFC3339, payload.Workflow.UpdatedAt)
		if err1 == nil && err2 == nil {
			duration := updatedAt.Sub(createdAt).Seconds()
			workflowDurationHistogram.WithLabelValues(
				payload.Workflow.Repository.FullName,
				payload.Workflow.Name,
				payload.Workflow.Status,
				payload.Workflow.Conclusion,
			).Observe(duration)
		}
	}
}

func updateJobMetrics(body []byte) {
	var payload GithubJob

	if err := json.Unmarshal(body, &payload); err != nil {
		logger.Error("Failed to unmarshal workflow_job payload", zap.Error(err))
		return
	}

	jobStatusCounter.WithLabelValues(
		payload.Job.Repository.FullName,
		payload.Job.WorkflowName,
		payload.Job.Status,
		payload.Job.Conclusion,
	).Inc()

	switch strings.ToLower(payload.Job.Status) {
	case "queued":
		jobQueuedGauge.WithLabelValues(
			payload.Job.Repository.FullName,
			payload.Job.WorkflowName,
		).Inc()
	case "in_progress":
		jobInProgressGauge.WithLabelValues(
			payload.Job.Repository.FullName,
			payload.Job.WorkflowName,
		).Inc()
		jobQueuedGauge.WithLabelValues(
			payload.Job.Repository.FullName,
			payload.Job.WorkflowName,
		).Dec()
	case "completed":
		jobCompletedGauge.WithLabelValues(
			payload.Job.Repository.FullName,
			payload.Job.Conclusion,
			payload.Job.WorkflowName,
		).Inc()
		jobInProgressGauge.WithLabelValues(
			payload.Job.Repository.FullName,
			payload.Job.WorkflowName,
		).Dec()

		startedAt, err1 := time.Parse(time.RFC3339, payload.Job.StartedAt)
		completedAt, err2 := time.Parse(time.RFC3339, payload.Job.CompletedAt)
		if err1 == nil && err2 == nil {
			duration := completedAt.Sub(startedAt).Seconds()
			jobDurationHistogram.WithLabelValues(
				payload.Job.Repository.FullName,
				payload.Job.WorkflowName,
				payload.Job.Status,
				payload.Job.Conclusion,
			).Observe(duration)
		}
	}
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
