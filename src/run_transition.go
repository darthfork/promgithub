package main

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

type runMetricDetails struct {
	repository string
	branch     string
	name       string
	status     string
	conclusion string
	startedAt  string
	endedAt    string
}

type runMetricSet struct {
	statusCounter     *prometheus.CounterVec
	queuedGauge       *prometheus.GaugeVec
	inProgressGauge   *prometheus.GaugeVec
	completedGauge    *prometheus.GaugeVec
	durationHistogram *prometheus.HistogramVec
}

type runStoreMethods struct {
	get    func(context.Context, int) (RunState, bool, error)
	update func(context.Context, int, RunState) error
}

type runTransitionStore interface {
	GetRunState(context.Context, int) (RunState, bool, error)
	UpdateRunState(context.Context, int, RunState) error
}

type runTransitionRecorder interface {
	RecordStatus(RunState)
	AddGauge(RunState, float64)
	ObserveDuration(RunState, float64)
}

type runTransitionResult struct {
	Applied bool
	Skipped bool
	Err     error
}

type runTransitionProcessor struct {
	store      runTransitionStore
	recorder   runTransitionRecorder
	logger     *zap.Logger
	entityName string
}

func (p *runTransitionProcessor) Apply(ctx context.Context, id int, details runMetricDetails) runTransitionResult {
	nextState := normalizeRunState(details)

	if p.store == nil {
		p.recordTransition(nextState, nil)
		return runTransitionResult{Applied: true}
	}

	previousState, found, err := p.store.GetRunState(ctx, id)
	if err != nil {
		p.logError("Failed to load run state from redis", id, err)
		return runTransitionResult{Err: err}
	}

	var previous *RunState
	if found {
		previous = &previousState
		if !shouldApplyStateTransition(previousState, nextState) {
			p.logDebug("Skipping stale or duplicate run transition", id, nextState)
			return runTransitionResult{Skipped: true}
		}
	}

	if err := p.store.UpdateRunState(ctx, id, nextState); err != nil {
		p.logError("Failed to update run state in redis", id, err)
		return runTransitionResult{Err: err}
	}

	p.recordTransition(nextState, previous)
	return runTransitionResult{Applied: true}
}

func (p *runTransitionProcessor) recordTransition(nextState RunState, previous *RunState) {
	if p.recorder == nil {
		return
	}

	p.recorder.RecordStatus(nextState)
	if previous != nil {
		p.recorder.AddGauge(*previous, -1)
	}
	p.recorder.AddGauge(nextState, 1)

	if previous == nil || normalizeStatus(previous.Status) != statusCompleted {
		if duration, ok := runDurationSeconds(nextState); ok {
			p.recorder.ObserveDuration(nextState, duration)
		}
	}
}

func (p *runTransitionProcessor) logError(message string, id int, err error) {
	if p.logger == nil {
		return
	}

	p.logger.Error(message, zap.String("entity", p.entityName), zap.Int("id", id), zap.Error(err))
}

func (p *runTransitionProcessor) logDebug(message string, id int, state RunState) {
	if p.logger == nil {
		return
	}

	p.logger.Debug(message,
		zap.String("entity", p.entityName),
		zap.Int("id", id),
		zap.String("status", state.Status),
		zap.String("conclusion", state.Conclusion),
	)
}

func runDurationSeconds(state RunState) (float64, bool) {
	if normalizeStatus(state.Status) != statusCompleted {
		return 0, false
	}

	startedAt, startedOK := parseMetricTime(state.StartedAt)
	endedAt, endedOK := parseMetricTime(state.EndedAt)
	if !startedOK || !endedOK || endedAt.Before(startedAt) {
		return 0, false
	}

	return endedAt.Sub(startedAt).Seconds(), true
}

type runStoreAdapter struct {
	methods runStoreMethods
}

func (a runStoreAdapter) GetRunState(ctx context.Context, id int) (RunState, bool, error) {
	return a.methods.get(ctx, id)
}

func (a runStoreAdapter) UpdateRunState(ctx context.Context, id int, state RunState) error {
	return a.methods.update(ctx, id, state)
}

type prometheusRunTransitionRecorder struct {
	metrics runMetricSet
}

func (r prometheusRunTransitionRecorder) RecordStatus(state RunState) {
	r.metrics.statusCounter.WithLabelValues(
		state.Repository,
		state.Branch,
		state.Name,
		state.Status,
		state.Conclusion,
	).Inc()
}

func (r prometheusRunTransitionRecorder) AddGauge(state RunState, delta float64) {
	switch normalizeStatus(state.Status) {
	case statusQueued:
		r.metrics.queuedGauge.WithLabelValues(state.Repository, state.Branch, state.Name).Add(delta)
	case statusInProgress:
		r.metrics.inProgressGauge.WithLabelValues(state.Repository, state.Branch, state.Name).Add(delta)
	case statusCompleted:
		r.metrics.completedGauge.WithLabelValues(state.Repository, state.Branch, state.Conclusion, state.Name).Add(delta)
	}
}

func (r prometheusRunTransitionRecorder) ObserveDuration(state RunState, durationSeconds float64) {
	r.metrics.durationHistogram.WithLabelValues(
		state.Repository,
		state.Branch,
		state.Name,
		state.Status,
		state.Conclusion,
	).Observe(durationSeconds)
}
