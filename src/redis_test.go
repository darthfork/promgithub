//go:build !integration

package main

import (
	"os"
	"testing"
	"time"
)

func TestLoadRedisConfigFromEnv(t *testing.T) {
	for _, key := range []string{
		"PROMGITHUB_REDIS_ADDR",
		"PROMGITHUB_REDIS_ADDRESS",
		"PROMGITHUB_REDIS_DB",
		"PROMGITHUB_REDIS_PASSWORD",
		"PROMGITHUB_REDIS_KEY_PREFIX",
		"PROMGITHUB_REDIS_DELIVERY_TTL",
	} {
		_ = os.Unsetenv(key)
	}

	_, enabled, err := loadRedisConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if enabled {
		t.Fatalf("expected redis to be disabled when address is not configured")
	}

	_ = os.Setenv("PROMGITHUB_REDIS_ADDR", "localhost:6379")
	_ = os.Setenv("PROMGITHUB_REDIS_DB", "2")
	_ = os.Setenv("PROMGITHUB_REDIS_PASSWORD", "secret")
	_ = os.Setenv("PROMGITHUB_REDIS_KEY_PREFIX", "custom")
	_ = os.Setenv("PROMGITHUB_REDIS_DELIVERY_TTL", "2h")
	defer func() {
		_ = os.Unsetenv("PROMGITHUB_REDIS_ADDR")
		_ = os.Unsetenv("PROMGITHUB_REDIS_DB")
		_ = os.Unsetenv("PROMGITHUB_REDIS_PASSWORD")
		_ = os.Unsetenv("PROMGITHUB_REDIS_KEY_PREFIX")
		_ = os.Unsetenv("PROMGITHUB_REDIS_DELIVERY_TTL")
	}()

	cfg, enabled, err := loadRedisConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !enabled {
		t.Fatalf("expected redis to be enabled")
	}
	if cfg.Addr != "localhost:6379" || cfg.DB != 2 || cfg.Password != "secret" || cfg.KeyPrefix != "custom" || cfg.DeliveryTTL != 2*time.Hour {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}
