package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

const (
	statusQueued     = "queued"
	statusInProgress = "in_progress"
	statusCompleted  = "completed"
)

func TestWorkflowStatusCounter(t *testing.T) {
	workflowStatusCounter.Reset()
	reg.MustRegister(workflowStatusCounter)
	body, err := os.ReadFile("../test_data/workflow_run.json")
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}
	updateWorkflowMetrics(body)

	// Test counter
	if err := testutil.CollectAndCompare(workflowStatusCounter, strings.NewReader(`
		# HELP promgithub_workflow_status Total number of workflow runs with status
		# TYPE promgithub_workflow_status counter
		promgithub_workflow_status{conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed"} 1
	`)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestJobStatusCounter(t *testing.T) {
	jobStatusCounter.Reset()
	reg.MustRegister(jobStatusCounter)
	body, err := os.ReadFile("../test_data/workflow_job.json")
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}
	updateJobMetrics(body)

	// Test counter
	if err := testutil.CollectAndCompare(jobStatusCounter, strings.NewReader(`
        # HELP promgithub_job_status Total number of jobs with status
        # TYPE promgithub_job_status counter
        promgithub_job_status{job_conclusion="success",job_status="completed",repository="user/repo",workflow_name="CI"} 1
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

	// Test counter
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

	// Test counter
	if err := testutil.CollectAndCompare(pullRequestCounter, strings.NewReader(`
		# HELP promgithub_pull_request Total number of pull requests
		# TYPE promgithub_pull_request counter
		promgithub_pull_request{base_branch="main",pull_request_status="opened",repository="user/repo"} 1
	`)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestWorkflowDurationHistogram(t *testing.T) {
	workflowDurationHistogram.Reset()
	reg.MustRegister(workflowDurationHistogram)
	body, err := os.ReadFile("../test_data/workflow_run.json")
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}
	updateWorkflowMetrics(body)

	// Test histogram
	if err := testutil.CollectAndCompare(workflowDurationHistogram, strings.NewReader(`
		# HELP promgithub_workflow_duration Duration of workflow runs
		# TYPE promgithub_workflow_duration histogram
		promgithub_workflow_duration_bucket{conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed",le="0.005"} 0
		promgithub_workflow_duration_bucket{conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed",le="0.01"} 0
		promgithub_workflow_duration_bucket{conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed",le="0.025"} 0
		promgithub_workflow_duration_bucket{conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed",le="0.05"} 0
		promgithub_workflow_duration_bucket{conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed",le="0.1"} 0
		promgithub_workflow_duration_bucket{conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed",le="0.25"} 0
		promgithub_workflow_duration_bucket{conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed",le="0.5"} 0
		promgithub_workflow_duration_bucket{conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed",le="1"} 0
		promgithub_workflow_duration_bucket{conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed",le="2.5"} 0
		promgithub_workflow_duration_bucket{conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed",le="5"} 0
		promgithub_workflow_duration_bucket{conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed",le="10"} 0
		promgithub_workflow_duration_bucket{conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed",le="+Inf"} 1
		promgithub_workflow_duration_sum{conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed"} 3600
		promgithub_workflow_duration_count{conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed"} 1
	`)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestJobDurationHistogram(t *testing.T) {
	jobDurationHistogram.Reset()
	reg.MustRegister(jobDurationHistogram)
	body, err := os.ReadFile("../test_data/workflow_job.json")
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}
	updateJobMetrics(body)

	// Test histogram
	if err := testutil.CollectAndCompare(jobDurationHistogram, strings.NewReader(`
        # HELP promgithub_job_duration Duration of jobs runs in seconds
        # TYPE promgithub_job_duration histogram
        promgithub_job_duration_bucket{job_conclusion="success",job_status="completed",repository="user/repo",workflow_name="CI",le="0.005"} 0
        promgithub_job_duration_bucket{job_conclusion="success",job_status="completed",repository="user/repo",workflow_name="CI",le="0.01"} 0
        promgithub_job_duration_bucket{job_conclusion="success",job_status="completed",repository="user/repo",workflow_name="CI",le="0.025"} 0
        promgithub_job_duration_bucket{job_conclusion="success",job_status="completed",repository="user/repo",workflow_name="CI",le="0.05"} 0
        promgithub_job_duration_bucket{job_conclusion="success",job_status="completed",repository="user/repo",workflow_name="CI",le="0.1"} 0
        promgithub_job_duration_bucket{job_conclusion="success",job_status="completed",repository="user/repo",workflow_name="CI",le="0.25"} 0
        promgithub_job_duration_bucket{job_conclusion="success",job_status="completed",repository="user/repo",workflow_name="CI",le="0.5"} 0
        promgithub_job_duration_bucket{job_conclusion="success",job_status="completed",repository="user/repo",workflow_name="CI",le="1"} 0
        promgithub_job_duration_bucket{job_conclusion="success",job_status="completed",repository="user/repo",workflow_name="CI",le="2.5"} 0
        promgithub_job_duration_bucket{job_conclusion="success",job_status="completed",repository="user/repo",workflow_name="CI",le="5"} 0
        promgithub_job_duration_bucket{job_conclusion="success",job_status="completed",repository="user/repo",workflow_name="CI",le="10"} 0
        promgithub_job_duration_bucket{job_conclusion="success",job_status="completed",repository="user/repo",workflow_name="CI",le="+Inf"} 1
        promgithub_job_duration_sum{job_conclusion="success",job_status="completed",repository="user/repo",workflow_name="CI"} 3600
        promgithub_job_duration_count{job_conclusion="success",job_status="completed",repository="user/repo",workflow_name="CI"} 1
    `)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestWorkflowQueuedGauge(t *testing.T) {
	workflowQueuedGauge.Reset()
	reg.MustRegister(workflowQueuedGauge)
	body, err := os.ReadFile("../test_data/workflow_run.json")
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}

	var payload GithubWorkflow

	// Unmarshal the JSON data into the struct
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("Failed to unmarshal JSON data: %v", err)
	}

	// Modify the status field
	payload.Workflow.Status = statusQueued

	// Marshal the modified struct back to JSON if needed
	modifiedBody, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal modified JSON data: %v", err)
	}

	updateWorkflowMetrics(modifiedBody)

	// Test gauge
	if err := testutil.CollectAndCompare(workflowQueuedGauge, strings.NewReader(`
        # HELP promgithub_workflow_queued Number of workflow runs queued
        # TYPE promgithub_workflow_queued gauge
        promgithub_workflow_queued{repository="user/repo",workflow_name="CI"} 1
    `)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestWorkflowInProgressGauge(t *testing.T) {
	workflowInProgressGauge.Reset()
	reg.MustRegister(workflowInProgressGauge)
	body, err := os.ReadFile("../test_data/workflow_run.json")
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}

	var payload GithubWorkflow

	// Unmarshal the JSON data into the struct
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("Failed to unmarshal JSON data: %v", err)
	}

	// Modify the status field
	payload.Workflow.Status = statusInProgress

	// Marshal the modified struct back to JSON if needed
	modifiedBody, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal modified JSON data: %v", err)
	}

	updateWorkflowMetrics(modifiedBody)

	// Test gauge
	if err := testutil.CollectAndCompare(workflowInProgressGauge, strings.NewReader(`
        # HELP promgithub_workflow_in_progress Number of workflow runs in progress
        # TYPE promgithub_workflow_in_progress gauge
        promgithub_workflow_in_progress{repository="user/repo",workflow_name="CI"} 1
    `)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestWorkflowCompletedGauge(t *testing.T) {
	workflowCompletedGauge.Reset()
	reg.MustRegister(workflowCompletedGauge)
	body, err := os.ReadFile("../test_data/workflow_run.json")
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}
	updateWorkflowMetrics(body)

	// Test gauge
	if err := testutil.CollectAndCompare(workflowCompletedGauge, strings.NewReader(`
		# HELP promgithub_workflow_completed Number of workflow runs completed
		# TYPE promgithub_workflow_completed gauge
		promgithub_workflow_completed{repository="user/repo",workflow_conclusion="success",workflow_name="CI"} 1
	`)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestJobQueuedGauge(t *testing.T) {
	jobQueuedGauge.Reset()
	reg.MustRegister(jobQueuedGauge)
	body, err := os.ReadFile("../test_data/workflow_job.json")
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}

	var payload GithubJob

	// Unmarshal the JSON data into the struct
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("Failed to unmarshal JSON data: %v", err)
	}

	// Modify the status field
	payload.Job.Status = statusQueued

	// Marshal the modified struct back to JSON
	modifiedBody, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal modified JSON data: %v", err)
	}

	updateJobMetrics(modifiedBody)

	// Test gauge
	if err := testutil.CollectAndCompare(jobQueuedGauge, strings.NewReader(`
		# HELP promgithub_job_queued Number of jobs queued
		# TYPE promgithub_job_queued gauge
		promgithub_job_queued{repository="user/repo",workflow_name="CI"} 1
	`)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestJobInProgressGauge(t *testing.T) {
	jobInProgressGauge.Reset()
	reg.MustRegister(jobInProgressGauge)
	body, err := os.ReadFile("../test_data/workflow_job.json")
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}

	var payload GithubJob

	// Unmarshal the JSON data into the struct
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("Failed to unmarshal JSON data: %v", err)
	}

	// Modify the status field
	payload.Job.Status = statusInProgress

	// Marshal the modified struct back to JSON
	modifiedBody, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal modified JSON data: %v", err)
	}

	updateJobMetrics(modifiedBody)

	// Test gauge
	if err := testutil.CollectAndCompare(jobInProgressGauge, strings.NewReader(`
		# HELP promgithub_job_in_progress Number of jobs in progress
		# TYPE promgithub_job_in_progress gauge
		promgithub_job_in_progress{repository="user/repo",workflow_name="CI"} 1
	`)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestJobCompletedGauge(t *testing.T) {
	jobCompletedGauge.Reset()
	reg.MustRegister(jobCompletedGauge)

	body, err := os.ReadFile("../test_data/workflow_job.json")
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}

	var payload GithubJob

	// Unmarshal the JSON data into the struct
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("Failed to unmarshal JSON data: %v", err)
	}

	// Modify the status field
	payload.Job.Status = statusCompleted

	// Marshal the modified struct back to JSON
	modifiedBody, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal modified JSON data: %v", err)
	}

	updateJobMetrics(modifiedBody)

	// Test gauge
	if err := testutil.CollectAndCompare(jobCompletedGauge, strings.NewReader(`
		# HELP promgithub_job_completed Number of jobs completed
		# TYPE promgithub_job_completed gauge
		promgithub_job_completed{job_conclusion="success",repository="user/repo",workflow_name="CI"} 1
	`)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}
