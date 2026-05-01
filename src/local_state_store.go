package main

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

type localStateStore struct {
	mu          sync.Mutex
	deliveryTTL time.Duration
	deliveries  map[string]time.Time
	workflows   map[int]RunState
	jobs        map[int]RunState
}

func newLocalStateStore(deliveryTTL time.Duration) *localStateStore {
	if deliveryTTL <= 0 {
		deliveryTTL = defaultRedisDeliveryTTL
	}

	return &localStateStore{
		deliveryTTL: deliveryTTL,
		deliveries:  make(map[string]time.Time),
		workflows:   make(map[int]RunState),
		jobs:        make(map[int]RunState),
	}
}

func (s *localStateStore) MarkDeliveryProcessed(_ context.Context, deliveryID string) (bool, error) {
	deliveryID = strings.TrimSpace(deliveryID)
	if deliveryID == "" {
		return false, errors.New("delivery id is required")
	}

	now := time.Now()
	expiresAt := now.Add(s.deliveryTTL)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.pruneExpiredDeliveriesLocked(now)
	if existingExpiresAt, ok := s.deliveries[deliveryID]; ok && existingExpiresAt.After(now) {
		return false, nil
	}

	s.deliveries[deliveryID] = expiresAt
	return true, nil
}

func (s *localStateStore) GetWorkflowRun(_ context.Context, runID int) (RunState, bool, error) {
	if runID == 0 {
		return RunState{}, false, errors.New("workflow run id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.workflows[runID]
	return state, ok, nil
}

func (s *localStateStore) UpdateWorkflowRun(_ context.Context, runID int, state RunState) error {
	if runID == 0 {
		return errors.New("workflow run id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.workflows[runID] = state
	return nil
}

func (s *localStateStore) GetWorkflowJob(_ context.Context, jobID int) (RunState, bool, error) {
	if jobID == 0 {
		return RunState{}, false, errors.New("workflow job id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.jobs[jobID]
	return state, ok, nil
}

func (s *localStateStore) UpdateWorkflowJob(_ context.Context, jobID int, state RunState) error {
	if jobID == 0 {
		return errors.New("workflow job id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.jobs[jobID] = state
	return nil
}

func (s *localStateStore) Close() error {
	return nil
}

func (s *localStateStore) pruneExpiredDeliveriesLocked(now time.Time) {
	for deliveryID, expiresAt := range s.deliveries {
		if !expiresAt.After(now) {
			delete(s.deliveries, deliveryID)
		}
	}
}
