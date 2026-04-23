package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	defaultRedisDeliveryTTL = 24 * time.Hour
	redisKeyPrefix          = "promgithub"
)

type RedisConfig struct {
	Addr        string
	Password    string
	DB          int
	KeyPrefix   string
	DeliveryTTL time.Duration
}

type RunState struct {
	Repository string `json:"repository"`
	Branch     string `json:"branch"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	StartedAt  string `json:"started_at"`
	EndedAt    string `json:"ended_at"`
}

type RedisStateStore struct {
	client      *redis.Client
	keyPrefix   string
	deliveryTTL time.Duration
}

func NewRedisStateStore(cfg RedisConfig) (*RedisStateStore, error) {
	if strings.TrimSpace(cfg.Addr) == "" {
		return nil, errors.New("redis address is required")
	}

	keyPrefix := strings.TrimSpace(cfg.KeyPrefix)
	if keyPrefix == "" {
		keyPrefix = redisKeyPrefix
	}

	deliveryTTL := cfg.DeliveryTTL
	if deliveryTTL <= 0 {
		deliveryTTL = defaultRedisDeliveryTTL
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return &RedisStateStore{
		client:      client,
		keyPrefix:   keyPrefix,
		deliveryTTL: deliveryTTL,
	}, nil
}

func (s *RedisStateStore) MarkDeliveryProcessed(ctx context.Context, deliveryID string) (bool, error) {
	if strings.TrimSpace(deliveryID) == "" {
		return false, errors.New("delivery id is required")
	}

	key := s.key("delivery", deliveryID)
	created, err := s.client.SetNX(ctx, key, "1", s.deliveryTTL).Result()
	if err != nil {
		return false, err
	}

	return created, nil
}

func (s *RedisStateStore) UpdateWorkflowRun(ctx context.Context, runID int, state RunState) error {
	if runID == 0 {
		return errors.New("workflow run id is required")
	}

	return s.writeState(ctx, s.key("workflow_run", fmt.Sprintf("%d", runID)), state)
}

func (s *RedisStateStore) UpdateWorkflowJob(ctx context.Context, jobID int, state RunState) error {
	if jobID == 0 {
		return errors.New("workflow job id is required")
	}

	return s.writeState(ctx, s.key("workflow_job", fmt.Sprintf("%d", jobID)), state)
}

func (s *RedisStateStore) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

func (s *RedisStateStore) writeState(ctx context.Context, key string, state RunState) error {
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}

	return s.client.Set(ctx, key, payload, 0).Err()
}

func (s *RedisStateStore) key(parts ...string) string {
	return strings.Join(append([]string{s.keyPrefix}, parts...), ":")
}

func loadRedisConfigFromEnv() (RedisConfig, bool, error) {
	addr := strings.TrimSpace(getEnvAny("PROMGITHUB_REDIS_ADDR", "PROMGITHUB_REDIS_ADDRESS"))
	if addr == "" {
		return RedisConfig{}, false, nil
	}

	db, err := parseEnvInt("PROMGITHUB_REDIS_DB", 0)
	if err != nil {
		return RedisConfig{}, false, err
	}

	ttl, err := parseEnvDuration("PROMGITHUB_REDIS_DELIVERY_TTL", defaultRedisDeliveryTTL)
	if err != nil {
		return RedisConfig{}, false, err
	}

	cfg := RedisConfig{
		Addr:        addr,
		Password:    strings.TrimSpace(os.Getenv("PROMGITHUB_REDIS_PASSWORD")),
		DB:          db,
		KeyPrefix:   strings.TrimSpace(os.Getenv("PROMGITHUB_REDIS_KEY_PREFIX")),
		DeliveryTTL: ttl,
	}

	return cfg, true, nil
}

func logRedisMode(logger *zap.Logger, enabled bool, addr string) {
	if enabled {
		logger.Info("Redis-backed multi-instance mode enabled", zap.String("redisAddr", addr))
		return
	}

	logger.Info("Redis-backed multi-instance mode disabled, running without shared state")
}
