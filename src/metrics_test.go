package main

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestWorkflowStatusCounter(t *testing.T) {
	reg := prometheus.NewRegistry()
	workflowStatusCounter.Reset()
	reg.MustRegister(workflowStatusCounter)
	body := []byte(`{"workflow_run": {"id": 1, "status": "completed", "run_id": 1001, "name": "CI", "head_branch": "main", "repository": {"full_name": "user/repo"}, "conclusion": "success", "html_url": "https://github.com/user/repo/actions/runs/1001", "created_at": "2023-01-01T00:00:00Z", "updated_at": "2023-01-01T01:00:00Z"}}`)
	updateWorkflowMetrics(body)

	// Test counter
	if err := testutil.CollectAndCompare(workflowStatusCounter, strings.NewReader(`
		# HELP promgithub_workflow_status Total number of workflow runs with status
		# TYPE promgithub_workflow_status counter
		promgithub_workflow_status{branch="main",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed",workflow_url="https://github.com/user/repo/actions/runs/1001"} 1
	`)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestJobStatusCounter(t *testing.T) {
	reg := prometheus.NewRegistry()
	jobStatusCounter.Reset()
	reg.MustRegister(jobStatusCounter)
	body := []byte(`{"workflow_job": {"id": 1, "status": "completed", "name": "Job1", "head_branch": "main", "repository": {"full_name": "user/repo"}, "runner_name": "runner1", "conclusion": "success", "html_url": "https://github.com/user/repo/actions/jobs/1", "started_at": "2023-01-01T00:00:00Z", "completed_at": "2023-01-01T01:00:00Z"}}`)
	updateJobMetrics(body)

	// Test counter
	if err := testutil.CollectAndCompare(jobStatusCounter, strings.NewReader(`
        # HELP promgithub_job_status Total number of jobs with status
        # TYPE promgithub_job_status counter
        promgithub_job_status{branch="main",job_conclusion="success",job_name="Job1",job_status="completed",job_url="https://github.com/user/repo/actions/jobs/1",repository="user/repo",runner="runner1",workflow_name="Job1"} 1
    `)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestCommitsPushedCounter(t *testing.T) {
	reg := prometheus.NewRegistry()
	commitPushedCounter.Reset()
	reg.MustRegister(commitPushedCounter)
	body := []byte(`{"repository": {"full_name": "user/repo"}, "commits": [{"id": "commit1", "author": {"name": "Author1", "email": "author1@example.com"}}], "ref": "refs/heads/main"}`)
	updateCommitMetrics(body)

	// Test counter
	if err := testutil.CollectAndCompare(commitPushedCounter, strings.NewReader(`
		# HELP promgithub_commit_pushed Total number of commits pushed
		# TYPE promgithub_commit_pushed counter
		promgithub_commit_pushed{branch="refs/heads/main",commit_author="Author1",commit_author_email="author1@example.com",repository="user/repo"} 1
	`)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestPullRequestsCounter(t *testing.T) {
	reg := prometheus.NewRegistry()
	pullRequestCounter.Reset()
	reg.MustRegister(pullRequestCounter)
	body := []byte(`{"action": "opened", "pull_request": {"id": 1, "state": "open", "title": "PR title", "base": {"ref": "main"}, "head": {"ref": "feature-branch"}, "user": {"login": "user1", "email": "user1@example.com"}}, "repository": {"full_name": "user/repo"}}`)
	updatePullRequestMetrics(body)

	// Test counter
	if err := testutil.CollectAndCompare(pullRequestCounter, strings.NewReader(`
		# HELP promgithub_pull_request Total number of pull requests
		# TYPE promgithub_pull_request counter
		promgithub_pull_request{base_branch="main",head_branch="feature-branch",pull_request_author="user1",pull_request_author_email="user1@example.com",pull_request_status="opened",repository="user/repo"} 1
	`)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestWorkflowDurationHistogram(t *testing.T) {
	reg := prometheus.NewRegistry()
	workflowDurationHistogram.Reset()
	reg.MustRegister(workflowDurationHistogram)
	body := []byte(`{"workflow_run": {"id": 1, "status": "completed", "run_id": 1001, "name": "CI", "head_branch": "main", "repository": {"full_name": "user/repo"}, "conclusion": "success", "created_at": "2023-01-01T00:00:00Z", "updated_at": "2023-01-01T01:00:00Z"}}`)
	updateWorkflowMetrics(body)

	// Test histogram
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
	reg := prometheus.NewRegistry()
	jobDurationHistogram.Reset()
	reg.MustRegister(jobDurationHistogram)
	body := []byte(`{"workflow_job": {"id": 1, "status": "completed", "name": "Job1", "head_branch": "main", "repository": {"full_name": "user/repo"}, "runner_name": "runner1", "conclusion": "success", "started_at": "2023-01-01T00:00:00Z", "completed_at": "2023-01-01T01:00:00Z"}}`)
	updateJobMetrics(body)

	// Test histogram
	if err := testutil.CollectAndCompare(jobDurationHistogram, strings.NewReader(`
        # HELP promgithub_job_duration Duration of jobs runs in seconds
        # TYPE promgithub_job_duration histogram
        promgithub_job_duration_bucket{branch="main",job_conclusion="success",job_name="Job1",job_status="completed",repository="user/repo",runner="runner1",workflow_name="Job1",le="0.005"} 0
        promgithub_job_duration_bucket{branch="main",job_conclusion="success",job_name="Job1",job_status="completed",repository="user/repo",runner="runner1",workflow_name="Job1",le="0.01"} 0
        promgithub_job_duration_bucket{branch="main",job_conclusion="success",job_name="Job1",job_status="completed",repository="user/repo",runner="runner1",workflow_name="Job1",le="0.025"} 0
        promgithub_job_duration_bucket{branch="main",job_conclusion="success",job_name="Job1",job_status="completed",repository="user/repo",runner="runner1",workflow_name="Job1",le="0.05"} 0
        promgithub_job_duration_bucket{branch="main",job_conclusion="success",job_name="Job1",job_status="completed",repository="user/repo",runner="runner1",workflow_name="Job1",le="0.1"} 0
        promgithub_job_duration_bucket{branch="main",job_conclusion="success",job_name="Job1",job_status="completed",repository="user/repo",runner="runner1",workflow_name="Job1",le="0.25"} 0
        promgithub_job_duration_bucket{branch="main",job_conclusion="success",job_name="Job1",job_status="completed",repository="user/repo",runner="runner1",workflow_name="Job1",le="0.5"} 0
        promgithub_job_duration_bucket{branch="main",job_conclusion="success",job_name="Job1",job_status="completed",repository="user/repo",runner="runner1",workflow_name="Job1",le="1"} 0
        promgithub_job_duration_bucket{branch="main",job_conclusion="success",job_name="Job1",job_status="completed",repository="user/repo",runner="runner1",workflow_name="Job1",le="2.5"} 0
        promgithub_job_duration_bucket{branch="main",job_conclusion="success",job_name="Job1",job_status="completed",repository="user/repo",runner="runner1",workflow_name="Job1",le="5"} 0
        promgithub_job_duration_bucket{branch="main",job_conclusion="success",job_name="Job1",job_status="completed",repository="user/repo",runner="runner1",workflow_name="Job1",le="10"} 0
        promgithub_job_duration_bucket{branch="main",job_conclusion="success",job_name="Job1",job_status="completed",repository="user/repo",runner="runner1",workflow_name="Job1",le="+Inf"} 1
        promgithub_job_duration_sum{branch="main",job_conclusion="success",job_name="Job1",job_status="completed",repository="user/repo",runner="runner1",workflow_name="Job1"} 3600
        promgithub_job_duration_count{branch="main",job_conclusion="success",job_name="Job1",job_status="completed",repository="user/repo",runner="runner1",workflow_name="Job1"} 1
    `)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestWorkflowQueuedGauge(t *testing.T) {
	reg := prometheus.NewRegistry()
	workflowQueuedGauge.Reset()
	reg.MustRegister(workflowQueuedGauge)
	body := []byte(`{"workflow_run": {"id": 1, "status": "queued", "run_id": 1001, "name": "CI", "head_branch": "main", "repository": {"full_name": "user/repo"}}}`)
	updateWorkflowMetrics(body)

	// Test gauge
	if err := testutil.CollectAndCompare(workflowQueuedGauge, strings.NewReader(`
        # HELP promgithub_workflow_queued Number of workflow runs queued
        # TYPE promgithub_workflow_queued gauge
        promgithub_workflow_queued{branch="main",repository="user/repo",workflow_name="CI"} 1
    `)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestWorkflowInProgressGauge(t *testing.T) {
	reg := prometheus.NewRegistry()
	workflowInProgressGauge.Reset()
	reg.MustRegister(workflowInProgressGauge)
	body := []byte(`{"workflow_run": {"id": 1, "status": "in_progress", "run_id": 1001, "name": "CI", "head_branch": "main", "repository": {"full_name": "user/repo"}}}`)
	updateWorkflowMetrics(body)

	// Test gauge
	if err := testutil.CollectAndCompare(workflowInProgressGauge, strings.NewReader(`
        # HELP promgithub_workflow_in_progress Number of workflow runs in progress
        # TYPE promgithub_workflow_in_progress gauge
        promgithub_workflow_in_progress{branch="main",repository="user/repo",workflow_name="CI"} 1
    `)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestWorkflowCompletedGauge(t *testing.T) {
	reg := prometheus.NewRegistry()
	workflowCompletedGauge.Reset()
	reg.MustRegister(workflowCompletedGauge)
	body := []byte(`{"workflow_run": {"id": 1, "status": "completed", "run_id": 1001, "name": "CI", "head_branch": "main", "repository": {"full_name": "user/repo"}}}`)
	updateWorkflowMetrics(body)

	// Test gauge
	if err := testutil.CollectAndCompare(workflowCompletedGauge, strings.NewReader(`
		# HELP promgithub_workflow_completed Number of workflow runs completed
		# TYPE promgithub_workflow_completed gauge
		promgithub_workflow_completed{branch="main",repository="user/repo",workflow_name="CI"} 1
	`)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestJobQueuedGauge(t *testing.T) {
	reg := prometheus.NewRegistry()
	jobQueuedGauge.Reset()
	reg.MustRegister(jobQueuedGauge)
	body := []byte(`{"workflow_job": {"id": 1, "status": "queued", "name": "Job1", "head_branch": "main", "repository": {"full_name": "user/repo"}, "runner_name": "runner1"}}`)
	updateJobMetrics(body)

	// Test gauge
	if err := testutil.CollectAndCompare(jobQueuedGauge, strings.NewReader(`
		# HELP promgithub_job_queued Number of jobs queued
		# TYPE promgithub_job_queued gauge
		promgithub_job_queued{branch="main",job_name="Job1",repository="user/repo",runner="runner1",workflow_name="Job1"} 1
	`)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestJobInProgressGauge(t *testing.T) {
	reg := prometheus.NewRegistry()
	jobInProgressGauge.Reset()
	reg.MustRegister(jobInProgressGauge)
	body := []byte(`{"workflow_job": {"id": 1, "status": "in_progress", "name": "Job1", "head_branch": "main", "repository": {"full_name": "user/repo"}, "runner_name": "runner1"}}`)
	updateJobMetrics(body)

	// Test gauge
	if err := testutil.CollectAndCompare(jobInProgressGauge, strings.NewReader(`
		# HELP promgithub_job_in_progress Number of jobs in progress
		# TYPE promgithub_job_in_progress gauge
		promgithub_job_in_progress{branch="main",job_name="Job1",repository="user/repo",runner="runner1",workflow_name="Job1"} 1
	`)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestJobCompletedGauge(t *testing.T) {
	reg := prometheus.NewRegistry()
	jobCompletedGauge.Reset()
	reg.MustRegister(jobCompletedGauge)
	body := []byte(`{"workflow_job": {"id": 1, "status": "completed", "name": "Job1", "head_branch": "main", "repository": {"full_name": "user/repo"}, "runner_name": "runner1"}}`)
	updateJobMetrics(body)

	// Test gauge
	if err := testutil.CollectAndCompare(jobCompletedGauge, strings.NewReader(`
		# HELP promgithub_job_completed Number of jobs completed
		# TYPE promgithub_job_completed gauge
		promgithub_job_completed{branch="main",job_name="Job1",repository="user/repo",runner="runner1",workflow_name="Job1"} 1
	`)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}
