//go:build !integration

package main

import (
	"bytes"
	"context"
	"testing"
)

func TestGitHubEventDispatcherRoutesSupportedEventsAndRejectsUnknown(t *testing.T) {
	body := []byte(`{"ok":true}`)
	var routedEvent string
	var routedBody []byte

	dispatcher := githubEventDispatcher{
		handlers: map[string]eventHandler{
			githubEventWorkflowRun: func(_ context.Context, eventBody []byte) {
				routedEvent = githubEventWorkflowRun
				routedBody = eventBody
			},
		},
	}

	if ok := dispatcher.Dispatch(context.Background(), githubEventWorkflowRun, body); !ok {
		t.Fatal("expected workflow_run to be dispatched")
	}
	if routedEvent != githubEventWorkflowRun || !bytes.Equal(routedBody, body) {
		t.Fatalf("unexpected routed event %q with body %q", routedEvent, string(routedBody))
	}
	if ok := dispatcher.Dispatch(context.Background(), "unknown_event", body); ok {
		t.Fatal("expected unknown event to be rejected")
	}
}

func TestDefaultGitHubEventDispatcherOwnsSupportedEventPolicy(t *testing.T) {
	dispatcher := newDefaultGitHubEventDispatcher()

	for _, eventType := range []string{
		githubEventWorkflowRun,
		githubEventWorkflowJob,
		githubEventPush,
		githubEventPullRequest,
	} {
		if !dispatcher.Supports(eventType) {
			t.Fatalf("expected %s to be supported", eventType)
		}
	}

	if dispatcher.Supports("unknown_event") {
		t.Fatal("expected unknown_event to be unsupported")
	}
}
