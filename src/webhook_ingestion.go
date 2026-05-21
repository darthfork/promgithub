package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

type webhookIngestionRequest struct {
	Body       []byte
	Signature  string
	EventType  string
	DeliveryID string
}

type webhookIngestionResult struct {
	StatusCode   int
	ErrorMessage string
}

type webhookAcceptor interface {
	Accept(context.Context, webhookIngestionRequest) webhookIngestionResult
}

type webhookDeliveryStore interface {
	MarkDeliveryProcessed(ctx context.Context, deliveryID string) (bool, error)
}

type webhookEventQueue interface {
	Enqueue(ctx context.Context, eventType string, body []byte) error
}

type webhookEventDispatcher interface {
	Dispatch(ctx context.Context, eventType string, body []byte) bool
}

type webhookIngestionMetrics interface {
	RecordDuplicateDelivery(eventType string)
}

const (
	githubEventWorkflowRun = "workflow_run"
	githubEventWorkflowJob = "workflow_job"
	githubEventPush        = "push"
	githubEventPullRequest = "pull_request"
)

type webhookIngestion struct {
	secret        []byte
	logger        *zap.Logger
	deliveryStore webhookDeliveryStore
	localDeduper  *deliveryDeduper
	queue         webhookEventQueue
	dispatcher    webhookEventDispatcher
	metrics       webhookIngestionMetrics
	now           func() time.Time
}

func newDefaultWebhookIngestion() *webhookIngestion {
	ingestion := &webhookIngestion{
		secret:        githubWebhookSecret,
		logger:        logger,
		deliveryStore: stateStore,
		localDeduper:  deliveryDeduperCache,
		dispatcher:    defaultWebhookEventDispatcher{},
		metrics:       defaultMetricRecorder,
		now:           time.Now,
	}
	if eventProcessor != nil {
		ingestion.queue = eventProcessor
	}

	return ingestion
}

func (i *webhookIngestion) Accept(ctx context.Context, request webhookIngestionRequest) webhookIngestionResult {
	if !validateHMAC(request.Body, request.Signature, i.secret) {
		i.logError("Invalid signature")
		return webhookIngestionResult{
			StatusCode:   http.StatusUnauthorized,
			ErrorMessage: "Invalid signature",
		}
	}

	eventType := request.EventType
	deliveryID := strings.TrimSpace(request.DeliveryID)
	duplicate, err := i.markDuplicateDelivery(ctx, eventType, deliveryID)
	if err != nil {
		i.logError("Unable to record webhook delivery", zap.String("deliveryID", deliveryID), zap.Error(err))
		return webhookIngestionResult{
			StatusCode:   http.StatusInternalServerError,
			ErrorMessage: "Unable to record webhook delivery",
		}
	}
	if duplicate {
		return webhookIngestionResult{StatusCode: http.StatusOK}
	}

	if i.queue != nil {
		if err := i.queue.Enqueue(ctx, eventType, request.Body); err != nil {
			i.logWarn("Dropping webhook event because queue is full", zap.String("eventType", eventType), zap.Error(err))
			return webhookIngestionResult{
				StatusCode:   http.StatusServiceUnavailable,
				ErrorMessage: "Webhook queue is full",
			}
		}
		return webhookIngestionResult{StatusCode: http.StatusAccepted}
	}

	if i.dispatcher != nil {
		if ok := i.dispatcher.Dispatch(ctx, eventType, request.Body); !ok {
			i.logWarn("Invalid GitHub event type", zap.String("eventType", eventType))
		}
	}

	return webhookIngestionResult{StatusCode: http.StatusOK}
}

func (i *webhookIngestion) markDuplicateDelivery(ctx context.Context, eventType, deliveryID string) (bool, error) {
	if deliveryID == "" {
		return false, nil
	}

	switch {
	case i.deliveryStore != nil:
		processed, err := i.deliveryStore.MarkDeliveryProcessed(ctx, deliveryID)
		if err != nil {
			return false, err
		}
		if processed {
			return false, nil
		}
	case i.localDeduper != nil:
		now := time.Now
		if i.now != nil {
			now = i.now
		}
		if !i.localDeduper.SeenBefore(deliveryID, now()) {
			return false, nil
		}
	default:
		return false, nil
	}

	if i.metrics != nil {
		i.metrics.RecordDuplicateDelivery(eventType)
	}
	i.logInfo("Skipping duplicate GitHub delivery", zap.String("deliveryID", deliveryID), zap.String("eventType", eventType))
	return true, nil
}

func (i *webhookIngestion) logInfo(message string, fields ...zap.Field) {
	if i.logger != nil {
		i.logger.Info(message, fields...)
	}
}

func (i *webhookIngestion) logWarn(message string, fields ...zap.Field) {
	if i.logger != nil {
		i.logger.Warn(message, fields...)
	}
}

func (i *webhookIngestion) logError(message string, fields ...zap.Field) {
	if i.logger != nil {
		i.logger.Error(message, fields...)
	}
}

type defaultWebhookEventDispatcher struct{}

func (defaultWebhookEventDispatcher) Dispatch(ctx context.Context, eventType string, body []byte) bool {
	switch eventType {
	case githubEventWorkflowRun:
		updateWorkflowMetrics(ctx, body)
	case githubEventWorkflowJob:
		updateJobMetrics(ctx, body)
	case githubEventPush:
		updateCommitMetrics(body)
	case githubEventPullRequest:
		updatePullRequestMetrics(body)
	default:
		return false
	}

	return true
}

func webhookHTTPHandler(acceptor webhookAcceptor, logger *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Unable to read request body", http.StatusInternalServerError)
			if logger != nil {
				logger.Error("Unable to read request body", zap.Error(err))
			}
			return
		}

		result := acceptor.Accept(r.Context(), webhookIngestionRequest{
			Body:       body,
			Signature:  r.Header.Get("X-Hub-Signature-256"),
			EventType:  r.Header.Get("X-GitHub-Event"),
			DeliveryID: r.Header.Get("X-GitHub-Delivery"),
		})

		if result.ErrorMessage != "" {
			http.Error(w, result.ErrorMessage, result.StatusCode)
			return
		}

		w.WriteHeader(result.StatusCode)
	}
}
