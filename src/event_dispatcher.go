package main

import "context"

type githubEventDispatcher struct {
	handlers map[string]eventHandler
}

func newDefaultGitHubEventDispatcher() githubEventDispatcher {
	return githubEventDispatcher{
		handlers: map[string]eventHandler{
			githubEventWorkflowRun: updateWorkflowMetrics,
			githubEventWorkflowJob: updateJobMetrics,
			githubEventPush:        func(_ context.Context, body []byte) { updateCommitMetrics(body) },
			githubEventPullRequest: func(_ context.Context, body []byte) { updatePullRequestMetrics(body) },
		},
	}
}

func (d githubEventDispatcher) Supports(eventType string) bool {
	_, ok := d.handlers[eventType]
	return ok
}

func (d githubEventDispatcher) Dispatch(ctx context.Context, eventType string, body []byte) bool {
	handler, ok := d.handlers[eventType]
	if !ok {
		return false
	}

	handler(ctx, body)
	return true
}
