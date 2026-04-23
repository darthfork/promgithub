package main

import "context"

type StateStore interface {
	MarkDeliveryProcessed(ctx context.Context, deliveryID string) (bool, error)
	GetWorkflowRun(ctx context.Context, runID int) (RunState, bool, error)
	UpdateWorkflowRun(ctx context.Context, runID int, state RunState) error
	GetWorkflowJob(ctx context.Context, jobID int) (RunState, bool, error)
	UpdateWorkflowJob(ctx context.Context, jobID int, state RunState) error
	Close() error
}
