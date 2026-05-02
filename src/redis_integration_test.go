//go:build integration && redis

package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

func TestRedisIntegrationDuplicateDeliverySharedAcrossServers(t *testing.T) {
	store := newRedisIntegrationStore(t)
	serverA := newRedisIntegrationTestServer(t, store)
	defer serverA.Close()
	serverB := httptestServerSharingGlobals(t)
	defer serverB.Close()

	body := mustReadFixture(t, "workflow_run.json")

	resp := sendWebhookRequest(t, serverA.URL, "workflow_run", body, "redis-shared-delivery")
	if resp.StatusCode != http.StatusAccepted {
		_ = resp.Body.Close()
		t.Fatalf("expected first status %d, got %d", http.StatusAccepted, resp.StatusCode)
	}
	_ = resp.Body.Close()

	resp = sendWebhookRequest(t, serverB.URL, "workflow_run", body, "redis-shared-delivery")
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		t.Fatalf("expected duplicate status %d from second server, got %d", http.StatusOK, resp.StatusCode)
	}
	_ = resp.Body.Close()

	metrics := waitForMetricsSubstring(t, serverA.URL, `promgithub_duplicate_deliveries_seen_total{event_type="workflow_run"} 1`)
	if !strings.Contains(metrics, `promgithub_workflow_status{branch="main",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed"} 1`) {
		t.Fatalf("expected workflow metric to be recorded once, got:\n%s", metrics)
	}
	if !strings.Contains(metrics, `promgithub_duplicate_deliveries_dropped_total{event_type="workflow_run"} 1`) {
		t.Fatalf("expected duplicate delivery to be dropped by shared Redis state, got:\n%s", metrics)
	}
}

func TestRedisIntegrationWorkflowAndJobStatePersistAcrossLookups(t *testing.T) {
	store := newRedisIntegrationStore(t)
	server := newRedisIntegrationTestServer(t, store)
	defer server.Close()

	workflowBody := mustReadFixture(t, "workflow_run.json")
	resp := sendWebhookRequest(t, server.URL, "workflow_run", workflowBody, "redis-workflow-state-1")
	if resp.StatusCode != http.StatusAccepted {
		_ = resp.Body.Close()
		t.Fatalf("expected workflow status %d, got %d", http.StatusAccepted, resp.StatusCode)
	}
	_ = resp.Body.Close()
	waitForMetricsSubstring(t, server.URL, `promgithub_workflow_status{branch="main",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed"} 1`)

	workflowState := waitForRedisWorkflowRun(t, store, 1001)
	if workflowState.Repository != "user/repo" || workflowState.Branch != "main" || workflowState.Name != "CI" || workflowState.Status != "completed" || workflowState.Conclusion != "success" {
		t.Fatalf("unexpected workflow state persisted in redis: %+v", workflowState)
	}

	jobBody := mustReadFixture(t, "workflow_job.json")
	resp = sendWebhookRequest(t, server.URL, "workflow_job", jobBody, "redis-job-state-1")
	if resp.StatusCode != http.StatusAccepted {
		_ = resp.Body.Close()
		t.Fatalf("expected job status %d, got %d", http.StatusAccepted, resp.StatusCode)
	}
	_ = resp.Body.Close()
	waitForMetricsSubstring(t, server.URL, `promgithub_job_status{branch="main",job_conclusion="success",job_status="completed",repository="user/repo",workflow_name="CI"} 1`)

	jobState := waitForRedisWorkflowJob(t, store, 1)
	if jobState.Repository != "user/repo" || jobState.Branch != "main" || jobState.Name != "CI" || jobState.Status != "completed" || jobState.Conclusion != "success" {
		t.Fatalf("unexpected job state persisted in redis: %+v", jobState)
	}
}

func TestRedisIntegrationDuplicateRunTransitionDoesNotDoubleCount(t *testing.T) {
	store := newRedisIntegrationStore(t)
	serverA := newRedisIntegrationTestServer(t, store)
	defer serverA.Close()
	serverB := httptestServerSharingGlobals(t)
	defer serverB.Close()

	body := mustReadFixture(t, "workflow_run.json")
	for _, request := range []struct {
		serverURL  string
		deliveryID string
	}{
		{serverURL: serverA.URL, deliveryID: "redis-run-transition-1"},
		{serverURL: serverB.URL, deliveryID: "redis-run-transition-2"},
	} {
		resp := sendWebhookRequest(t, request.serverURL, "workflow_run", body, request.deliveryID)
		if resp.StatusCode != http.StatusAccepted {
			_ = resp.Body.Close()
			t.Fatalf("expected status %d for %s, got %d", http.StatusAccepted, request.deliveryID, resp.StatusCode)
		}
		_ = resp.Body.Close()
	}

	metrics := waitForMetricsSubstring(t, serverA.URL, `promgithub_workflow_status{branch="main",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed"} 1`)
	if strings.Contains(metrics, `promgithub_workflow_status{branch="main",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed"} 2`) {
		t.Fatalf("expected Redis-backed run state to suppress duplicate transition, got:\n%s", metrics)
	}
}

func TestRedisIntegrationConnectionFailureIsClear(t *testing.T) {
	_, err := NewRedisStateStore(RedisConfig{
		Addr:        "127.0.0.1:0",
		KeyPrefix:   "promgithub-test-failure",
		DeliveryTTL: time.Minute,
	})
	if err == nil {
		t.Fatal("expected redis connection failure")
	}
	if !strings.Contains(err.Error(), "ping redis") {
		t.Fatalf("expected redis ping context in error, got %q", err.Error())
	}
}

func newRedisIntegrationStore(t *testing.T) *RedisStateStore {
	t.Helper()

	addr := strings.TrimSpace(os.Getenv("PROMGITHUB_REDIS_ADDR"))
	if addr == "" {
		addr = "127.0.0.1:6379"
	}

	keyPrefix := fmt.Sprintf("promgithub:test:%s:%d", strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()), time.Now().UnixNano())
	store, err := NewRedisStateStore(RedisConfig{
		Addr:        addr,
		KeyPrefix:   keyPrefix,
		DeliveryTTL: time.Minute,
	})
	if err != nil {
		t.Fatalf("real Redis integration tests require a reachable Redis at %s: %v", addr, err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cleanupRedisKeys(ctx, store, keyPrefix)
		_ = store.Close()
	})

	return store
}

func newRedisIntegrationTestServer(t *testing.T, store StateStore) *httptest.Server {
	t.Helper()
	resetIntegrationTestMetrics()

	githubWebhookSecret = []byte("integration-test-secret")
	stateStore = store
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

func httptestServerSharingGlobals(t *testing.T) *httptest.Server {
	t.Helper()
	router := setupRouter(zap.NewNop(), defaultServiceMetrics, prometheus.DefaultGatherer)
	return httptest.NewServer(router)
}

func waitForRedisWorkflowRun(t *testing.T, store *RedisStateStore, id int) RunState {
	t.Helper()
	return waitForRedisState(t, func(ctx context.Context) (RunState, bool, error) {
		return store.GetWorkflowRun(ctx, id)
	})
}

func waitForRedisWorkflowJob(t *testing.T, store *RedisStateStore, id int) RunState {
	t.Helper()
	return waitForRedisState(t, func(ctx context.Context) (RunState, bool, error) {
		return store.GetWorkflowJob(ctx, id)
	})
}

func waitForRedisState(t *testing.T, get func(context.Context) (RunState, bool, error)) RunState {
	t.Helper()
	ctx := context.Background()
	var last RunState
	for i := 0; i < 50; i++ {
		state, found, err := get(ctx)
		if err != nil {
			t.Fatalf("failed to read redis state: %v", err)
		}
		if found {
			return state
		}
		last = state
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for redis state, last=%+v", last)
	return RunState{}
}

func cleanupRedisKeys(ctx context.Context, store *RedisStateStore, keyPrefix string) {
	iter := store.client.Scan(ctx, 0, keyPrefix+":*", 0).Iterator()
	for iter.Next(ctx) {
		_ = store.client.Del(ctx, iter.Val()).Err()
	}
}
