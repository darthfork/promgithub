//go:build !integration

package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func computeHMAC(message, secret []byte) string {
	h := hmac.New(sha256.New, secret)
	h.Write(message)
	return "sha256=" + hex.EncodeToString(h.Sum(nil))
}

func resetWebhookTestState() {
	workflowStatusCounter.Reset()
	workflowDurationHistogram.Reset()
	workflowQueuedGauge.Reset()
	workflowInProgressGauge.Reset()
	workflowCompletedGauge.Reset()
	jobStatusCounter.Reset()
	jobDurationHistogram.Reset()
	jobQueuedGauge.Reset()
	jobInProgressGauge.Reset()
	jobCompletedGauge.Reset()
	commitPushedCounter.Reset()
	pullRequestCounter.Reset()
	asyncProcessedEventsCounter.Reset()
	asyncEventsDroppedCounter.Reset()
	asyncProcessingFailuresCounter.Reset()
	asyncProcessingDurationHistogram.Reset()
	duplicateDeliveriesSeenCounter.Reset()
	duplicateDeliveriesDroppedCounter.Reset()
	asyncQueueDepthGauge.Set(0)
	asyncQueueCapacityGauge.Set(0)
	asyncWorkerCountGauge.Set(0)
	githubWebhookSecret = []byte("test-secret")
	logger = zap.NewNop()
	stateStore = nil
	eventProcessor = nil
	deliveryDeduperCache = newDeliveryDeduper(defaultDeliveryRetention, defaultDeliveryCacheEntries)
}

func sendTestRequest(payload []byte, eventType, deliveryID string) *httptest.ResponseRecorder {
	signature := computeHMAC(payload, githubWebhookSecret)
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBuffer(payload))
	req.Header.Set("X-Hub-Signature-256", signature)
	req.Header.Set("X-GitHub-Event", eventType)
	if deliveryID != "" {
		req.Header.Set("X-GitHub-Delivery", deliveryID)
	}

	recorder := httptest.NewRecorder()
	handler := http.HandlerFunc(githubEventsHandler)
	handler.ServeHTTP(recorder, req)

	return recorder
}

func sendWorkflowWebhookRequest(t *testing.T, serverURL string, payload []byte, deliveryID string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, serverURL+"/webhook", bytes.NewBuffer(payload))
	if err != nil {
		t.Fatalf("Failed to create webhook request: %v", err)
	}

	req.Header.Set("X-Hub-Signature-256", computeHMAC(payload, githubWebhookSecret))
	req.Header.Set("X-GitHub-Event", "workflow_run")
	if deliveryID != "" {
		req.Header.Set("X-GitHub-Delivery", deliveryID)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to send webhook request: %v", err)
	}

	return resp
}

func TestValidateHMAC(t *testing.T) {
	resetWebhookTestState()

	body := []byte("test body")
	signature := computeHMAC(body, githubWebhookSecret)

	valid := validateHMAC(body, signature, githubWebhookSecret)
	assert.True(t, valid)
}

func TestValidWorkflowPayload(t *testing.T) {
	resetWebhookTestState()

	dir, err := os.ReadDir("../test_data")
	if err != nil {
		t.Fatalf("Failed to read test data directory: %v", err)
	}
	for _, file := range dir {
		if file.IsDir() {
			continue
		}
		body, err := os.ReadFile("../test_data/" + file.Name())
		if err != nil {
			t.Fatalf("Failed to read test data file: %v", err)
		}
		eventType := strings.TrimSuffix(file.Name(), ".json")
		recorder := sendTestRequest(body, eventType, eventType+"-delivery")
		assert.Equal(t, http.StatusOK, recorder.Code)
	}
}

func TestInvalidSignature(t *testing.T) {
	resetWebhookTestState()

	body, err := os.ReadFile("../test_data/workflow_run.json")
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBuffer(body))
	req.Header.Set("X-Hub-Signature-256", "invalid_signature")
	req.Header.Set("X-GitHub-Event", "workflow_run")

	handler := http.HandlerFunc(githubEventsHandler)
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestMissingSignature(t *testing.T) {
	resetWebhookTestState()

	body, err := os.ReadFile("../test_data/workflow_run.json")
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBuffer(body))
	req.Header.Set("X-GitHub-Event", "workflow_run")

	handler := http.HandlerFunc(githubEventsHandler)
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestUnknownEvent(t *testing.T) {
	resetWebhookTestState()

	body, err := os.ReadFile("../test_data/workflow_run.json")
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}
	recorder := sendTestRequest(body, "unknown_event", "unknown-delivery")
	assert.Equal(t, http.StatusOK, recorder.Code)
}

func TestDuplicateDeliveryIsIgnored(t *testing.T) {
	resetWebhookTestState()
	stateStore = newInMemoryStateStore()
	defer func() { stateStore = nil }()

	body, err := os.ReadFile("../test_data/workflow_run.json")
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}

	recorder := sendTestRequest(body, "workflow_run", "delivery-1")
	assert.Equal(t, http.StatusOK, recorder.Code)

	recorder = sendTestRequest(body, "workflow_run", "delivery-1")
	assert.Equal(t, http.StatusOK, recorder.Code)

	if err := testutil.CollectAndCompare(workflowStatusCounter, strings.NewReader(`
		# HELP promgithub_workflow_status Total number of workflow runs with status
		# TYPE promgithub_workflow_status counter
		promgithub_workflow_status{branch="main",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed"} 1
	`)); err != nil {
		t.Fatalf("unexpected workflow metrics: %v", err)
	}

	if err := testutil.CollectAndCompare(duplicateDeliveriesSeenCounter, strings.NewReader(`
		# HELP promgithub_duplicate_deliveries_seen_total Total number of duplicate GitHub webhook deliveries observed
		# TYPE promgithub_duplicate_deliveries_seen_total counter
		promgithub_duplicate_deliveries_seen_total{event_type="workflow_run"} 1
	`)); err != nil {
		t.Fatalf("unexpected duplicate seen metrics: %v", err)
	}

	if err := testutil.CollectAndCompare(duplicateDeliveriesDroppedCounter, strings.NewReader(`
		# HELP promgithub_duplicate_deliveries_dropped_total Total number of duplicate GitHub webhook deliveries dropped
		# TYPE promgithub_duplicate_deliveries_dropped_total counter
		promgithub_duplicate_deliveries_dropped_total{event_type="workflow_run"} 1
	`)); err != nil {
		t.Fatalf("unexpected duplicate dropped metrics: %v", err)
	}
}

func TestDuplicateDeliveryIsDroppedInMetricsEndpoint(t *testing.T) {
	resetWebhookTestState()
	stateStore = newInMemoryStateStore()
	defer func() { stateStore = nil }()

	body, err := os.ReadFile("../test_data/workflow_run.json")
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}

	server := httptest.NewServer(setupRouter(logger, defaultServiceMetrics, prometheus.DefaultGatherer))
	defer server.Close()

	resp := sendWorkflowWebhookRequest(t, server.URL, body, "delivery-1")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	resp = sendWorkflowWebhookRequest(t, server.URL, body, "delivery-1")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	if err := testutil.ScrapeAndCompare(server.URL+"/metrics", strings.NewReader(`
		# HELP promgithub_duplicate_deliveries_dropped_total Total number of duplicate GitHub webhook deliveries dropped
		# TYPE promgithub_duplicate_deliveries_dropped_total counter
		promgithub_duplicate_deliveries_dropped_total{event_type="workflow_run"} 1
		# HELP promgithub_duplicate_deliveries_seen_total Total number of duplicate GitHub webhook deliveries observed
		# TYPE promgithub_duplicate_deliveries_seen_total counter
		promgithub_duplicate_deliveries_seen_total{event_type="workflow_run"} 1
		# HELP promgithub_workflow_status Total number of workflow runs with status
		# TYPE promgithub_workflow_status counter
		promgithub_workflow_status{branch="main",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed"} 1
	`), "promgithub_duplicate_deliveries_dropped_total", "promgithub_duplicate_deliveries_seen_total", "promgithub_workflow_status"); err != nil {
		t.Fatalf("unexpected metrics: %v", err)
	}
}

func TestWebhookIsAcceptedWhenAsyncProcessorEnabled(t *testing.T) {
	resetWebhookTestState()

	body, err := os.ReadFile("../test_data/workflow_run.json")
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}

	processed := make(chan struct{}, 1)
	eventProcessor = newAsyncEventProcessor(asyncProcessorConfig{WorkerCount: 1, QueueSize: 1}, zap.NewNop())
	eventProcessor.processFn["workflow_run"] = func(_ context.Context, _ []byte) {
		processed <- struct{}{}
	}
	eventProcessor.Start()
	defer func() {
		eventProcessor.Stop()
		eventProcessor = nil
	}()

	recorder := sendTestRequest(body, "workflow_run", "delivery-1")
	assert.Equal(t, http.StatusAccepted, recorder.Code)

	select {
	case <-processed:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for async processing")
	}
}

func TestWebhookReturnsUnavailableWhenAsyncQueueIsFull(t *testing.T) {
	resetWebhookTestState()

	body, err := os.ReadFile("../test_data/workflow_run.json")
	if err != nil {
		t.Fatalf("Failed to read test data file: %v", err)
	}

	blocker := make(chan struct{})
	eventProcessor = newAsyncEventProcessor(asyncProcessorConfig{WorkerCount: 1, QueueSize: 1}, zap.NewNop())
	eventProcessor.processFn["workflow_run"] = func(_ context.Context, _ []byte) {
		<-blocker
	}
	defer func() {
		close(blocker)
		eventProcessor.Stop()
		eventProcessor = nil
	}()

	if err := eventProcessor.Enqueue(context.Background(), "workflow_run", []byte(`{"id":1}`)); err != nil {
		t.Fatalf("unexpected enqueue error: %v", err)
	}

	recorder := sendTestRequest(body, "workflow_run", "delivery-2")
	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}
