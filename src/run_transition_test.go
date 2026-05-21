//go:build !integration

package main

import (
	"context"
	"errors"
	"testing"
)

const (
	runTransitionTestRepository = "user/repo"
	runTransitionTestBranch     = "main"
	runTransitionTestName       = "CI"
	runTransitionStartedAt      = "2023-01-01T00:00:00Z"
	runTransitionCompletedAt    = "2023-01-01T01:00:00Z"
)

type fakeRunTransitionStore struct {
	state       RunState
	found       bool
	getErr      error
	updateErr   error
	updateCalls int
}

func (s *fakeRunTransitionStore) GetRunState(_ context.Context, _ int) (RunState, bool, error) {
	if s.getErr != nil {
		return RunState{}, false, s.getErr
	}

	return s.state, s.found, nil
}

func (s *fakeRunTransitionStore) UpdateRunState(_ context.Context, _ int, state RunState) error {
	if s.updateErr != nil {
		return s.updateErr
	}

	s.state = state
	s.found = true
	s.updateCalls++
	return nil
}

type fakeRunTransitionRecorder struct {
	statuses  []RunState
	gauges    []recordedGaugeDelta
	durations []float64
}

type recordedGaugeDelta struct {
	state RunState
	delta float64
}

func (r *fakeRunTransitionRecorder) RecordStatus(state RunState) {
	r.statuses = append(r.statuses, state)
}

func (r *fakeRunTransitionRecorder) AddGauge(state RunState, delta float64) {
	r.gauges = append(r.gauges, recordedGaugeDelta{state: state, delta: delta})
}

func (r *fakeRunTransitionRecorder) ObserveDuration(_ RunState, durationSeconds float64) {
	r.durations = append(r.durations, durationSeconds)
}

func TestRunTransitionProcessorAppliesLifecycleThroughInterface(t *testing.T) {
	store := &fakeRunTransitionStore{}
	recorder := &fakeRunTransitionRecorder{}
	processor := &runTransitionProcessor{
		store:    store,
		recorder: recorder,
	}

	queued := runTransitionDetails(statusQueued, "", runTransitionStartedAt)
	if result := processor.Apply(context.Background(), 1001, queued); !result.Applied {
		t.Fatalf("expected queued transition to apply, got %#v", result)
	}

	inProgress := runTransitionDetails(statusInProgress, "", runTransitionStartedAt)
	if result := processor.Apply(context.Background(), 1001, inProgress); !result.Applied {
		t.Fatalf("expected in-progress transition to apply, got %#v", result)
	}

	completed := runTransitionDetails(statusCompleted, testConclusionSuccess, runTransitionCompletedAt)
	if result := processor.Apply(context.Background(), 1001, completed); !result.Applied {
		t.Fatalf("expected completed transition to apply, got %#v", result)
	}

	if store.updateCalls != 3 {
		t.Fatalf("expected 3 stored transitions, got %d", store.updateCalls)
	}
	if store.state.Status != statusCompleted || store.state.Conclusion != testConclusionSuccess {
		t.Fatalf("expected final completed state, got %#v", store.state)
	}
	if len(recorder.statuses) != 3 {
		t.Fatalf("expected 3 status records, got %d", len(recorder.statuses))
	}

	assertGaugeDelta(t, recorder.gauges, 0, statusQueued, 1)
	assertGaugeDelta(t, recorder.gauges, 1, statusQueued, -1)
	assertGaugeDelta(t, recorder.gauges, 2, statusInProgress, 1)
	assertGaugeDelta(t, recorder.gauges, 3, statusInProgress, -1)
	assertGaugeDelta(t, recorder.gauges, 4, statusCompleted, 1)

	if len(recorder.durations) != 1 || recorder.durations[0] != 3600 {
		t.Fatalf("expected one 3600s duration observation, got %#v", recorder.durations)
	}
}

func TestRunTransitionProcessorDoesNotRecordMetricsWhenStoreFails(t *testing.T) {
	store := &fakeRunTransitionStore{getErr: errors.New("store unavailable")}
	recorder := &fakeRunTransitionRecorder{}
	processor := &runTransitionProcessor{
		store:    store,
		recorder: recorder,
	}

	result := processor.Apply(context.Background(), 1001, runTransitionDetails(statusCompleted, testConclusionSuccess, runTransitionCompletedAt))

	if result.Err == nil {
		t.Fatal("expected store error")
	}
	if len(recorder.statuses) != 0 || len(recorder.gauges) != 0 || len(recorder.durations) != 0 {
		t.Fatalf("expected no metric records, got %#v", recorder)
	}
	if store.updateCalls != 0 {
		t.Fatalf("expected no store updates, got %d", store.updateCalls)
	}
}

func runTransitionDetails(status, conclusion, endedAt string) runMetricDetails {
	return runMetricDetails{
		repository: runTransitionTestRepository,
		branch:     runTransitionTestBranch,
		name:       runTransitionTestName,
		status:     status,
		conclusion: conclusion,
		startedAt:  runTransitionStartedAt,
		endedAt:    endedAt,
	}
}

func assertGaugeDelta(t *testing.T, gauges []recordedGaugeDelta, index int, status string, delta float64) {
	t.Helper()

	if len(gauges) <= index {
		t.Fatalf("expected gauge delta at index %d, got %#v", index, gauges)
	}
	if gauges[index].state.Status != status || gauges[index].delta != delta {
		t.Fatalf("expected gauge %d to be %s/%v, got %#v", index, status, delta, gauges[index])
	}
}
