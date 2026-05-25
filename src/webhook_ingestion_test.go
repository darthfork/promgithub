//go:build !integration

package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

type fakeDeliveryMarker struct {
	created bool
	err     error
	calls   []string
}

func (f *fakeDeliveryMarker) MarkDeliveryProcessed(_ context.Context, deliveryID string) (bool, error) {
	f.calls = append(f.calls, deliveryID)
	return f.created, f.err
}

type fakeWebhookQueue struct {
	err     error
	events  []webhookIngestionRequest
	lastCtx context.Context
}

func (f *fakeWebhookQueue) Enqueue(ctx context.Context, eventType string, body []byte) error {
	f.lastCtx = ctx
	f.events = append(f.events, webhookIngestionRequest{
		EventType: eventType,
		Body:      append([]byte(nil), body...),
	})
	return f.err
}

type fakeWebhookDispatcher struct {
	events []webhookIngestionRequest
}

func (f *fakeWebhookDispatcher) Dispatch(_ context.Context, eventType string, body []byte) bool {
	f.events = append(f.events, webhookIngestionRequest{
		EventType: eventType,
		Body:      append([]byte(nil), body...),
	})
	return eventType == githubEventWorkflowRun
}

type fakeWebhookMetrics struct {
	duplicates []string
}

func (f *fakeWebhookMetrics) RecordDuplicateDelivery(eventType string) {
	f.duplicates = append(f.duplicates, eventType)
}

func signedWorkflowRunWebhookRequest(body []byte) webhookIngestionRequest {
	return webhookIngestionRequest{
		Body:       body,
		Signature:  computeHMAC(body, []byte("test-secret")),
		EventType:  githubEventWorkflowRun,
		DeliveryID: "delivery-1",
	}
}

func newTestWebhookIngestion() (*webhookIngestion, *fakeDeliveryMarker, *fakeWebhookDispatcher, *fakeWebhookMetrics) {
	delivery := &fakeDeliveryMarker{created: true}
	dispatcher := &fakeWebhookDispatcher{}
	metrics := &fakeWebhookMetrics{}

	return &webhookIngestion{
		secret:        []byte("test-secret"),
		logger:        zap.NewNop(),
		deliveryStore: delivery,
		dispatcher:    dispatcher,
		metrics:       metrics,
	}, delivery, dispatcher, metrics
}

func TestWebhookIngestionRejectsInvalidSignature(t *testing.T) {
	ingestion, delivery, dispatcher, _ := newTestWebhookIngestion()

	result := ingestion.Accept(context.Background(), webhookIngestionRequest{
		Body:       []byte(`{"ok":true}`),
		Signature:  "sha256=bad",
		EventType:  githubEventWorkflowRun,
		DeliveryID: "delivery-1",
	})

	if result.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, result.StatusCode)
	}
	if len(delivery.calls) != 0 {
		t.Fatalf("expected no delivery calls, got %d", len(delivery.calls))
	}
	if len(dispatcher.events) != 0 {
		t.Fatalf("expected no dispatch calls, got %d", len(dispatcher.events))
	}
}

func TestWebhookIngestionDropsDuplicateBeforeDispatch(t *testing.T) {
	ingestion, delivery, dispatcher, metrics := newTestWebhookIngestion()
	delivery.created = false

	result := ingestion.Accept(context.Background(), signedWorkflowRunWebhookRequest([]byte(`{"ok":true}`)))

	if result.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, result.StatusCode)
	}
	if len(dispatcher.events) != 0 {
		t.Fatalf("expected no dispatch calls, got %d", len(dispatcher.events))
	}
	if got := metrics.duplicates; len(got) != 1 || got[0] != githubEventWorkflowRun {
		t.Fatalf("expected duplicate metric for workflow_run, got %#v", got)
	}
}

func TestWebhookIngestionReturnsServerErrorWhenDeliveryRecordingFails(t *testing.T) {
	ingestion, delivery, dispatcher, _ := newTestWebhookIngestion()
	delivery.err = errors.New("redis unavailable")

	result := ingestion.Accept(context.Background(), signedWorkflowRunWebhookRequest([]byte(`{"ok":true}`)))

	if result.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, result.StatusCode)
	}
	if len(dispatcher.events) != 0 {
		t.Fatalf("expected no dispatch calls, got %d", len(dispatcher.events))
	}
}

func TestWebhookIngestionDispatchesSynchronouslyWithoutQueue(t *testing.T) {
	ingestion, _, dispatcher, _ := newTestWebhookIngestion()
	body := []byte(`{"ok":true}`)

	result := ingestion.Accept(context.Background(), signedWorkflowRunWebhookRequest(body))

	if result.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, result.StatusCode)
	}
	if len(dispatcher.events) != 1 {
		t.Fatalf("expected one dispatch call, got %d", len(dispatcher.events))
	}
	if dispatcher.events[0].EventType != githubEventWorkflowRun || !bytes.Equal(dispatcher.events[0].Body, body) {
		t.Fatalf("unexpected dispatched event: %#v", dispatcher.events[0])
	}
}

func TestWebhookIngestionEnqueuesAcceptedEvent(t *testing.T) {
	ingestion, _, dispatcher, _ := newTestWebhookIngestion()
	queue := &fakeWebhookQueue{}
	ingestion.queue = queue
	body := []byte(`{"ok":true}`)

	result := ingestion.Accept(context.Background(), signedWorkflowRunWebhookRequest(body))

	if result.StatusCode != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, result.StatusCode)
	}
	if len(queue.events) != 1 {
		t.Fatalf("expected one queued event, got %d", len(queue.events))
	}
	if len(dispatcher.events) != 0 {
		t.Fatalf("expected no synchronous dispatch calls, got %d", len(dispatcher.events))
	}
}

func TestDefaultWebhookIngestionUsesDeliveryStateWithoutRunState(t *testing.T) {
	oldDeliveryStore := deliveryStateStore
	oldWorkflowRunStore := workflowRunStateStore
	oldWorkflowJobStore := workflowJobStateStore
	oldProcessor := eventProcessor
	oldSecret := githubWebhookSecret
	t.Cleanup(func() {
		deliveryStateStore = oldDeliveryStore
		workflowRunStateStore = oldWorkflowRunStore
		workflowJobStateStore = oldWorkflowJobStore
		eventProcessor = oldProcessor
		githubWebhookSecret = oldSecret
	})

	delivery := &fakeDeliveryMarker{created: true}
	deliveryStateStore = delivery
	workflowRunStateStore = nil
	workflowJobStateStore = nil
	eventProcessor = nil
	githubWebhookSecret = []byte("test-secret")

	ingestion := newDefaultWebhookIngestion()
	result := ingestion.Accept(context.Background(), signedWorkflowRunWebhookRequest([]byte(`{"ok":true}`)))

	if result.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, result.StatusCode)
	}
	if len(delivery.calls) != 1 || delivery.calls[0] != "delivery-1" {
		t.Fatalf("expected delivery state to record delivery-1, got %#v", delivery.calls)
	}
}

func TestWebhookIngestionReturnsUnavailableWhenQueueIsFull(t *testing.T) {
	ingestion, _, dispatcher, _ := newTestWebhookIngestion()
	ingestion.queue = &fakeWebhookQueue{err: errors.New("queue full")}

	result := ingestion.Accept(context.Background(), signedWorkflowRunWebhookRequest([]byte(`{"ok":true}`)))

	if result.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, result.StatusCode)
	}
	if len(dispatcher.events) != 0 {
		t.Fatalf("expected no synchronous dispatch calls, got %d", len(dispatcher.events))
	}
}

type fakeWebhookAcceptor struct {
	request webhookIngestionRequest
	result  webhookIngestionResult
}

func (f *fakeWebhookAcceptor) Accept(_ context.Context, request webhookIngestionRequest) webhookIngestionResult {
	f.request = request
	return f.result
}

func TestWebhookHTTPHandlerAdaptsHeadersAndBody(t *testing.T) {
	acceptor := &fakeWebhookAcceptor{
		result: webhookIngestionResult{StatusCode: http.StatusAccepted},
	}
	handler := webhookHTTPHandler(acceptor, zap.NewNop())
	body := []byte(`{"ok":true}`)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", "sha256=test")
	req.Header.Set("X-GitHub-Event", githubEventWorkflowRun)
	req.Header.Set("X-GitHub-Delivery", "delivery-1")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, recorder.Code)
	}
	if acceptor.request.Signature != "sha256=test" {
		t.Fatalf("unexpected signature %q", acceptor.request.Signature)
	}
	if acceptor.request.EventType != githubEventWorkflowRun {
		t.Fatalf("unexpected event type %q", acceptor.request.EventType)
	}
	if acceptor.request.DeliveryID != "delivery-1" {
		t.Fatalf("unexpected delivery id %q", acceptor.request.DeliveryID)
	}
	if !bytes.Equal(acceptor.request.Body, body) {
		t.Fatalf("unexpected body %q", string(acceptor.request.Body))
	}
}
