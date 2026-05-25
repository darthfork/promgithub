package main

import (
	"context"
	"testing"
)

const testConclusionSuccess = "success"

type inMemoryStateStore struct {
	deliveries map[string]struct{}
	workflow   map[int]RunState
	jobs       map[int]RunState
}

func newInMemoryStateStore() *inMemoryStateStore {
	return &inMemoryStateStore{
		deliveries: map[string]struct{}{},
		workflow:   map[int]RunState{},
		jobs:       map[int]RunState{},
	}
}

func useInMemoryStateBackends(t *testing.T) {
	t.Helper()

	store := newInMemoryStateStore()
	useStateBackends(t, store, store, store)
}

func useStateBackends(
	t *testing.T,
	deliveryStore deliveryStateBackend,
	workflowRunStore workflowRunStateBackend,
	workflowJobStore workflowJobStateBackend,
) {
	t.Helper()

	oldDeliveryStore := deliveryStateStore
	oldWorkflowRunStore := workflowRunStateStore
	oldWorkflowJobStore := workflowJobStateStore
	deliveryStateStore = deliveryStore
	workflowRunStateStore = workflowRunStore
	workflowJobStateStore = workflowJobStore
	t.Cleanup(func() {
		deliveryStateStore = oldDeliveryStore
		workflowRunStateStore = oldWorkflowRunStore
		workflowJobStateStore = oldWorkflowJobStore
	})
}

func (s *inMemoryStateStore) MarkDeliveryProcessed(_ context.Context, deliveryID string) (bool, error) {
	if _, ok := s.deliveries[deliveryID]; ok {
		return false, nil
	}
	s.deliveries[deliveryID] = struct{}{}
	return true, nil
}

func (s *inMemoryStateStore) GetWorkflowRun(_ context.Context, runID int) (RunState, bool, error) {
	state, ok := s.workflow[runID]
	return state, ok, nil
}

func (s *inMemoryStateStore) UpdateWorkflowRun(_ context.Context, runID int, state RunState) error {
	s.workflow[runID] = state
	return nil
}

func (s *inMemoryStateStore) GetWorkflowJob(_ context.Context, jobID int) (RunState, bool, error) {
	state, ok := s.jobs[jobID]
	return state, ok, nil
}

func (s *inMemoryStateStore) UpdateWorkflowJob(_ context.Context, jobID int, state RunState) error {
	s.jobs[jobID] = state
	return nil
}

func (s *inMemoryStateStore) Close() error {
	return nil
}
