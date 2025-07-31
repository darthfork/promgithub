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

	workflowStatusCounter.With(prometheus.Labels{
		"repository":      payload.Workflow.Repository.FullName,
		"branch":          payload.Workflow.Branch,
		"workflow_name":   payload.Workflow.Name,
		"workflow_status": payload.Workflow.Status,
		"conclusion":      payload.Workflow.Conclusion,
	}).Inc()

	// Handle updating the gauges based on workflow status
	switch strings.ToLower(payload.Workflow.Status) {
	case "queued":
		workflowQueuedGauge.With(prometheus.Labels{
			"repository":    payload.Workflow.Repository.FullName,
			"branch":        payload.Workflow.Branch,
			"workflow_name": payload.Workflow.Name,
		}).Inc()
	case "in_progress":
		workflowInProgressGauge.With(prometheus.Labels{
			"repository":    payload.Workflow.Repository.FullName,
			"branch":        payload.Workflow.Branch,
			"workflow_name": payload.Workflow.Name,
		}).Inc()
		workflowQueuedGauge.With(prometheus.Labels{
			"repository":    payload.Workflow.Repository.FullName,
			"branch":        payload.Workflow.Branch,
			"workflow_name": payload.Workflow.Name,
		}).Dec()
	case "completed":
		workflowCompletedGauge.With(prometheus.Labels{
			"repository":          payload.Workflow.Repository.FullName,
			"branch":              payload.Workflow.Branch,
			"workflow_conclusion": payload.Workflow.Conclusion,
			"workflow_name":       payload.Workflow.Name,
		}).Inc()
		workflowInProgressGauge.With(prometheus.Labels{
			"repository":    payload.Workflow.Repository.FullName,
			"branch":        payload.Workflow.Branch,
			"workflow_name": payload.Workflow.Name,
		}).Dec()

		// Update duration histogram when the workflow is completed
		createdAt, err1 := time.Parse(time.RFC3339, payload.Workflow.CreatedAt)
		updatedAt, err2 := time.Parse(time.RFC3339, payload.Workflow.UpdatedAt)
		if err1 == nil && err2 == nil {
			duration := updatedAt.Sub(createdAt).Seconds()
			workflowDurationHistogram.With(prometheus.Labels{
				"repository":      payload.Workflow.Repository.FullName,
				"branch":          payload.Workflow.Branch,
				"workflow_name":   payload.Workflow.Name,
				"workflow_status": payload.Workflow.Status,
				"conclusion":      payload.Workflow.Conclusion,
			}).Observe(duration)
		}
	}
}

func updateJobMetrics(body []byte) {

	var payload GithubJob

	if err := json.Unmarshal(body, &payload); err != nil {
		logger.Error("Failed to unmarshal workflow_job payload", zap.Error(err))
		return
	}

	jobStatusCounter.With(prometheus.Labels{
		"runner":         payload.Job.RunnerName,
		"repository":     payload.Job.Repository.FullName,
		"branch":         payload.Job.Branch,
		"workflow_name":  payload.Job.WorkflowName,
		"job_name":       payload.Job.Name,
		"job_status":     payload.Job.Status,
		"job_conclusion": payload.Job.Conclusion,
	}).Inc()

	// Handle updating the gauges based on job status
	switch strings.ToLower(payload.Job.Status) {
	case "queued":
		jobQueuedGauge.With(prometheus.Labels{
			"runner":        payload.Job.RunnerName,
			"repository":    payload.Job.Repository.FullName,
			"branch":        payload.Job.Branch,
			"workflow_name": payload.Job.WorkflowName,
			"job_name":      payload.Job.Name,
		}).Inc()
	case "in_progress":
		jobInProgressGauge.With(prometheus.Labels{
			"runner":        payload.Job.RunnerName,
			"repository":    payload.Job.Repository.FullName,
			"branch":        payload.Job.Branch,
			"workflow_name": payload.Job.WorkflowName,
			"job_name":      payload.Job.Name,
		}).Inc()
		jobQueuedGauge.With(prometheus.Labels{
			"runner":        payload.Job.RunnerName,
			"repository":    payload.Job.Repository.FullName,
			"branch":        payload.Job.Branch,
			"workflow_name": payload.Job.WorkflowName,
			"job_name":      payload.Job.Name,
		}).Dec()
	case "completed":
		jobCompletedGauge.With(prometheus.Labels{
			"runner":         payload.Job.RunnerName,
			"repository":     payload.Job.Repository.FullName,
			"branch":         payload.Job.Branch,
			"job_conclusion": payload.Job.Conclusion,
			"workflow_name":  payload.Job.WorkflowName,
			"job_name":       payload.Job.Name,
		}).Inc()
		jobInProgressGauge.With(prometheus.Labels{
			"runner":        payload.Job.RunnerName,
			"repository":    payload.Job.Repository.FullName,
			"branch":        payload.Job.Branch,
			"workflow_name": payload.Job.WorkflowName,
			"job_name":      payload.Job.Name,
		}).Dec()

		// Update duration histogram when the job is completed
		startedAt, err1 := time.Parse(time.RFC3339, payload.Job.StartedAt)
		completedAt, err2 := time.Parse(time.RFC3339, payload.Job.CompletedAt)
		if err1 == nil && err2 == nil {
			duration := completedAt.Sub(startedAt).Seconds()
			jobDurationHistogram.With(prometheus.Labels{
				"runner":         payload.Job.RunnerName,
				"repository":     payload.Job.Repository.FullName,
				"branch":         payload.Job.Branch,
				"workflow_name":  payload.Job.WorkflowName,
				"job_name":       payload.Job.Name,
				"job_status":     payload.Job.Status,
				"job_conclusion": payload.Job.Conclusion,
			}).Observe(duration)
		}
	}
}

func updateCommitMetrics(body []byte) {

	var payload GithubCommit

	if err := json.Unmarshal(body, &payload); err != nil {
		logger.Error("Failed to unmarshal push payload", zap.Error(err))
		return
	}

	for _, commit := range payload.Commits {
		commitPushedCounter.With(prometheus.Labels{
			"repository":          payload.Repository.FullName,
			"branch":              payload.Ref,
			"commit_author":       commit.Author.Name,
			"commit_author_email": commit.Author.Email,
		}).Inc()
	}
}

func updatePullRequestMetrics(body []byte) {

	var payload GithubPullRequest

	if err := json.Unmarshal(body, &payload); err != nil {
		logger.Error("Failed to unmarshal pull_request payload", zap.Error(err))
		return
	}

	pullRequestCounter.With(prometheus.Labels{
		"repository":          payload.Repository.FullName,
		"base_branch":         payload.PullRequest.Base.Ref,
		"pull_request_author": payload.PullRequest.User.Login,
		"pull_request_status": payload.Action,
	}).Inc()
}
