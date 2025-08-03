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
		[]string{"runner", "repository", "branch", "workflow_name", "job_name", "job_status", "job_conclusion"},
	)

	jobDurationHistogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "promgithub_job_duration",
			Help:    "Duration of jobs runs in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"runner", "repository", "branch", "workflow_name", "job_name", "job_status", "job_conclusion"},
	)

	jobQueuedGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "promgithub_job_queued",
			Help: "Number of jobs queued",
		},
		[]string{"runner", "repository", "branch", "workflow_name", "job_name"},
	)

	jobInProgressGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "promgithub_job_in_progress",
			Help: "Number of jobs in progress",
		},
		[]string{"runner", "repository", "branch", "workflow_name", "job_name"},
	)

	jobCompletedGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "promgithub_job_completed",
			Help: "Number of jobs completed",
		},
		[]string{"runner", "repository", "branch", "job_conclusion", "workflow_name", "job_name"},
	)

	commitPushedCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "promgithub_commit_pushed",
			Help: "Total number of commits pushed",
		},
		[]string{"repository", "branch", "commit_author", "commit_author_email"},
	)

	pullRequestCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "promgithub_pull_request",
			Help: "Total number of pull requests",
		},
		[]string{"repository", "base_branch", "pull_request_author", "pull_request_status"},
	)
)
