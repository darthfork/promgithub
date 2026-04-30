package main

import (
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const detailedMetricsEnvVar = "PROMGITHUB_ENABLE_DETAILED_METRICS"

var (
	enableDetailedMetrics = parseBoolEnv(os.Getenv(detailedMetricsEnvVar))

	// Workflow metrics.
	workflowStatusCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "promgithub_workflow_status",
			Help: "Total number of workflow runs with status",
		},
		[]string{"repository", "workflow_status", "conclusion"},
	)

	workflowDurationHistogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "promgithub_workflow_duration",
			Help:    "Duration of workflow runs",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"repository", "workflow_status", "conclusion"},
	)

	workflowQueuedGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "promgithub_workflow_queued",
			Help: "Number of workflow runs queued",
		},
		[]string{"repository"},
	)

	workflowInProgressGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "promgithub_workflow_in_progress",
			Help: "Number of workflow runs in progress",
		},
		[]string{"repository"},
	)

	workflowCompletedGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "promgithub_workflow_completed",
			Help: "Number of workflow runs completed",
		},
		[]string{"repository", "workflow_conclusion"},
	)

	workflowStatusDetailedCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "promgithub_workflow_status_detailed",
			Help: "Total number of workflow runs with status and optional high-cardinality labels",
		},
		[]string{"repository", "branch", "workflow_name", "workflow_status", "conclusion"},
	)

	workflowDurationDetailedHistogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "promgithub_workflow_duration_detailed",
			Help:    "Duration of workflow runs with optional high-cardinality labels",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"repository", "branch", "workflow_name", "workflow_status", "conclusion"},
	)

	workflowQueuedDetailedGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "promgithub_workflow_queued_detailed",
			Help: "Number of workflow runs queued with optional high-cardinality labels",
		},
		[]string{"repository", "branch", "workflow_name"},
	)

	workflowInProgressDetailedGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "promgithub_workflow_in_progress_detailed",
			Help: "Number of workflow runs in progress with optional high-cardinality labels",
		},
		[]string{"repository", "branch", "workflow_name"},
	)

	workflowCompletedDetailedGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "promgithub_workflow_completed_detailed",
			Help: "Number of workflow runs completed with optional high-cardinality labels",
		},
		[]string{"repository", "branch", "workflow_conclusion", "workflow_name"},
	)

	// Job metrics.
	jobStatusCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "promgithub_job_status",
			Help: "Total number of jobs with status",
		},
		[]string{"repository", "job_status", "job_conclusion"},
	)

	jobDurationHistogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "promgithub_job_duration",
			Help:    "Duration of jobs runs in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"repository", "job_status", "job_conclusion"},
	)

	jobQueuedGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "promgithub_job_queued",
			Help: "Number of jobs queued",
		},
		[]string{"repository"},
	)

	jobInProgressGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "promgithub_job_in_progress",
			Help: "Number of jobs in progress",
		},
		[]string{"repository"},
	)

	jobCompletedGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "promgithub_job_completed",
			Help: "Number of jobs completed",
		},
		[]string{"repository", "job_conclusion"},
	)

	jobStatusDetailedCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "promgithub_job_status_detailed",
			Help: "Total number of jobs with status and optional high-cardinality labels",
		},
		[]string{"repository", "branch", "workflow_name", "job_status", "job_conclusion"},
	)

	jobDurationDetailedHistogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "promgithub_job_duration_detailed",
			Help:    "Duration of jobs runs in seconds with optional high-cardinality labels",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"repository", "branch", "workflow_name", "job_status", "job_conclusion"},
	)

	jobQueuedDetailedGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "promgithub_job_queued_detailed",
			Help: "Number of jobs queued with optional high-cardinality labels",
		},
		[]string{"repository", "branch", "workflow_name"},
	)

	jobInProgressDetailedGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "promgithub_job_in_progress_detailed",
			Help: "Number of jobs in progress with optional high-cardinality labels",
		},
		[]string{"repository", "branch", "workflow_name"},
	)

	jobCompletedDetailedGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "promgithub_job_completed_detailed",
			Help: "Number of jobs completed with optional high-cardinality labels",
		},
		[]string{"repository", "branch", "job_conclusion", "workflow_name"},
	)

	commitPushedCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "promgithub_commit_pushed",
			Help: "Total number of commits pushed",
		},
		[]string{"repository"},
	)

	pullRequestCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "promgithub_pull_request",
			Help: "Total number of pull requests",
		},
		[]string{"repository", "pull_request_status"},
	)

	pullRequestDetailedCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "promgithub_pull_request_detailed",
			Help: "Total number of pull requests with optional high-cardinality labels",
		},
		[]string{"repository", "base_branch", "pull_request_status"},
	)

	asyncQueueDepthGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "promgithub_event_queue_depth",
			Help: "Current number of queued webhook events awaiting processing",
		},
	)

	asyncQueueCapacityGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "promgithub_event_queue_capacity",
			Help: "Configured capacity of the webhook event queue",
		},
	)

	asyncWorkerCountGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "promgithub_event_worker_count",
			Help: "Configured number of async webhook event workers",
		},
	)

	asyncProcessedEventsCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "promgithub_event_processed_total",
			Help: "Total number of webhook events processed asynchronously",
		},
		[]string{"event_type"},
	)

	asyncEventsDroppedCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "promgithub_event_dropped_total",
			Help: "Total number of webhook events dropped before processing",
		},
		[]string{"event_type", "reason"},
	)

	asyncProcessingFailuresCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "promgithub_event_processing_failures_total",
			Help: "Total number of async webhook processing failures",
		},
		[]string{"event_type"},
	)

	asyncProcessingDurationHistogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "promgithub_event_processing_duration_seconds",
			Help:    "Duration of async webhook event processing",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"event_type"},
	)
)

func parseBoolEnv(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
