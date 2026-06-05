package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

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
	queue      chan webhookEvent
	workers    int
	dispatcher webhookEventDispatcher
	logger     *zap.Logger
	wg         sync.WaitGroup
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
		queue:      make(chan webhookEvent, cfg.QueueSize),
		workers:    cfg.WorkerCount,
		dispatcher: newDefaultGitHubEventDispatcher(),
		logger:     logger,
	}

	defaultMetricRecorder.SetAsyncWorkerCount(cfg.WorkerCount)
	defaultMetricRecorder.SetAsyncQueueCapacity(cfg.QueueSize)
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
		ctx:       context.WithoutCancel(ctx),
		eventType: eventType,
		body:      append([]byte(nil), body...),
	}

	select {
	case p.queue <- event:
		defaultMetricRecorder.SetAsyncQueueDepth(len(p.queue))
		return nil
	default:
		defaultMetricRecorder.RecordAsyncQueueDropped(eventType)
		defaultMetricRecorder.SetAsyncQueueDepth(len(p.queue))
		return fmt.Errorf("event queue is full")
	}
}

func (p *asyncEventProcessor) runWorker(workerID int) {
	defer p.wg.Done()

	for event := range p.queue {
		defaultMetricRecorder.SetAsyncQueueDepth(len(p.queue))
		start := time.Now()

		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					defaultMetricRecorder.RecordAsyncProcessingFailure(event.eventType)
					p.logger.Error("Recovered from async event processor panic",
						zap.Int("workerID", workerID),
						zap.String("eventType", event.eventType),
						zap.Any("panic", recovered),
					)
				}
			}()

			if p.dispatcher == nil || !p.dispatcher.Dispatch(event.ctx, event.eventType, event.body) {
				defaultMetricRecorder.RecordAsyncUnsupportedEvent(event.eventType)
				return
			}

			defaultMetricRecorder.RecordAsyncProcessedEvent(event.eventType)
			defaultMetricRecorder.ObserveAsyncProcessingDuration(event.eventType, time.Since(start).Seconds())
		}()
	}
}
