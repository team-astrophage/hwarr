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
	AdminGeoJSONPath string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	redisURL := envOr("REDIS_URL", "redis://localhost:6379/0")
	addr, password, db := parseRedisURL(redisURL)

	return &Config{
		Host:             envOr("HOST", "0.0.0.0"),
		Port:             envOr("PORT", "8000"),
		RedisAddr:        addr,
		RedisPassword:    password,
		RedisDB:          db,
		AdminGeoJSONPath: envOr("ADMIN_GEOJSON_PATH", "data/admin_dong.geojson"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// parseRedisURL parses a redis:// URL into addr, password, and db.
// Supports formats:
//   - redis://localhost:6379/0
//   - redis://:password@localhost:6379/0
//   - localhost:6379 (plain)
func parseRedisURL(raw string) (addr, password string, db int) {
	db = 0

	if !strings.HasPrefix(raw, "redis://") {
		addr = raw
		return
	}

	raw = strings.TrimPrefix(raw, "redis://")

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
