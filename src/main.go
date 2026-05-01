package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

type HealthCheckResposne struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

type serviceMetrics struct {
	apiCallsCounter          *prometheus.CounterVec
	requestDurationHistogram *prometheus.HistogramVec
}

var (
	Version             string
	githubWebhookSecret []byte
	logger              *zap.Logger
	enableDebug         string // Compile time flag to enable debug mode
	debug               bool

	defaultServiceMetrics = newServiceMetrics(prometheus.DefaultRegisterer)
)

func newServiceMetrics(registerer prometheus.Registerer) *serviceMetrics {
	factory := promauto.With(registerer)

	return &serviceMetrics{
		apiCallsCounter: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "promgithub_api_calls_total",
				Help: "Number of API calls",
			},
			[]string{"status", "method", "path"},
		),
		requestDurationHistogram: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "promgithub_request_duration_seconds",
				Help:    "Request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"path", "method"},
		),
	}
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(body []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(body)
}

func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		if !r.wroteHeader {
			r.WriteHeader(http.StatusOK)
		}
		flusher.Flush()
	}
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

func (r *statusRecorder) Push(target string, opts *http.PushOptions) error {
	pusher, ok := r.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func apiHandler(logger *zap.Logger, metrics *serviceMetrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := statusRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(&rec, r)

			duration := time.Since(start).Seconds()
			statusText := http.StatusText(rec.status)
			if statusText == "" {
				statusText = "UNKNOWN"
			}

			metrics.apiCallsCounter.WithLabelValues(statusText, r.Method, r.URL.Path).Inc()
			metrics.requestDurationHistogram.WithLabelValues(r.URL.Path, r.Method).Observe(duration)

			fields := []zap.Field{
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", rec.status),
				zap.Float64("durationSeconds", duration),
			}

			switch {
			case rec.status >= http.StatusInternalServerError:
				logger.Error("Request completed", fields...)
			case rec.status >= http.StatusBadRequest:
				logger.Warn("Request completed", fields...)
			default:
				logger.Debug("Request completed", fields...)
			}
		})
	}
}

func healthCheck(w http.ResponseWriter, _ *http.Request) {
	response := HealthCheckResposne{Status: "ok", Version: Version}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func init() {
	var err error
	loggerConfig := zap.NewProductionConfig()

	debug = (os.Getenv("ENVIRONMENT") == "development") || (enableDebug == "true")

	if debug {
		loggerConfig = zap.NewDevelopmentConfig()
	}

	logger, err = loggerConfig.Build(zap.Fields(zap.String("appVersion", Version)))
	if err != nil {
		panic(err)
	}

	defer func() {
		if err := logger.Sync(); err != nil {
			_ = err
		}
	}()
}

func setupRouter(logger *zap.Logger, metrics *serviceMetrics, gatherer prometheus.Gatherer) *mux.Router {
	r := mux.NewRouter()
	r.Use(apiHandler(logger, metrics))

	r.HandleFunc("/health", healthCheck).Methods("GET")
	r.Handle("/metrics", promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{}))
	r.HandleFunc("/webhook", githubEventsHandler).Methods("POST")

	if debug {
		r.HandleFunc("/debug/pprof/", pprof.Index)
		r.HandleFunc("/debug/pprof/allocs", pprof.Handler("allocs").ServeHTTP)
		r.HandleFunc("/debug/pprof/block", pprof.Handler("block").ServeHTTP)
		r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		r.HandleFunc("/debug/pprof/goroutine", pprof.Handler("goroutine").ServeHTTP)
		r.HandleFunc("/debug/pprof/heap", pprof.Handler("heap").ServeHTTP)
		r.HandleFunc("/debug/pprof/mutex", pprof.Handler("mutex").ServeHTTP)
		r.HandleFunc("/debug/pprof/profile", pprof.Profile)
		r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		r.HandleFunc("/debug/pprof/threadcreate", pprof.Handler("threadcreate").ServeHTTP)
		r.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}

	return r
}

func runServer(ctx context.Context, server *http.Server, logger *zap.Logger) error {
	errCh := make(chan error, 1)
	go func() {
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		logger.Info("Shutting down server")
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}

		return <-errCh
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	port := strings.TrimSpace(os.Getenv("PROMGITHUB_SERVICE_PORT"))
	if port == "" {
		port = "8080"
	}

	ghWebhookSecretEnv := strings.TrimSpace(os.Getenv("PROMGITHUB_WEBHOOK_SECRET"))
	if ghWebhookSecretEnv == "" {
		logger.Fatal("Environment variable PROMGITHUB_WEBHOOK_SECRET is not set")
	}
	githubWebhookSecret = []byte(ghWebhookSecretEnv)

	redisConfig, redisEnabled, err := loadRedisConfigFromEnv()
	if err != nil {
		logger.Fatal("Invalid Redis configuration", zap.Error(err))
	}
	if redisEnabled {
		stateStore, err = NewRedisStateStore(redisConfig)
		if err != nil {
			logger.Fatal("Unable to initialize Redis state store", zap.Error(err))
		}
	} else {
		stateStore = newLocalStateStore(defaultRedisDeliveryTTL)
	}
	defer func() {
		if closeErr := stateStore.Close(); closeErr != nil {
			logger.Warn("Failed to close state store", zap.Error(closeErr))
		}
	}()
	logRedisMode(logger, redisEnabled, redisConfig.Addr)

	asyncConfig, err := newAsyncProcessorConfigFromEnv()
	if err != nil {
		logger.Fatal("Invalid async event processor configuration", zap.Error(err))
	}
	eventProcessor = newAsyncEventProcessor(asyncConfig, logger)
	eventProcessor.Start()
	defer eventProcessor.Stop()
	logger.Info("Async webhook processing enabled",
		zap.Int("workerCount", asyncConfig.WorkerCount),
		zap.Int("queueSize", asyncConfig.QueueSize),
	)

	r := setupRouter(logger, defaultServiceMetrics, prometheus.DefaultGatherer)
	server := &http.Server{
		Addr:           ":" + port,
		Handler:        r,
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	logger.Info("Starting server", zap.String("port", port))
	if err := runServer(ctx, server, logger); err != nil {
		logger.Fatal("Server exited with error", zap.Error(err))
	}
}
