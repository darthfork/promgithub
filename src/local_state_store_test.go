//go:build !integration

package main

import (
	"context"
	"testing"
	"time"
)

func TestLocalStateStoreDeduplicatesDeliveryIDsUntilTTLExpires(t *testing.T) {
	store := newLocalStateStore(20 * time.Millisecond)
	ctx := context.Background()

	processed, err := store.MarkDeliveryProcessed(ctx, "delivery-1")
	if err != nil {
		t.Fatalf("unexpected error marking first delivery: %v", err)
	}
	if !processed {
		t.Fatal("expected first delivery to be processed")
	}

	processed, err = store.MarkDeliveryProcessed(ctx, "delivery-1")
	if err != nil {
		t.Fatalf("unexpected error marking duplicate delivery: %v", err)
	}
	if processed {
		t.Fatal("expected duplicate delivery to be ignored")
	}

	time.Sleep(30 * time.Millisecond)
	processed, err = store.MarkDeliveryProcessed(ctx, "delivery-1")
	if err != nil {
		t.Fatalf("unexpected error marking expired delivery: %v", err)
	}
	if !processed {
		t.Fatal("expected delivery to be processed again after TTL expires")
	}
}

func TestLocalStateStoreTracksWorkflowAndJobState(t *testing.T) {
	store := newLocalStateStore(time.Hour)
	ctx := context.Background()

	workflow := RunState{Repository: "user/repo", Branch: "main", Name: "CI", Status: statusInProgress}
	if err := store.UpdateWorkflowRun(ctx, 101, workflow); err != nil {
		t.Fatalf("failed to update workflow state: %v", err)
	}
	gotWorkflow, found, err := store.GetWorkflowRun(ctx, 101)
	if err != nil {
		t.Fatalf("failed to get workflow state: %v", err)
	}
	if !found || gotWorkflow != workflow {
		t.Fatalf("unexpected workflow state found=%v state=%+v", found, gotWorkflow)
	}

	job := RunState{Repository: "user/repo", Branch: "main", Name: "CI", Status: statusQueued}
	if err := store.UpdateWorkflowJob(ctx, 202, job); err != nil {
		t.Fatalf("failed to update job state: %v", err)
	}
	gotJob, found, err := store.GetWorkflowJob(ctx, 202)
	if err != nil {
		t.Fatalf("failed to get job state: %v", err)
	}
	if !found || gotJob != job {
		t.Fatalf("unexpected job state found=%v state=%+v", found, gotJob)
	}
}
