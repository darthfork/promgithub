//go:build integration

package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const (
	integrationRunStartedAt   = "2023-01-01T00:00:00Z"
	integrationRunCompletedAt = "2023-01-01T01:00:00Z"
)

func TestIntegrationWebhookMetrics(t *testing.T) {
	testCases := []struct {
		name           string
		eventType      string
		fixture        string
		expectedStatus int
		expectedMetric string
	}{
		{
			name:           "workflow run updates workflow metrics",
			eventType:      "workflow_run",
			fixture:        "workflow_run.json",
			expectedStatus: http.StatusAccepted,
			expectedMetric: `promgithub_workflow_status{branch="main",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed"} 1`,
		},
		{
			name:           "workflow job updates job metrics",
			eventType:      "workflow_job",
			fixture:        "workflow_job.json",
			expectedStatus: http.StatusAccepted,
			expectedMetric: `promgithub_job_status{branch="main",job_conclusion="success",job_status="completed",repository="user/repo",workflow_name="CI"} 1`,
		},
		{
			name:           "push updates commit metrics",
			eventType:      "push",
			fixture:        "push.json",
			expectedStatus: http.StatusAccepted,
			expectedMetric: `promgithub_commit_pushed{repository="user/repo"} 1`,
		},
		{
			name:           "pull request updates pull request metrics",
			eventType:      "pull_request",
			fixture:        "pull_request.json",
			expectedStatus: http.StatusAccepted,
			expectedMetric: `promgithub_pull_request{base_branch="main",pull_request_status="opened",repository="user/repo"} 1`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := newIntegrationTestServer(t)
			defer server.Close()

			body := mustReadFixture(t, tc.fixture)
			resp := sendWebhookRequest(t, server.URL, tc.eventType, body, "delivery-1")
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != tc.expectedStatus {
				t.Fatalf("expected status %d, got %d", tc.expectedStatus, resp.StatusCode)
			}

			metrics := waitForMetricsSubstring(t, server.URL, tc.expectedMetric)
			if !strings.Contains(metrics, tc.expectedMetric) {
				t.Fatalf("expected metrics to contain %q, got:\n%s", tc.expectedMetric, metrics)
			}
		})
	}
}

func TestIntegrationWebhookInvalidSignature(t *testing.T) {
	server := newIntegrationTestServer(t)
	defer server.Close()

	body := mustReadFixture(t, "workflow_run.json")
	req, err := http.NewRequest(http.MethodPost, server.URL+"/webhook", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("X-Hub-Signature-256", "sha256=invalid")
	req.Header.Set("X-GitHub-Event", "workflow_run")
	req.Header.Set("X-GitHub-Delivery", "delivery-invalid")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, resp.StatusCode)
	}

	metrics := mustFetchMetrics(t, server.URL)
	if strings.Contains(metrics, `promgithub_workflow_status{branch="main",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed"} 1`) {
		t.Fatalf("workflow metrics changed after invalid signature:\n%s", metrics)
	}
}

func TestIntegrationWebhookUnsupportedEvent(t *testing.T) {
	server := newIntegrationTestServer(t)
	defer server.Close()

	body := mustReadFixture(t, "workflow_run.json")
	resp := sendWebhookRequest(t, server.URL, "unknown_event", body, "delivery-unsupported")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, resp.StatusCode)
	}

	metrics := waitForMetricsSubstring(t, server.URL, `promgithub_event_unsupported_total{event_type="unknown_event"} 1`)
	if strings.Contains(metrics, `promgithub_workflow_status{branch="main",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed"} 1`) {
		t.Fatalf("unsupported event unexpectedly updated workflow metrics:\n%s", metrics)
	}
	if !strings.Contains(metrics, `promgithub_event_unsupported_total{event_type="unknown_event"} 1`) {
		t.Fatalf("expected unsupported event metric, got:\n%s", metrics)
	}
}

func TestIntegrationHealthAndMetricsEndpoints(t *testing.T) {
	server := newIntegrationTestServer(t)
	defer server.Close()

	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("failed to get health endpoint: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected health status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	metricsResp, err := http.Get(server.URL + "/metrics")
	if err != nil {
		t.Fatalf("failed to get metrics endpoint: %v", err)
	}
	defer func() { _ = metricsResp.Body.Close() }()

	if metricsResp.StatusCode != http.StatusOK {
		t.Fatalf("expected metrics status %d, got %d", http.StatusOK, metricsResp.StatusCode)
	}
}

func TestIntegrationDuplicateDeliveryDoesNotInflateMetrics(t *testing.T) {
	server := newIntegrationTestServer(t)
	defer server.Close()

	body := mustReadFixture(t, "workflow_run.json")

	resp := sendWebhookRequest(t, server.URL, "workflow_run", body, "delivery-duplicate")
	if resp.StatusCode != http.StatusAccepted {
		_ = resp.Body.Close()
		t.Fatalf("expected first status %d, got %d", http.StatusAccepted, resp.StatusCode)
	}
	_ = resp.Body.Close()

	resp = sendWebhookRequest(t, server.URL, "workflow_run", body, "delivery-duplicate")
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		t.Fatalf("expected duplicate status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	_ = resp.Body.Close()

	metrics := waitForMetricsSubstring(t, server.URL, `promgithub_duplicate_deliveries_seen_total{event_type="workflow_run"} 1`)
	if !strings.Contains(metrics, `promgithub_workflow_status{branch="main",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed"} 1`) {
		t.Fatalf("expected workflow metric to remain at 1, got:\n%s", metrics)
	}
	if !strings.Contains(metrics, `promgithub_duplicate_deliveries_seen_total{event_type="workflow_run"} 1`) {
		t.Fatalf("expected duplicate seen metric, got:\n%s", metrics)
	}
	if !strings.Contains(metrics, `promgithub_duplicate_deliveries_dropped_total{event_type="workflow_run"} 1`) {
		t.Fatalf("expected duplicate dropped metric, got:\n%s", metrics)
	}
}

func TestIntegrationDeliveryStoreFailurePreventsWebhookProcessing(t *testing.T) {
	runStore := newInMemoryStateStore()
	server := newIntegrationTestServerWithStateBackends(
		t,
		asyncProcessorConfig{WorkerCount: 1, QueueSize: 8},
		failingDeliveryStore{},
		runStore,
		runStore,
	)
	defer server.Close()

	body := mustReadFixture(t, "workflow_run.json")
	resp := sendWebhookRequest(t, server.URL, githubEventWorkflowRun, body, "delivery-store-failure")
	assertResponseStatus(t, resp, http.StatusInternalServerError)

	metrics := mustFetchMetrics(t, server.URL)
	if strings.Contains(metrics, `promgithub_workflow_status{branch="main",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed"} 1`) {
		t.Fatalf("workflow metrics changed after delivery store failure:\n%s", metrics)
	}
	if strings.Contains(metrics, `promgithub_event_processed_total{event_type="workflow_run"}`) {
		t.Fatalf("event was enqueued after delivery store failure:\n%s", metrics)
	}
}

func TestIntegrationWorkflowRunLifecycleBalancesStatefulMetrics(t *testing.T) {
	server := newIntegrationTestServer(t)
	defer server.Close()

	queuedBody := workflowRunFixtureWithStatus(t, statusQueued, "", integrationRunStartedAt)
	queuedResp := sendWebhookRequest(t, server.URL, githubEventWorkflowRun, queuedBody, "delivery-lifecycle-queued")
	assertResponseStatus(t, queuedResp, http.StatusAccepted)
	waitForMetricsSubstring(t, server.URL, `promgithub_workflow_queued{branch="main",repository="user/repo",workflow_name="CI"} 1`)

	inProgressBody := workflowRunFixtureWithStatus(t, statusInProgress, "", integrationRunStartedAt)
	inProgressResp := sendWebhookRequest(t, server.URL, githubEventWorkflowRun, inProgressBody, "delivery-lifecycle-in-progress")
	assertResponseStatus(t, inProgressResp, http.StatusAccepted)
	waitForMetricsSubstring(t, server.URL, `promgithub_workflow_in_progress{branch="main",repository="user/repo",workflow_name="CI"} 1`)

	completedBody := workflowRunFixtureWithStatus(t, statusCompleted, testConclusionSuccess, integrationRunCompletedAt)
	completedResp := sendWebhookRequest(t, server.URL, githubEventWorkflowRun, completedBody, "delivery-lifecycle-completed")
	assertResponseStatus(t, completedResp, http.StatusAccepted)

	metrics := waitForMetricsSubstring(t, server.URL, `promgithub_workflow_completed{branch="main",repository="user/repo",workflow_conclusion="success",workflow_name="CI"} 1`)
	expectedMetrics := []string{
		`promgithub_workflow_queued{branch="main",repository="user/repo",workflow_name="CI"} 0`,
		`promgithub_workflow_in_progress{branch="main",repository="user/repo",workflow_name="CI"} 0`,
		`promgithub_workflow_completed{branch="main",repository="user/repo",workflow_conclusion="success",workflow_name="CI"} 1`,
		`promgithub_workflow_duration_sum{branch="main",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed"} 3600`,
		`promgithub_workflow_duration_count{branch="main",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed"} 1`,
	}
	for _, expected := range expectedMetrics {
		if !strings.Contains(metrics, expected) {
			t.Fatalf("expected metrics to contain %q, got:\n%s", expected, metrics)
		}
	}
}

func TestIntegrationAsyncQueueFullReturnsUnavailableAndExposesQueueDropMetrics(t *testing.T) {
	server := newIntegrationTestServerWithAsyncConfig(t, asyncProcessorConfig{WorkerCount: 1, QueueSize: 1})
	defer server.Close()

	started := make(chan struct{})
	unblock := make(chan struct{})
	eventProcessor.processFn["workflow_run"] = func(_ context.Context, _ []byte) {
		select {
		case <-started:
		default:
			close(started)
		}
		<-unblock
	}
	t.Cleanup(func() {
		select {
		case <-unblock:
		default:
			close(unblock)
		}
	})

	body := mustReadFixture(t, "workflow_run.json")
	first := sendWebhookRequest(t, server.URL, "workflow_run", body, "delivery-queue-full-1")
	assertResponseStatus(t, first, http.StatusAccepted)

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first async event to start processing")
	}

	second := sendWebhookRequest(t, server.URL, "workflow_run", body, "delivery-queue-full-2")
	assertResponseStatus(t, second, http.StatusAccepted)

	third := sendWebhookRequest(t, server.URL, "workflow_run", body, "delivery-queue-full-3")
	assertResponseStatus(t, third, http.StatusServiceUnavailable)

	metrics := waitForMetricsSubstring(t, server.URL, `promgithub_event_queue_dropped_total{event_type="workflow_run"} 1`)
	if !strings.Contains(metrics, `promgithub_event_queue_dropped_total{event_type="workflow_run"} 1`) {
		t.Fatalf("expected queue-full drop metric, got:\n%s", metrics)
	}
}

func TestIntegrationAsyncProcessingFailureIsVisibleAndWorkerContinues(t *testing.T) {
	server := newIntegrationTestServer(t)
	defer server.Close()

	var attempts atomic.Int32
	eventProcessor.processFn["workflow_run"] = func(ctx context.Context, body []byte) {
		if attempts.Add(1) == 1 {
			panic("synthetic async processor failure")
		}

		updateWorkflowMetrics(ctx, body)
	}

	body := mustReadFixture(t, "workflow_run.json")
	first := sendWebhookRequest(t, server.URL, "workflow_run", body, "delivery-failure-1")
	assertResponseStatus(t, first, http.StatusAccepted)

	metrics := waitForMetricsSubstring(t, server.URL, `promgithub_event_processing_failures_total{event_type="workflow_run"} 1`)
	if !strings.Contains(metrics, `promgithub_event_processing_failures_total{event_type="workflow_run"} 1`) {
		t.Fatalf("expected async processing failure metric, got:\n%s", metrics)
	}

	second := sendWebhookRequest(t, server.URL, "workflow_run", body, "delivery-failure-2")
	assertResponseStatus(t, second, http.StatusAccepted)

	metrics = waitForMetricsSubstring(t, server.URL, `promgithub_workflow_status{branch="main",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed"} 1`)
	if !strings.Contains(metrics, `promgithub_event_processed_total{event_type="workflow_run"} 1`) {
		t.Fatalf("expected worker to continue and process the second event, got:\n%s", metrics)
	}
}

func TestIntegrationAsyncShutdownDrainsQueuedEvents(t *testing.T) {
	server := newIntegrationTestServerWithAsyncConfig(t, asyncProcessorConfig{WorkerCount: 1, QueueSize: 2})
	defer server.Close()

	started := make(chan struct{})
	unblock := make(chan struct{})
	eventProcessor.processFn["workflow_run"] = func(ctx context.Context, body []byte) {
		select {
		case <-started:
		default:
			close(started)
		}
		<-unblock
		updateWorkflowMetrics(ctx, body)
	}
	t.Cleanup(func() {
		select {
		case <-unblock:
		default:
			close(unblock)
		}
	})

	body := mustReadFixture(t, "workflow_run.json")
	first := sendWebhookRequest(t, server.URL, "workflow_run", body, "delivery-shutdown-1")
	assertResponseStatus(t, first, http.StatusAccepted)

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first async event to start processing")
	}

	second := sendWebhookRequest(t, server.URL, "workflow_run", body, "delivery-shutdown-2")
	assertResponseStatus(t, second, http.StatusAccepted)

	close(unblock)
	eventProcessor.Stop()
	eventProcessor = nil

	metrics := mustFetchMetrics(t, server.URL)
	if !strings.Contains(metrics, `promgithub_event_processed_total{event_type="workflow_run"} 2`) {
		t.Fatalf("expected shutdown to drain both accepted events, got:\n%s", metrics)
	}
}

func newIntegrationTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return newIntegrationTestServerWithAsyncConfig(t, asyncProcessorConfig{WorkerCount: 1, QueueSize: 8})
}

func newIntegrationTestServerWithAsyncConfig(t *testing.T, cfg asyncProcessorConfig) *httptest.Server {
	t.Helper()
	return newIntegrationTestServerWithStateStore(t, cfg, newInMemoryStateStore())
}

func newIntegrationTestServerWithStateStore(
	t *testing.T,
	cfg asyncProcessorConfig,
	store interface {
		deliveryStateBackend
		workflowRunStateBackend
		workflowJobStateBackend
	},
) *httptest.Server {
	t.Helper()
	return newIntegrationTestServerWithStateBackends(t, cfg, store, store, store)
}

func newIntegrationTestServerWithStateBackends(
	t *testing.T,
	cfg asyncProcessorConfig,
	deliveryStore deliveryStateBackend,
	workflowRunStore workflowRunStateBackend,
	workflowJobStore workflowJobStateBackend,
) *httptest.Server {
	t.Helper()
	resetIntegrationTestMetrics()

	githubWebhookSecret = []byte("integration-test-secret")
	deliveryStateStore = deliveryStore
	workflowRunStateStore = workflowRunStore
	workflowJobStateStore = workflowJobStore
	eventProcessor = newAsyncEventProcessor(cfg, zap.NewNop())
	eventProcessor.Start()
	t.Cleanup(func() {
		if eventProcessor != nil {
			eventProcessor.Stop()
		}
		eventProcessor = nil
		deliveryStateStore = nil
		workflowRunStateStore = nil
		workflowJobStateStore = nil
	})

	router := setupRouter(zap.NewNop(), defaultServiceMetrics, prometheus.DefaultGatherer)
	return httptest.NewServer(router)
}

type failingDeliveryStore struct{}

func (s failingDeliveryStore) MarkDeliveryProcessed(context.Context, string) (bool, error) {
	return false, errors.New("delivery store unavailable")
}

func assertResponseStatus(t *testing.T, resp *http.Response, expectedStatus int) {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != expectedStatus {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status %d, got %d with body %q", expectedStatus, resp.StatusCode, strings.TrimSpace(string(body)))
	}
}

func resetIntegrationTestMetrics() {
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
	asyncQueueDroppedCounter.Reset()
	asyncUnsupportedEventsCounter.Reset()
	asyncProcessingFailuresCounter.Reset()
	asyncProcessingDurationHistogram.Reset()
	duplicateDeliveriesSeenCounter.Reset()
	duplicateDeliveriesDroppedCounter.Reset()
	defaultServiceMetrics.apiCallsCounter.Reset()
	defaultServiceMetrics.requestDurationHistogram.Reset()
	asyncQueueDepthGauge.Set(0)
	asyncQueueCapacityGauge.Set(0)
	asyncWorkerCountGauge.Set(0)
}

func mustReadFixture(t *testing.T, name string) []byte {
	t.Helper()
	allowed := map[string]string{
		"workflow_run.json": "../test_data/workflow_run.json",
		"workflow_job.json": "../test_data/workflow_job.json",
		"push.json":         "../test_data/push.json",
		"pull_request.json": "../test_data/pull_request.json",
	}
	path, ok := allowed[name]
	if !ok {
		t.Fatalf("unknown fixture %q", name)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", path, err)
	}
	return body
}

func workflowRunFixtureWithStatus(t *testing.T, status, conclusion, updatedAt string) []byte {
	t.Helper()

	var payload GithubWorkflow
	if err := json.Unmarshal(mustReadFixture(t, "workflow_run.json"), &payload); err != nil {
		t.Fatalf("failed to unmarshal workflow fixture: %v", err)
	}
	payload.Workflow.Status = status
	payload.Workflow.Conclusion = conclusion
	payload.Workflow.UpdatedAt = updatedAt

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal workflow fixture: %v", err)
	}
	return body
}

func sendWebhookRequest(t *testing.T, serverURL, eventType string, body []byte, deliveryID string) *http.Response {
	t.Helper()
	signature := webhookSignature(body, githubWebhookSecret)
	req, err := http.NewRequest(http.MethodPost, serverURL+"/webhook", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("X-Hub-Signature-256", signature)
	req.Header.Set("X-GitHub-Event", eventType)
	req.Header.Set("X-GitHub-Delivery", deliveryID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	return resp
}

func webhookSignature(body, secret []byte) string {
	h := hmac.New(sha256.New, secret)
	_, _ = h.Write(body)
	return fmt.Sprintf("sha256=%s", hex.EncodeToString(h.Sum(nil)))
}

func mustFetchMetrics(t *testing.T, serverURL string) string {
	t.Helper()

	resp, err := http.Get(serverURL + "/metrics")
	if err != nil {
		t.Fatalf("failed to fetch metrics: %v", err)
	}

	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil {
		t.Fatalf("failed to read metrics response: %v", readErr)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected metrics status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	return string(body)
}

func waitForMetricsSubstring(t *testing.T, serverURL, needle string) string {
	t.Helper()

	var lastBody string
	for i := 0; i < 50; i++ {
		lastBody = mustFetchMetrics(t, serverURL)
		if strings.Contains(lastBody, needle) {
			return lastBody
		}
		time.Sleep(20 * time.Millisecond)
	}

	return lastBody
}
