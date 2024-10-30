package main

import (
	"encoding/json"
	"net/http"
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
	debug               bool
	apiCallsCounter     = promauto.NewCounterVec(
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

func APIHandler(logger *zap.Logger) func(http.Handler) http.Handler {
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

func init() {
	var err error
	loggerConfig := zap.NewProductionConfig()

	if os.Getenv("ENVIRONMENT") == "development" {
		loggerConfig = zap.NewDevelopmentConfig()
	}

	logger, err = loggerConfig.Build(zap.Fields(zap.String("appVersion", Version)))
	if err != nil {
		panic(err)
	}
	defer logger.Sync()
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

	r := mux.NewRouter()
	r.Use(APIHandler(logger))

	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		response := HealthCheckResposne{Status: "ok", Version: Version}
		json.NewEncoder(w).Encode(response)
	}).Methods("GET")

	r.Handle("/metrics", promhttp.Handler())

	r.HandleFunc("/webhook", githubEventsHandler).Methods("POST")

	logger.Info("Starting server", zap.String("port", port))
	if err := http.ListenAndServe(":"+port, r); err != nil {
		logger.Fatal("Error starting server", zap.Error(err))
	}
}
