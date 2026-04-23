package main

import "context"

type StateStore interface {
	MarkDeliveryProcessed(ctx context.Context, deliveryID string) (bool, error)
	UpdateWorkflowRun(ctx context.Context, runID int, state RunState) error
	UpdateWorkflowJob(ctx context.Context, jobID int, state RunState) error
	Close() error
}
