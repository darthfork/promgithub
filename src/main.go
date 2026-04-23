package main

import (
	"encoding/json"
	"net/http"
	"net/http/pprof"
	"os"
	"strings"
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
	status int
}

var (
	Version             string
	githubWebhookSecret []byte
	logger              *zap.Logger
	enableDebug         string // Compile time flag to enable debug mode
	debug               bool

	apiCallsCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "promgithub_api_calls_total",
			Help: "Number of API calls",
		},
		[]string{"status", "method", "path"},
	)

	requestDurationHistogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "promgithub_request_duration_seconds",
			Help:    "Request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"path", "method"},
	)
)

func apiHandler(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			rec := statusRecorder{ResponseWriter: w, status: 200}

			logger.Info("Received request",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("remoteAddr", r.RemoteAddr),
				zap.String("userAgent", r.UserAgent()),
			)

			next.ServeHTTP(&rec, r)

			duration := time.Since(start).Seconds()

			apiCallsCounter.With(prometheus.Labels{
				"status": http.StatusText(rec.status),
				"method": r.Method,
				"path":   r.URL.Path,
			}).Inc()

			requestDurationHistogram.With(prometheus.Labels{
				"path":   r.URL.Path,
				"method": r.Method,
			}).Observe(duration)
		})
	}
}

func healthCheck(w http.ResponseWriter, _ *http.Request) {
	response := HealthCheckResposne{Status: "ok", Version: Version}
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
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
			// Logger sync errors on program exit are typically not critical
			// and often occur when stdout/stderr are closed before sync
			_ = err // Explicitly ignore the error
		}
	}()
}

func setupRouter(logger *zap.Logger) *mux.Router {
	r := mux.NewRouter()
	r.Use(apiHandler(logger))

	r.HandleFunc("/health", healthCheck).Methods("GET")
	r.Handle("/metrics", promhttp.Handler())
	r.HandleFunc("/webhook", githubEventsHandler).Methods("POST")

	// Profiling endpoints
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

func main() {
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
		defer func() {
			if closeErr := stateStore.Close(); closeErr != nil {
				logger.Warn("Failed to close Redis state store", zap.Error(closeErr))
			}
		}()
	}
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

	r := setupRouter(logger)

	server := &http.Server{
		Addr:           ":" + port,
		Handler:        r,
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}

	logger.Info("Starting server", zap.String("port", port))
	if err := server.ListenAndServe(); err != nil {
		logger.Fatal("Error starting server", zap.Error(err))
	}
}
