package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Workflow metrics.
	workflowStatusCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "promgithub_workflow_status",
			Help: "Total number of workflow runs with status",
		},
		[]string{"repository", "branch", "workflow_name", "workflow_status", "conclusion"},
	)

	workflowDurationHistogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "promgithub_workflow_duration",
			Help:    "Duration of workflow runs",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"repository", "branch", "workflow_name", "workflow_status", "conclusion"},
	)

	workflowQueuedGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "promgithub_workflow_queued",
			Help: "Number of workflow runs queued",
		},
		[]string{"repository", "branch", "workflow_name"},
	)

	workflowInProgressGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "promgithub_workflow_in_progress",
			Help: "Number of workflow runs in progress",
		},
		[]string{"repository", "branch", "workflow_name"},
	)

	workflowCompletedGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "promgithub_workflow_completed",
			Help: "Number of workflow runs completed",
		},
		[]string{"repository", "branch", "workflow_conclusion", "workflow_name"},
	)

	// Job metrics.
	jobStatusCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "promgithub_job_status",
			Help: "Total number of jobs with status",
		},
		[]string{"repository", "branch", "workflow_name", "job_status", "job_conclusion"},
	)

	jobDurationHistogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "promgithub_job_duration",
			Help:    "Duration of jobs runs in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"repository", "branch", "workflow_name", "job_status", "job_conclusion"},
	)

	jobQueuedGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "promgithub_job_queued",
			Help: "Number of jobs queued",
		},
		[]string{"repository", "branch", "workflow_name"},
	)

	jobInProgressGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "promgithub_job_in_progress",
			Help: "Number of jobs in progress",
		},
		[]string{"repository", "branch", "workflow_name"},
	)

	jobCompletedGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "promgithub_job_completed",
			Help: "Number of jobs completed",
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

	duplicateDeliveriesSeenCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "promgithub_duplicate_deliveries_seen_total",
			Help: "Total number of duplicate GitHub webhook deliveries observed",
		},
		[]string{"event_type"},
	)

	duplicateDeliveriesDroppedCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "promgithub_duplicate_deliveries_dropped_total",
			Help: "Total number of duplicate GitHub webhook deliveries dropped",
		},
		[]string{"event_type"},
	)
)
