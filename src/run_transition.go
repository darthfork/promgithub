package main

import (
	"context"

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

type runTransitionStore interface {
	GetRunState(context.Context, int) (RunState, bool, error)
	UpdateRunState(context.Context, int, RunState) error
}

type runTransitionRecorder interface {
	RecordStatus(RunState)
	AddGauge(RunState, float64)
	ObserveDuration(RunState, float64)
}

type runMetricRecorder interface {
	RecordRunStatus(runMetricKind, RunState)
	AddRunGauge(runMetricKind, RunState, float64)
	ObserveRunDuration(runMetricKind, RunState, float64)
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

type workflowRunStateAdapter struct {
	store workflowRunStateBackend
}

func (a workflowRunStateAdapter) GetRunState(ctx context.Context, id int) (RunState, bool, error) {
	return a.store.GetWorkflowRun(ctx, id)
}

func (a workflowRunStateAdapter) UpdateRunState(ctx context.Context, id int, state RunState) error {
	return a.store.UpdateWorkflowRun(ctx, id, state)
}

type workflowJobStateAdapter struct {
	store workflowJobStateBackend
}

func (a workflowJobStateAdapter) GetRunState(ctx context.Context, id int) (RunState, bool, error) {
	return a.store.GetWorkflowJob(ctx, id)
}

func (a workflowJobStateAdapter) UpdateRunState(ctx context.Context, id int, state RunState) error {
	return a.store.UpdateWorkflowJob(ctx, id, state)
}

type metricRunTransitionRecorder struct {
	kind     runMetricKind
	recorder runMetricRecorder
}

func (r metricRunTransitionRecorder) RecordStatus(state RunState) {
	r.recorder.RecordRunStatus(r.kind, state)
}

func (r metricRunTransitionRecorder) AddGauge(state RunState, delta float64) {
	r.recorder.AddRunGauge(r.kind, state, delta)
}

func (r metricRunTransitionRecorder) ObserveDuration(state RunState, durationSeconds float64) {
	r.recorder.ObserveRunDuration(r.kind, state, durationSeconds)
}
