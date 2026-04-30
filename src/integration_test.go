//go:build integration

package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
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

	metrics := mustFetchMetrics(t, server.URL)
	if strings.Contains(metrics, `promgithub_workflow_status{branch="main",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed"} 1`) {
		t.Fatalf("unsupported event unexpectedly updated workflow metrics:\n%s", metrics)
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

func newIntegrationTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	resetIntegrationTestMetrics()

	githubWebhookSecret = []byte("integration-test-secret")
	stateStore = newInMemoryStateStore()
	eventProcessor = newAsyncEventProcessor(asyncProcessorConfig{WorkerCount: 1, QueueSize: 8}, zap.NewNop())
	eventProcessor.Start()
	t.Cleanup(func() {
		eventProcessor.Stop()
		eventProcessor = nil
		stateStore = nil
	})

	router := setupRouter(zap.NewNop(), defaultServiceMetrics, prometheus.DefaultGatherer)
	return httptest.NewServer(router)
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
	asyncEventsDroppedCounter.Reset()
	asyncProcessingFailuresCounter.Reset()
	asyncProcessingDurationHistogram.Reset()
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
