package main

import (
	"encoding/json"
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

var (
	reg *prometheus.Registry
)

func init() {
	// Disable logging
	logger = zap.NewNop()
	reg = prometheus.NewRegistry()
}

func TestHealthCheck(t *testing.T) {
	// Set the Version variable for the test
	Version = "1.0.0"

	// Create a test HTTP request
	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}

	// Create a test HTTP response recorder
	rr := httptest.NewRecorder()

	// Call the healthCheck handler
	handler := http.HandlerFunc(healthCheck)
	handler.ServeHTTP(rr, req)

	// Verify the response status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, status)
	}

	// Verify the response body
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
	// Set environment variables for the test
	os.Setenv("PROMGITHUB_WEBHOOK_SECRET", "testsecret")
	defer os.Unsetenv("PROMGITHUB_WEBHOOK_SECRET")

	// Initialize the logger
	logger := zap.NewNop()

	// Set up the router
	r := setupRouter(logger)

	// Create a test HTTP server
	server := httptest.NewServer(r)
	defer server.Close()

	// Test the /health endpoint
	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("Failed to send HTTP request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Test the /metrics endpoint
	resp, err = http.Get(server.URL + "/metrics")
	if err != nil {
		t.Fatalf("Failed to send HTTP request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestAPIHandler(t *testing.T) {
	apiCallsCounter.Reset()
	reg.MustRegister(apiCallsCounter)

	// Create a test HTTP handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	// Wrap the test handler with the APIHandler middleware
	handler := APIHandler(logger)(testHandler)

	// Create a test HTTP server
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create a test HTTP client
	client := &http.Client{Timeout: 10 * time.Second}

	// Send a test HTTP request
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to send HTTP request: %v", err)
	}
	defer resp.Body.Close()

	// Verify the response status code
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Verify the Prometheus metrics
	if err := testutil.CollectAndCompare(apiCallsCounter, strings.NewReader(`
		# HELP promgithub_api_calls_total Number of API calls
		# TYPE promgithub_api_calls_total counter
		promgithub_api_calls_total{method="GET",path="/",status="OK"} 1
	`)); err != nil {
		t.Errorf("unexpected metrics: %v", err)
	}
}
