package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const (
	defaultWorkerCount = 4
	defaultQueueSize   = 256
)

type eventHandler func(context.Context, []byte)

type webhookEvent struct {
	ctx       context.Context
	eventType string
	body      []byte
}

type asyncProcessorConfig struct {
	WorkerCount int
	QueueSize   int
}

type asyncEventProcessor struct {
	queue     chan webhookEvent
	workers   int
	processFn map[string]eventHandler
	logger    *zap.Logger
	wg        sync.WaitGroup
}

func newAsyncProcessorConfigFromEnv() (asyncProcessorConfig, error) {
	workers, err := parseEnvInt("PROMGITHUB_EVENT_WORKERS", defaultWorkerCount)
	if err != nil {
		return asyncProcessorConfig{}, err
	}
	if workers <= 0 {
		return asyncProcessorConfig{}, errors.New("PROMGITHUB_EVENT_WORKERS must be greater than 0")
	}

	queueSize, err := parseEnvInt("PROMGITHUB_EVENT_QUEUE_SIZE", defaultQueueSize)
	if err != nil {
		return asyncProcessorConfig{}, err
	}
	if queueSize <= 0 {
		return asyncProcessorConfig{}, errors.New("PROMGITHUB_EVENT_QUEUE_SIZE must be greater than 0")
	}

	return asyncProcessorConfig{WorkerCount: workers, QueueSize: queueSize}, nil
}

func newAsyncEventProcessor(cfg asyncProcessorConfig, logger *zap.Logger) *asyncEventProcessor {
	processor := &asyncEventProcessor{
		queue:   make(chan webhookEvent, cfg.QueueSize),
		workers: cfg.WorkerCount,
		processFn: map[string]eventHandler{
			"workflow_run": updateWorkflowMetrics,
			"workflow_job": updateJobMetrics,
			"push":         func(_ context.Context, body []byte) { updateCommitMetrics(body) },
			"pull_request": func(_ context.Context, body []byte) { updatePullRequestMetrics(body) },
		},
		logger: logger,
	}

	asyncWorkerCountGauge.Set(float64(cfg.WorkerCount))
	asyncQueueCapacityGauge.Set(float64(cfg.QueueSize))
	return processor
}

func (p *asyncEventProcessor) Start() {
	for workerID := 0; workerID < p.workers; workerID++ {
		p.wg.Add(1)
		go p.runWorker(workerID)
	}
}

func (p *asyncEventProcessor) Stop() {
	if p == nil {
		return
	}
	close(p.queue)
	p.wg.Wait()
}

func (p *asyncEventProcessor) Enqueue(ctx context.Context, eventType string, body []byte) error {
	event := webhookEvent{
		ctx:       ctx,
		eventType: eventType,
		body:      append([]byte(nil), body...),
	}

	select {
	case p.queue <- event:
		asyncQueueDepthGauge.Set(float64(len(p.queue)))
		return nil
	default:
		asyncEventsDroppedCounter.WithLabelValues(eventType, "queue_full").Inc()
		asyncQueueDepthGauge.Set(float64(len(p.queue)))
		return fmt.Errorf("event queue is full")
	}
}

func (p *asyncEventProcessor) runWorker(workerID int) {
	defer p.wg.Done()

	for event := range p.queue {
		asyncQueueDepthGauge.Set(float64(len(p.queue)))
		start := time.Now()

		processor, ok := p.processFn[event.eventType]
		if !ok {
			asyncEventsDroppedCounter.WithLabelValues(event.eventType, "unsupported_event").Inc()
			continue
		}

		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					asyncProcessingFailuresCounter.WithLabelValues(event.eventType).Inc()
					p.logger.Error("Recovered from async event processor panic",
						zap.Int("workerID", workerID),
						zap.String("eventType", event.eventType),
						zap.Any("panic", recovered),
					)
				}
			}()

			processor(event.ctx, event.body)
			asyncProcessedEventsCounter.WithLabelValues(event.eventType).Inc()
			asyncProcessingDurationHistogram.With(prometheus.Labels{"event_type": event.eventType}).Observe(time.Since(start).Seconds())
		}()
	}
}
