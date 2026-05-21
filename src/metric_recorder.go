package main

type runMetricKind string

const (
	runMetricKindWorkflow runMetricKind = "workflow"
	runMetricKindJob      runMetricKind = "job"
)

var defaultMetricRecorder = prometheusMetricRecorder{}

type prometheusMetricRecorder struct{}

func (prometheusMetricRecorder) RecordDuplicateDelivery(eventType string) {
	duplicateDeliveriesSeenCounter.WithLabelValues(eventType).Inc()
	duplicateDeliveriesDroppedCounter.WithLabelValues(eventType).Inc()
}

func (prometheusMetricRecorder) RecordCommitPushed(repository string) {
	commitPushedCounter.WithLabelValues(repository).Inc()
}

func (prometheusMetricRecorder) RecordPullRequest(repository, baseBranch, status string) {
	pullRequestCounter.WithLabelValues(repository, baseBranch, status).Inc()
}

func (prometheusMetricRecorder) SetAsyncWorkerCount(workers int) {
	asyncWorkerCountGauge.Set(float64(workers))
}

func (prometheusMetricRecorder) SetAsyncQueueCapacity(capacity int) {
	asyncQueueCapacityGauge.Set(float64(capacity))
}

func (prometheusMetricRecorder) SetAsyncQueueDepth(depth int) {
	asyncQueueDepthGauge.Set(float64(depth))
}

func (prometheusMetricRecorder) RecordAsyncQueueDropped(eventType string) {
	asyncQueueDroppedCounter.WithLabelValues(eventType).Inc()
}

func (prometheusMetricRecorder) RecordAsyncUnsupportedEvent(eventType string) {
	asyncUnsupportedEventsCounter.WithLabelValues(eventType).Inc()
}

func (prometheusMetricRecorder) RecordAsyncProcessingFailure(eventType string) {
	asyncProcessingFailuresCounter.WithLabelValues(eventType).Inc()
}

func (prometheusMetricRecorder) RecordAsyncProcessedEvent(eventType string) {
	asyncProcessedEventsCounter.WithLabelValues(eventType).Inc()
}

func (prometheusMetricRecorder) ObserveAsyncProcessingDuration(eventType string, durationSeconds float64) {
	asyncProcessingDurationHistogram.WithLabelValues(eventType).Observe(durationSeconds)
}

func (prometheusMetricRecorder) RecordRunStatus(kind runMetricKind, state RunState) {
	switch kind {
	case runMetricKindWorkflow:
		workflowStatusCounter.WithLabelValues(
			state.Repository,
			state.Branch,
			state.Name,
			state.Status,
			state.Conclusion,
		).Inc()
	case runMetricKindJob:
		jobStatusCounter.WithLabelValues(
			state.Repository,
			state.Branch,
			state.Name,
			state.Status,
			state.Conclusion,
		).Inc()
	}
}

func (prometheusMetricRecorder) AddRunGauge(kind runMetricKind, state RunState, delta float64) {
	switch kind {
	case runMetricKindWorkflow:
		addWorkflowRunGauge(state, delta)
	case runMetricKindJob:
		addWorkflowJobGauge(state, delta)
	}
}

func (prometheusMetricRecorder) ObserveRunDuration(kind runMetricKind, state RunState, durationSeconds float64) {
	switch kind {
	case runMetricKindWorkflow:
		workflowDurationHistogram.WithLabelValues(
			state.Repository,
			state.Branch,
			state.Name,
			state.Status,
			state.Conclusion,
		).Observe(durationSeconds)
	case runMetricKindJob:
		jobDurationHistogram.WithLabelValues(
			state.Repository,
			state.Branch,
			state.Name,
			state.Status,
			state.Conclusion,
		).Observe(durationSeconds)
	}
}

func addWorkflowRunGauge(state RunState, delta float64) {
	switch normalizeStatus(state.Status) {
	case statusQueued:
		workflowQueuedGauge.WithLabelValues(state.Repository, state.Branch, state.Name).Add(delta)
	case statusInProgress:
		workflowInProgressGauge.WithLabelValues(state.Repository, state.Branch, state.Name).Add(delta)
	case statusCompleted:
		workflowCompletedGauge.WithLabelValues(state.Repository, state.Branch, state.Conclusion, state.Name).Add(delta)
	}
}

func addWorkflowJobGauge(state RunState, delta float64) {
	switch normalizeStatus(state.Status) {
	case statusQueued:
		jobQueuedGauge.WithLabelValues(state.Repository, state.Branch, state.Name).Add(delta)
	case statusInProgress:
		jobInProgressGauge.WithLabelValues(state.Repository, state.Branch, state.Name).Add(delta)
	case statusCompleted:
		jobCompletedGauge.WithLabelValues(state.Repository, state.Branch, state.Conclusion, state.Name).Add(delta)
	}
}
