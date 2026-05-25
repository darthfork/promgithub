package main

import "context"

type deliveryStateBackend interface {
	MarkDeliveryProcessed(ctx context.Context, deliveryID string) (bool, error)
}

type workflowRunStateBackend interface {
	GetWorkflowRun(ctx context.Context, runID int) (RunState, bool, error)
	UpdateWorkflowRun(ctx context.Context, runID int, state RunState) error
}

type workflowJobStateBackend interface {
	GetWorkflowJob(ctx context.Context, jobID int) (RunState, bool, error)
	UpdateWorkflowJob(ctx context.Context, jobID int, state RunState) error
}
