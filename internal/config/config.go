package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPPort        int
	DatabaseURL     string
	RedisAddr       string
	RedisPassword   string
	RedisDB         int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	LogLevel        string
	DBTimeout       time.Duration
	RedisTimeout    time.Duration
	DBMaxConns      int
	DBMaxIdle       int
	CacheTTL        time.Duration
	StartupRetries  int
	StartupRetryGap time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		HTTPPort:        getEnvAsInt("HTTP_PORT", 8080),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		RedisAddr:       getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:   os.Getenv("REDIS_PASSWORD"),
		RedisDB:         getEnvAsInt("REDIS_DB", 0),
		ReadTimeout:     getEnvAsDuration("HTTP_READ_TIMEOUT", 5*time.Second),
		WriteTimeout:    getEnvAsDuration("HTTP_WRITE_TIMEOUT", 10*time.Second),
		ShutdownTimeout: getEnvAsDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		DBTimeout:       getEnvAsDuration("DB_TIMEOUT", 2*time.Second),
		RedisTimeout:    getEnvAsDuration("REDIS_TIMEOUT", 2*time.Second),
		DBMaxConns:      getEnvAsInt("DB_MAX_CONNS", 20),
		DBMaxIdle:       getEnvAsInt("DB_MAX_IDLE", 5),
		CacheTTL:        getEnvAsDuration("CACHE_TTL", 45*time.Second),
		StartupRetries:  getEnvAsInt("STARTUP_RETRIES", 10),
		StartupRetryGap: getEnvAsDuration("STARTUP_RETRY_GAP", 2*time.Second),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getEnvAsInt(key string, def int) int {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}

	parsed, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return parsed
}

func getEnvAsDuration(key string, def time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}

	parsed, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return parsed
}
