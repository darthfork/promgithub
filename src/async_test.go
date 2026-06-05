//go:build !integration

package main

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.uber.org/zap"
)

func TestAsyncProcessorEnqueueAndProcess(t *testing.T) {
	asyncProcessedEventsCounter.Reset()
	asyncQueueDroppedCounter.Reset()
	asyncUnsupportedEventsCounter.Reset()
	asyncProcessingFailuresCounter.Reset()
	asyncQueueDepthGauge.Set(0)
	asyncQueueCapacityGauge.Set(0)
	asyncWorkerCountGauge.Set(0)

	processed := make(chan struct{}, 1)
	processor := newAsyncEventProcessor(asyncProcessorConfig{WorkerCount: 1, QueueSize: 2}, zap.NewNop())
	useWorkflowRunAsyncEventHandler(processor, func(_ context.Context, _ []byte) {
		processed <- struct{}{}
	})
	processor.Start()
	defer processor.Stop()

	if err := processor.Enqueue(context.Background(), "workflow_run", []byte(`{}`)); err != nil {
		t.Fatalf("unexpected enqueue error: %v", err)
	}

	select {
	case <-processed:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for async processing")
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		if got := testutil.ToFloat64(asyncProcessedEventsCounter.WithLabelValues("workflow_run")); got == 1 {
			break
		}
		if time.Now().After(deadline) {
			got := testutil.ToFloat64(asyncProcessedEventsCounter.WithLabelValues("workflow_run"))
			t.Fatalf("expected processed counter to be 1, got %v", got)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestAsyncProcessorUsesSharedGitHubEventDispatcher(t *testing.T) {
	asyncProcessedEventsCounter.Reset()
	asyncUnsupportedEventsCounter.Reset()

	processed := make(chan struct{}, 1)
	processor := newAsyncEventProcessor(asyncProcessorConfig{WorkerCount: 1, QueueSize: 2}, zap.NewNop())
	processor.dispatcher = githubEventDispatcher{
		handlers: map[string]eventHandler{
			githubEventWorkflowRun: func(_ context.Context, _ []byte) {
				processed <- struct{}{}
			},
		},
	}
	processor.Start()
	defer processor.Stop()

	if err := processor.Enqueue(context.Background(), githubEventWorkflowRun, []byte(`{}`)); err != nil {
		t.Fatalf("unexpected enqueue error: %v", err)
	}
	if err := processor.Enqueue(context.Background(), "unknown_event", []byte(`{}`)); err != nil {
		t.Fatalf("unexpected enqueue error: %v", err)
	}

	select {
	case <-processed:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for async processing")
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		if got := testutil.ToFloat64(asyncUnsupportedEventsCounter.WithLabelValues("unknown_event")); got == 1 {
			break
		}
		if time.Now().After(deadline) {
			got := testutil.ToFloat64(asyncUnsupportedEventsCounter.WithLabelValues("unknown_event"))
			t.Fatalf("expected unsupported counter to be 1, got %v", got)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestAsyncProcessorDropsWhenQueueFull(t *testing.T) {
	asyncQueueDroppedCounter.Reset()
	asyncQueueDepthGauge.Set(0)

	blocker := make(chan struct{})
	processor := newAsyncEventProcessor(asyncProcessorConfig{WorkerCount: 1, QueueSize: 1}, zap.NewNop())
	useWorkflowRunAsyncEventHandler(processor, func(_ context.Context, _ []byte) {
		<-blocker
	})
	processor.Start()
	defer func() {
		close(blocker)
		processor.Stop()
	}()

	if err := processor.Enqueue(context.Background(), "workflow_run", []byte(`{"id":1}`)); err != nil {
		t.Fatalf("unexpected enqueue error: %v", err)
	}
	if err := processor.Enqueue(context.Background(), "workflow_run", []byte(`{"id":2}`)); err == nil {
		t.Fatal("expected queue full error")
	}

	if got := testutil.ToFloat64(asyncQueueDroppedCounter.WithLabelValues("workflow_run")); got != 1 {
		t.Fatalf("expected dropped counter to be 1, got %v", got)
	}
}
