package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.uber.org/zap"
)

var reg *prometheus.Registry

func init() {
	logger = zap.NewNop()
	reg = prometheus.NewRegistry()
}

func TestHealthCheck(t *testing.T) {
	Version = "1.0.0"

	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(healthCheck)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, status)
	}

	expectedResponse := HealthCheckResposne{Status: "ok", Version: Version}
	var actualResponse HealthCheckResposne
	if err := json.NewDecoder(rr.Body).Decode(&actualResponse); err != nil {
		t.Fatalf("Failed to decode response body: %v", err)
	}

	if actualResponse != expectedResponse {
		t.Errorf("Expected response body %+v, got %+v", expectedResponse, actualResponse)
	}
}

func TestSetupRouter(t *testing.T) {
	_ = os.Setenv("PROMGITHUB_WEBHOOK_SECRET", "testsecret")
	defer func() { _ = os.Unsetenv("PROMGITHUB_WEBHOOK_SECRET") }()

	registry := prometheus.NewRegistry()
	metrics := newServiceMetrics(registry)
	r := setupRouter(zap.NewNop(), metrics, registry)

	server := httptest.NewServer(r)
	defer server.Close()

	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("Failed to send HTTP request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	resp, err = http.Get(server.URL + "/metrics")
	if err != nil {
		t.Fatalf("Failed to send HTTP request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestApiHandlerRecordsExplicitStatusCode(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := newServiceMetrics(registry)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	})

	handler := apiHandler(zap.NewNop(), metrics)(testHandler)
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to send HTTP request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status code %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	if err := testutil.CollectAndCompare(metrics.apiCallsCounter, strings.NewReader(`
		# HELP promgithub_api_calls_total Number of API calls
		# TYPE promgithub_api_calls_total counter
		promgithub_api_calls_total{method="GET",path="/",status="Created"} 1
	`)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestApiHandlerRecordsImplicitStatusCode(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := newServiceMetrics(registry)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	handler := apiHandler(zap.NewNop(), metrics)(testHandler)
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to send HTTP request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	if err := testutil.CollectAndCompare(metrics.apiCallsCounter, strings.NewReader(`
		# HELP promgithub_api_calls_total Number of API calls
		# TYPE promgithub_api_calls_total counter
		promgithub_api_calls_total{method="GET",path="/",status="OK"} 1
	`)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}

func TestRunServerShutsDownOnContextCancel(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := newServiceMetrics(registry)
	router := setupRouter(zap.NewNop(), metrics, registry)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer func() { _ = listener.Close() }()

	server := &http.Server{Handler: router}
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- runServerWithListener(ctx, server, zap.NewNop(), listener)
	}()

	resp, err := http.Get("http://" + listener.Addr().String() + "/health")
	if err != nil {
		t.Fatalf("Failed to send HTTP request: %v", err)
	}
	_ = resp.Body.Close()

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Expected graceful shutdown, got error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Timed out waiting for server shutdown")
	}
}

func runServerWithListener(ctx context.Context, server *http.Server, logger *zap.Logger, listener net.Listener) error {
	errCh := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return <-errCh
	}
}
