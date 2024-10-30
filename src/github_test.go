package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func computeHMAC(message, secret []byte) string {
	h := hmac.New(sha256.New, secret)
	h.Write(message)
	return "sha256=" + hex.EncodeToString(h.Sum(nil))
}

func sendTestRequest(payload []byte, eventType string) *httptest.ResponseRecorder {
	signature := computeHMAC(payload, githubWebhookSecret)
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBuffer(payload))
	req.Header.Set("X-Hub-Signature-256", signature)
	req.Header.Set("X-GitHub-Event", eventType)

	recorder := httptest.NewRecorder()
	handler := http.HandlerFunc(githubEventsHandler)
	handler.ServeHTTP(recorder, req)

	return recorder
}

func TestValidateHMAC(t *testing.T) {
	body := []byte("test body")
	signature := computeHMAC(body, githubWebhookSecret)

	valid := validateHMAC(body, signature, githubWebhookSecret)
	assert.True(t, valid)
}

func TestValidPayload(t *testing.T) {
	body := []byte(`{"workflow_run": {"id": 1, "status": "completed", "run_id": 1001, "name": "CI", "head_branch": "main", "repository": {"full_name": "user/repo"}, "conclusion": "success", "created_at": "2023-01-01T00:00:00Z", "updated_at": "2023-01-01T01:00:00Z"}}`)
	recorder := sendTestRequest(body, "workflow_run")
	assert.Equal(t, http.StatusOK, recorder.Code)
}

func TestInvalidSignature(t *testing.T) {
	body := []byte(`{"workflow_run": {"id": 1, "status": "completed", "run_id": 1001, "name": "CI", "head_branch": "main", "repository": {"full_name": "user/repo"}, "conclusion": "success", "created_at": "2023-01-01T00:00:00Z", "updated_at": "2023-01-01T01:00:00Z"}}`)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBuffer(body))
	req.Header.Set("X-Hub-Signature-256", "invalid_signature")
	req.Header.Set("X-GitHub-Event", "workflow_run")

	handler := http.HandlerFunc(githubEventsHandler)
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestMissingSignature(t *testing.T) {
	body := []byte(`{"workflow_run": {"id": 1, "status": "completed", "run_id": 1001, "name": "CI", "head_branch": "main", "repository": {"full_name": "user/repo"}, "conclusion": "success", "created_at": "2023-01-01T00:00:00Z", "updated_at": "2023-01-01T01:00:00Z"}}`)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBuffer(body))
	req.Header.Set("X-GitHub-Event", "workflow_run")

	handler := http.HandlerFunc(githubEventsHandler)
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestUnknownEvent(t *testing.T) {
	body := []byte(`{"workflow_run": {"id": 1, "status": "completed", "run_id": 1001, "name": "CI", "head_branch": "main", "repository": {"full_name": "user/repo"}, "conclusion": "success", "created_at": "2023-01-01T00:00:00Z", "updated_at": "2023-01-01T01:00:00Z"}}`)
	recorder := sendTestRequest(body, "unknown_event")
	assert.Equal(t, http.StatusOK, recorder.Code)
}
