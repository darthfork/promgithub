package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func withInMemoryStateStore(t *testing.T) {
	oldStore := stateStore
	stateStore = newInMemoryStateStore()
	t.Cleanup(func() { stateStore = oldStore })
}

func TestWorkflowStatusCounter(t *testing.T) {
	withInMemoryStateStore(t)
	workflowStatusCounter.Reset()
	reg.MustRegister(workflowStatusCounter)
	body, err := os.ReadFile("../test_data/workflow_run.json")
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}
	updateWorkflowMetrics(context.Background(), body)

	if err := testutil.CollectAndCompare(workflowStatusCounter, strings.NewReader(`
		# HELP promgithub_workflow_status Total number of workflow runs with status
		# TYPE promgithub_workflow_status counter
		promgithub_workflow_status{branch="main",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed"} 1
	`)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestJobStatusCounter(t *testing.T) {
	withInMemoryStateStore(t)
	jobStatusCounter.Reset()
	reg.MustRegister(jobStatusCounter)
	body, err := os.ReadFile("../test_data/workflow_job.json")
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}
	updateJobMetrics(context.Background(), body)

	if err := testutil.CollectAndCompare(jobStatusCounter, strings.NewReader(`
        # HELP promgithub_job_status Total number of jobs with status
        # TYPE promgithub_job_status counter
        promgithub_job_status{branch="main",job_conclusion="success",job_status="completed",repository="user/repo",workflow_name="CI"} 1
    `)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestCommitsPushedCounter(t *testing.T) {
	commitPushedCounter.Reset()
	reg.MustRegister(commitPushedCounter)
	body, err := os.ReadFile("../test_data/push.json")
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}
	updateCommitMetrics(body)

	if err := testutil.CollectAndCompare(commitPushedCounter, strings.NewReader(`
		# HELP promgithub_commit_pushed Total number of commits pushed
		# TYPE promgithub_commit_pushed counter
		promgithub_commit_pushed{repository="user/repo"} 1
	`)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestPullRequestsCounter(t *testing.T) {
	pullRequestCounter.Reset()
	reg.MustRegister(pullRequestCounter)
	body, err := os.ReadFile("../test_data/pull_request.json")
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}
	updatePullRequestMetrics(body)

	if err := testutil.CollectAndCompare(pullRequestCounter, strings.NewReader(`
		# HELP promgithub_pull_request Total number of pull requests
		# TYPE promgithub_pull_request counter
		promgithub_pull_request{base_branch="main",pull_request_status="opened",repository="user/repo"} 1
	`)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestWorkflowDurationHistogram(t *testing.T) {
	withInMemoryStateStore(t)
	workflowDurationHistogram.Reset()
	reg.MustRegister(workflowDurationHistogram)
	body, err := os.ReadFile("../test_data/workflow_run.json")
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}
	updateWorkflowMetrics(context.Background(), body)

	if err := testutil.CollectAndCompare(workflowDurationHistogram, strings.NewReader(`
		# HELP promgithub_workflow_duration Duration of workflow runs
		# TYPE promgithub_workflow_duration histogram
		promgithub_workflow_duration_bucket{branch="main",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed",le="0.005"} 0
		promgithub_workflow_duration_bucket{branch="main",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed",le="0.01"} 0
		promgithub_workflow_duration_bucket{branch="main",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed",le="0.025"} 0
		promgithub_workflow_duration_bucket{branch="main",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed",le="0.05"} 0
		promgithub_workflow_duration_bucket{branch="main",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed",le="0.1"} 0
		promgithub_workflow_duration_bucket{branch="main",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed",le="0.25"} 0
		promgithub_workflow_duration_bucket{branch="main",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed",le="0.5"} 0
		promgithub_workflow_duration_bucket{branch="main",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed",le="1"} 0
		promgithub_workflow_duration_bucket{branch="main",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed",le="2.5"} 0
		promgithub_workflow_duration_bucket{branch="main",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed",le="5"} 0
		promgithub_workflow_duration_bucket{branch="main",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed",le="10"} 0
		promgithub_workflow_duration_bucket{branch="main",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed",le="+Inf"} 1
		promgithub_workflow_duration_sum{branch="main",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed"} 3600
		promgithub_workflow_duration_count{branch="main",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed"} 1
	`)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestJobDurationHistogram(t *testing.T) {
	withInMemoryStateStore(t)
	jobDurationHistogram.Reset()
	reg.MustRegister(jobDurationHistogram)
	body, err := os.ReadFile("../test_data/workflow_job.json")
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}
	updateJobMetrics(context.Background(), body)

	if err := testutil.CollectAndCompare(jobDurationHistogram, strings.NewReader(`
        # HELP promgithub_job_duration Duration of jobs runs in seconds
        # TYPE promgithub_job_duration histogram
        promgithub_job_duration_bucket{branch="main",job_conclusion="success",job_status="completed",repository="user/repo",workflow_name="CI",le="0.005"} 0
        promgithub_job_duration_bucket{branch="main",job_conclusion="success",job_status="completed",repository="user/repo",workflow_name="CI",le="0.01"} 0
        promgithub_job_duration_bucket{branch="main",job_conclusion="success",job_status="completed",repository="user/repo",workflow_name="CI",le="0.025"} 0
        promgithub_job_duration_bucket{branch="main",job_conclusion="success",job_status="completed",repository="user/repo",workflow_name="CI",le="0.05"} 0
        promgithub_job_duration_bucket{branch="main",job_conclusion="success",job_status="completed",repository="user/repo",workflow_name="CI",le="0.1"} 0
        promgithub_job_duration_bucket{branch="main",job_conclusion="success",job_status="completed",repository="user/repo",workflow_name="CI",le="0.25"} 0
        promgithub_job_duration_bucket{branch="main",job_conclusion="success",job_status="completed",repository="user/repo",workflow_name="CI",le="0.5"} 0
        promgithub_job_duration_bucket{branch="main",job_conclusion="success",job_status="completed",repository="user/repo",workflow_name="CI",le="1"} 0
        promgithub_job_duration_bucket{branch="main",job_conclusion="success",job_status="completed",repository="user/repo",workflow_name="CI",le="2.5"} 0
        promgithub_job_duration_bucket{branch="main",job_conclusion="success",job_status="completed",repository="user/repo",workflow_name="CI",le="5"} 0
        promgithub_job_duration_bucket{branch="main",job_conclusion="success",job_status="completed",repository="user/repo",workflow_name="CI",le="10"} 0
        promgithub_job_duration_bucket{branch="main",job_conclusion="success",job_status="completed",repository="user/repo",workflow_name="CI",le="+Inf"} 1
        promgithub_job_duration_sum{branch="main",job_conclusion="success",job_status="completed",repository="user/repo",workflow_name="CI"} 3600
        promgithub_job_duration_count{branch="main",job_conclusion="success",job_status="completed",repository="user/repo",workflow_name="CI"} 1
    `)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestWorkflowQueuedGauge(t *testing.T) {
	withInMemoryStateStore(t)
	workflowQueuedGauge.Reset()
	reg.MustRegister(workflowQueuedGauge)
	body, err := os.ReadFile("../test_data/workflow_run.json")
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}

	var payload GithubWorkflow
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("Failed to unmarshal JSON data: %v", err)
	}
	payload.Workflow.Status = statusQueued
	payload.Workflow.Conclusion = ""

	modifiedBody, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal modified JSON data: %v", err)
	}

	updateWorkflowMetrics(context.Background(), modifiedBody)

	if err := testutil.CollectAndCompare(workflowQueuedGauge, strings.NewReader(`
		# HELP promgithub_workflow_queued Number of workflow runs queued
		# TYPE promgithub_workflow_queued gauge
		promgithub_workflow_queued{branch="main",repository="user/repo",workflow_name="CI"} 1
	`)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestWorkflowInProgressGauge(t *testing.T) {
	withInMemoryStateStore(t)
	workflowInProgressGauge.Reset()
	reg.MustRegister(workflowInProgressGauge)
	body, err := os.ReadFile("../test_data/workflow_run.json")
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}

	var payload GithubWorkflow
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("Failed to unmarshal JSON data: %v", err)
	}
	payload.Workflow.Status = statusInProgress
	payload.Workflow.Conclusion = ""
	payload.Workflow.UpdatedAt = ""

	modifiedBody, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal modified JSON data: %v", err)
	}

	updateWorkflowMetrics(context.Background(), modifiedBody)

	if err := testutil.CollectAndCompare(workflowInProgressGauge, strings.NewReader(`
		# HELP promgithub_workflow_in_progress Number of workflow runs in progress
		# TYPE promgithub_workflow_in_progress gauge
		promgithub_workflow_in_progress{branch="main",repository="user/repo",workflow_name="CI"} 1
	`)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestWorkflowCompletedGauge(t *testing.T) {
	withInMemoryStateStore(t)
	workflowCompletedGauge.Reset()
	reg.MustRegister(workflowCompletedGauge)
	body, err := os.ReadFile("../test_data/workflow_run.json")
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}
	updateWorkflowMetrics(context.Background(), body)

	if err := testutil.CollectAndCompare(workflowCompletedGauge, strings.NewReader(`
		# HELP promgithub_workflow_completed Number of workflow runs completed
		# TYPE promgithub_workflow_completed gauge
		promgithub_workflow_completed{branch="main",repository="user/repo",workflow_conclusion="success",workflow_name="CI"} 1
	`)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestJobQueuedGauge(t *testing.T) {
	withInMemoryStateStore(t)
	jobQueuedGauge.Reset()
	reg.MustRegister(jobQueuedGauge)
	body, err := os.ReadFile("../test_data/workflow_job.json")
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}

	var payload GithubJob
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("Failed to unmarshal JSON data: %v", err)
	}
	payload.Job.Status = statusQueued
	payload.Job.Conclusion = ""
	payload.Job.CompletedAt = ""

	modifiedBody, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal modified JSON data: %v", err)
	}

	updateJobMetrics(context.Background(), modifiedBody)

	if err := testutil.CollectAndCompare(jobQueuedGauge, strings.NewReader(`
		# HELP promgithub_job_queued Number of jobs queued
		# TYPE promgithub_job_queued gauge
		promgithub_job_queued{branch="main",repository="user/repo",workflow_name="CI"} 1
	`)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestJobInProgressGauge(t *testing.T) {
	withInMemoryStateStore(t)
	jobInProgressGauge.Reset()
	reg.MustRegister(jobInProgressGauge)
	body, err := os.ReadFile("../test_data/workflow_job.json")
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}

	var payload GithubJob
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("Failed to unmarshal JSON data: %v", err)
	}
	payload.Job.Status = statusInProgress
	payload.Job.Conclusion = ""
	payload.Job.CompletedAt = ""

	modifiedBody, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal modified JSON data: %v", err)
	}

	updateJobMetrics(context.Background(), modifiedBody)

	if err := testutil.CollectAndCompare(jobInProgressGauge, strings.NewReader(`
		# HELP promgithub_job_in_progress Number of jobs in progress
		# TYPE promgithub_job_in_progress gauge
		promgithub_job_in_progress{branch="main",repository="user/repo",workflow_name="CI"} 1
	`)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestJobCompletedGauge(t *testing.T) {
	withInMemoryStateStore(t)
	jobCompletedGauge.Reset()
	reg.MustRegister(jobCompletedGauge)
	body, err := os.ReadFile("../test_data/workflow_job.json")
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}
	updateJobMetrics(context.Background(), body)

	if err := testutil.CollectAndCompare(jobCompletedGauge, strings.NewReader(`
		# HELP promgithub_job_completed Number of jobs completed
		# TYPE promgithub_job_completed gauge
		promgithub_job_completed{branch="main",job_conclusion="success",repository="user/repo",workflow_name="CI"} 1
	`)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestWorkflowGaugeTransitionIsIdempotent(t *testing.T) {
	withInMemoryStateStore(t)
	workflowQueuedGauge.Reset()
	workflowInProgressGauge.Reset()
	workflowCompletedGauge.Reset()

	body, err := os.ReadFile("../test_data/workflow_run.json")
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}

	var payload GithubWorkflow
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("Failed to unmarshal JSON data: %v", err)
	}

	payload.Workflow.Status = statusQueued
	payload.Workflow.Conclusion = ""
	payload.Workflow.UpdatedAt = payload.Workflow.CreatedAt
	queuedBody, _ := json.Marshal(payload)
	updateWorkflowMetrics(context.Background(), queuedBody)
	updateWorkflowMetrics(context.Background(), queuedBody)

	payload.Workflow.Status = statusInProgress
	payload.Workflow.UpdatedAt = "2024-11-21T11:30:00Z"
	inProgressBody, _ := json.Marshal(payload)
	updateWorkflowMetrics(context.Background(), inProgressBody)

	payload.Workflow.Status = statusCompleted
	payload.Workflow.Conclusion = "success"
	payload.Workflow.UpdatedAt = "2024-11-21T12:00:00Z"
	completedBody, _ := json.Marshal(payload)
	updateWorkflowMetrics(context.Background(), completedBody)
	updateWorkflowMetrics(context.Background(), inProgressBody)

	if got := testutil.ToFloat64(workflowQueuedGauge.WithLabelValues("user/repo", "main", "CI")); got != 0 {
		t.Fatalf("expected queued gauge to be 0, got %v", got)
	}
	if got := testutil.ToFloat64(workflowInProgressGauge.WithLabelValues("user/repo", "main", "CI")); got != 0 {
		t.Fatalf("expected in progress gauge to be 0, got %v", got)
	}
	if got := testutil.ToFloat64(workflowCompletedGauge.WithLabelValues("user/repo", "main", "success", "CI")); got != 1 {
		t.Fatalf("expected completed gauge to be 1, got %v", got)
	}
}

func TestJobGaugeTransitionIsIdempotent(t *testing.T) {
	withInMemoryStateStore(t)
	jobQueuedGauge.Reset()
	jobInProgressGauge.Reset()
	jobCompletedGauge.Reset()

	body, err := os.ReadFile("../test_data/workflow_job.json")
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}

	var payload GithubJob
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("Failed to unmarshal JSON data: %v", err)
	}

	payload.Job.Status = statusQueued
	payload.Job.Conclusion = ""
	payload.Job.StartedAt = ""
	payload.Job.CompletedAt = ""
	queuedBody, _ := json.Marshal(payload)
	updateJobMetrics(context.Background(), queuedBody)
	updateJobMetrics(context.Background(), queuedBody)

	payload.Job.Status = statusInProgress
	payload.Job.StartedAt = "2024-11-21T11:00:00Z"
	inProgressBody, _ := json.Marshal(payload)
	updateJobMetrics(context.Background(), inProgressBody)

	payload.Job.Status = statusCompleted
	payload.Job.Conclusion = "success"
	payload.Job.CompletedAt = "2024-11-21T12:00:00Z"
	completedBody, _ := json.Marshal(payload)
	updateJobMetrics(context.Background(), completedBody)
	updateJobMetrics(context.Background(), inProgressBody)

	if got := testutil.ToFloat64(jobQueuedGauge.WithLabelValues("user/repo", "main", "CI")); got != 0 {
		t.Fatalf("expected queued gauge to be 0, got %v", got)
	}
	if got := testutil.ToFloat64(jobInProgressGauge.WithLabelValues("user/repo", "main", "CI")); got != 0 {
		t.Fatalf("expected in progress gauge to be 0, got %v", got)
	}
	if got := testutil.ToFloat64(jobCompletedGauge.WithLabelValues("user/repo", "main", "success", "CI")); got != 1 {
		t.Fatalf("expected completed gauge to be 1, got %v", got)
	}
}
