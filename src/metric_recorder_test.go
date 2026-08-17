//go:build !integration

package main

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMetricRecorderPreservesDuplicateDeliveryCardinality(t *testing.T) {
	duplicateDeliveriesSeenCounter.Reset()
	duplicateDeliveriesDroppedCounter.Reset()

	recorder := prometheusMetricRecorder{}
	recorder.RecordDuplicateDelivery(githubEventWorkflowRun)

	if err := testutil.CollectAndCompare(duplicateDeliveriesSeenCounter, strings.NewReader(`
		# HELP promgithub_duplicate_deliveries_seen_total Total number of duplicate GitHub webhook deliveries observed
		# TYPE promgithub_duplicate_deliveries_seen_total counter
		promgithub_duplicate_deliveries_seen_total{event_type="workflow_run"} 1
	`)); err != nil {
		t.Fatalf("unexpected duplicate seen metric: %v", err)
	}

	if err := testutil.CollectAndCompare(duplicateDeliveriesDroppedCounter, strings.NewReader(`
		# HELP promgithub_duplicate_deliveries_dropped_total Total number of duplicate GitHub webhook deliveries dropped
		# TYPE promgithub_duplicate_deliveries_dropped_total counter
		promgithub_duplicate_deliveries_dropped_total{event_type="workflow_run"} 1
	`)); err != nil {
		t.Fatalf("unexpected duplicate dropped metric: %v", err)
	}
}

func TestMetricRecorderCountsFilteredEvents(t *testing.T) {
	filteredEventsCounter.Reset()

	recorder := prometheusMetricRecorder{}
	recorder.RecordFilteredEvent(githubEventWorkflowRun, filterReasonRepository)

	if err := testutil.CollectAndCompare(filteredEventsCounter, strings.NewReader(`
		# HELP promgithub_event_filtered_total Total number of webhook events dropped by the configured label policy
		# TYPE promgithub_event_filtered_total counter
		promgithub_event_filtered_total{event_type="workflow_run",reason="repository"} 1
	`)); err != nil {
		t.Fatalf("unexpected filtered event metric: %v", err)
	}
}

func TestMetricRecorderPreservesRepositoryEventCardinality(t *testing.T) {
	commitPushedCounter.Reset()
	pullRequestCounter.Reset()

	recorder := prometheusMetricRecorder{}
	recorder.RecordCommitPushed("user/repo")
	recorder.RecordPullRequest("user/repo", "main", "opened")

	if err := testutil.CollectAndCompare(commitPushedCounter, strings.NewReader(`
		# HELP promgithub_commit_pushed Total number of commits pushed
		# TYPE promgithub_commit_pushed counter
		promgithub_commit_pushed{repository="user/repo"} 1
	`)); err != nil {
		t.Fatalf("unexpected commit metric: %v", err)
	}

	if err := testutil.CollectAndCompare(pullRequestCounter, strings.NewReader(`
		# HELP promgithub_pull_request Total number of pull requests
		# TYPE promgithub_pull_request counter
		promgithub_pull_request{base_branch="main",pull_request_status="opened",repository="user/repo"} 1
	`)); err != nil {
		t.Fatalf("unexpected pull request metric: %v", err)
	}
}

func TestMetricRecorderPreservesRunMetricCardinality(t *testing.T) {
	workflowStatusCounter.Reset()
	workflowCompletedGauge.Reset()
	workflowDurationHistogram.Reset()

	recorder := prometheusMetricRecorder{}
	state := RunState{
		Repository: runTransitionTestRepository,
		Branch:     runTransitionTestBranch,
		Name:       runTransitionTestName,
		Status:     statusCompleted,
		Conclusion: testConclusionSuccess,
		StartedAt:  runTransitionStartedAt,
		EndedAt:    runTransitionCompletedAt,
	}

	recorder.RecordRunStatus(runMetricKindWorkflow, state)
	recorder.AddRunGauge(runMetricKindWorkflow, state, 1)
	recorder.ObserveRunDuration(runMetricKindWorkflow, state, 3600)

	if err := testutil.CollectAndCompare(workflowStatusCounter, strings.NewReader(`
		# HELP promgithub_workflow_status Total number of workflow runs with status
		# TYPE promgithub_workflow_status counter
		promgithub_workflow_status{branch="main",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed"} 1
	`)); err != nil {
		t.Fatalf("unexpected workflow status metric: %v", err)
	}

	if got := testutil.ToFloat64(workflowCompletedGauge.WithLabelValues(runTransitionTestRepository, runTransitionTestBranch, testConclusionSuccess, runTransitionTestName)); got != 1 {
		t.Fatalf("expected completed gauge to be 1, got %v", got)
	}

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
		t.Fatalf("unexpected workflow duration metric: %v", err)
	}
}

func TestMetricRecorderPreservesAsyncMetricCardinality(t *testing.T) {
	asyncQueueDepthGauge.Set(0)
	asyncQueueCapacityGauge.Set(0)
	asyncWorkerCountGauge.Set(0)
	asyncQueueDroppedCounter.Reset()
	asyncUnsupportedEventsCounter.Reset()
	asyncProcessingFailuresCounter.Reset()
	asyncProcessedEventsCounter.Reset()
	asyncProcessingDurationHistogram.Reset()

	recorder := prometheusMetricRecorder{}
	recorder.SetAsyncQueueDepth(2)
	recorder.SetAsyncQueueCapacity(8)
	recorder.SetAsyncWorkerCount(4)
	recorder.RecordAsyncQueueDropped(githubEventWorkflowRun)
	recorder.RecordAsyncUnsupportedEvent("unknown_event")
	recorder.RecordAsyncProcessingFailure(githubEventWorkflowRun)
	recorder.RecordAsyncProcessedEvent(githubEventWorkflowRun)
	recorder.ObserveAsyncProcessingDuration(githubEventWorkflowRun, 1.5)

	if got := testutil.ToFloat64(asyncQueueDepthGauge); got != 2 {
		t.Fatalf("expected queue depth 2, got %v", got)
	}
	if got := testutil.ToFloat64(asyncQueueCapacityGauge); got != 8 {
		t.Fatalf("expected queue capacity 8, got %v", got)
	}
	if got := testutil.ToFloat64(asyncWorkerCountGauge); got != 4 {
		t.Fatalf("expected worker count 4, got %v", got)
	}
	if got := testutil.ToFloat64(asyncQueueDroppedCounter.WithLabelValues(githubEventWorkflowRun)); got != 1 {
		t.Fatalf("expected queue dropped counter 1, got %v", got)
	}
	if got := testutil.ToFloat64(asyncUnsupportedEventsCounter.WithLabelValues("unknown_event")); got != 1 {
		t.Fatalf("expected unsupported counter 1, got %v", got)
	}
	if got := testutil.ToFloat64(asyncProcessingFailuresCounter.WithLabelValues(githubEventWorkflowRun)); got != 1 {
		t.Fatalf("expected processing failures counter 1, got %v", got)
	}
	if got := testutil.ToFloat64(asyncProcessedEventsCounter.WithLabelValues(githubEventWorkflowRun)); got != 1 {
		t.Fatalf("expected processed counter 1, got %v", got)
	}
	if err := testutil.CollectAndCompare(asyncProcessingDurationHistogram, strings.NewReader(`
		# HELP promgithub_event_processing_duration_seconds Duration of async webhook event processing
		# TYPE promgithub_event_processing_duration_seconds histogram
		promgithub_event_processing_duration_seconds_bucket{event_type="workflow_run",le="0.005"} 0
		promgithub_event_processing_duration_seconds_bucket{event_type="workflow_run",le="0.01"} 0
		promgithub_event_processing_duration_seconds_bucket{event_type="workflow_run",le="0.025"} 0
		promgithub_event_processing_duration_seconds_bucket{event_type="workflow_run",le="0.05"} 0
		promgithub_event_processing_duration_seconds_bucket{event_type="workflow_run",le="0.1"} 0
		promgithub_event_processing_duration_seconds_bucket{event_type="workflow_run",le="0.25"} 0
		promgithub_event_processing_duration_seconds_bucket{event_type="workflow_run",le="0.5"} 0
		promgithub_event_processing_duration_seconds_bucket{event_type="workflow_run",le="1"} 0
		promgithub_event_processing_duration_seconds_bucket{event_type="workflow_run",le="2.5"} 1
		promgithub_event_processing_duration_seconds_bucket{event_type="workflow_run",le="5"} 1
		promgithub_event_processing_duration_seconds_bucket{event_type="workflow_run",le="10"} 1
		promgithub_event_processing_duration_seconds_bucket{event_type="workflow_run",le="+Inf"} 1
		promgithub_event_processing_duration_seconds_sum{event_type="workflow_run"} 1.5
		promgithub_event_processing_duration_seconds_count{event_type="workflow_run"} 1
	`)); err != nil {
		t.Fatalf("unexpected async duration metric: %v", err)
	}
}
