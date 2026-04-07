package config

import (
	"os"
	"strconv"
	"strings"
)

// Config holds all server configuration loaded from environment variables.
type Config struct {
	Host             string
	Port             string
	RedisAddr        string
	RedisPassword    string
	RedisDB          int
	RedisTLS         bool
	AdminGeoJSONPath string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	redisURL := envOr("REDIS_URL", "redis://localhost:6379/0")
	addr, password, db, useTLS := parseRedisURL(redisURL)

	return &Config{
		Host:             envOr("HOST", "0.0.0.0"),
		Port:             envOr("PORT", "8000"),
		RedisAddr:        addr,
		RedisPassword:    password,
		RedisDB:          db,
		RedisTLS:         useTLS,
		AdminGeoJSONPath: envOr("ADMIN_GEOJSON_PATH", "data/admin_dong.geojson"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// parseRedisURL parses a redis:// or rediss:// URL into addr, password, db, and TLS flag.
// Supports formats:
//   - redis://localhost:6379/0
//   - rediss://localhost:6379/0       (TLS)
//   - redis://:password@localhost:6379/0
//   - rediss://:password@localhost:6379/0
//   - localhost:6379 (plain)
func parseRedisURL(raw string) (addr, password string, db int, useTLS bool) {
	db = 0

	if strings.HasPrefix(raw, "rediss://") {
		useTLS = true
		raw = strings.TrimPrefix(raw, "rediss://")
	} else if strings.HasPrefix(raw, "redis://") {
		raw = strings.TrimPrefix(raw, "redis://")
	} else {
		addr = raw
		return
	}

	if idx := strings.Index(raw, "@"); idx >= 0 {
		passpart := raw[:idx]
		raw = raw[idx+1:]
		passpart = strings.TrimPrefix(passpart, ":")
		password = passpart
	}

	if idx := strings.LastIndex(raw, "/"); idx >= 0 {
		dbStr := raw[idx+1:]
		raw = raw[:idx]
		if n, err := strconv.Atoi(dbStr); err == nil {
			db = n
		}
	}

	addr = raw
	return
}
