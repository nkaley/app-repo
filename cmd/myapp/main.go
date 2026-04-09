package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"myapp/internal/app"
	"myapp/internal/config"
	httpapi "myapp/internal/http"
	"myapp/internal/migrations"
	"myapp/internal/metrics"
	"myapp/internal/storage"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

var buildVersion = "dev"

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config error", "error", err)
		os.Exit(1)
	}

	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	logStartupConfig(cfg)

	ctx := context.Background()

	pgPool, err := newPostgresPool(ctx, cfg)
	if err != nil {
		slog.Error("postgres init failed", "error", err)
		os.Exit(1)
	}

	if err = migrations.Up(ctx, pgPool); err != nil {
		slog.Error("migration error", "error", err)
		os.Exit(1)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:         cfg.RedisAddr,
		Password:     cfg.RedisPassword,
		DB:           cfg.RedisDB,
		DialTimeout:  cfg.RedisTimeout,
		ReadTimeout:  cfg.RedisTimeout,
		WriteTimeout: cfg.RedisTimeout,
	})
	if err = retry(ctx, cfg.StartupRetries, cfg.StartupRetryGap, func() error {
		redisCtx, cancel := context.WithTimeout(ctx, cfg.RedisTimeout)
		defer cancel()
		return redisClient.Ping(redisCtx).Err()
	}); err != nil {
		slog.Error("redis init failed", "error", err)
		os.Exit(1)
	}

	m := metrics.New()

	application := app.New(
		storage.NewPostgresStore(pgPool),
		storage.NewRedisStore(redisClient),
		cfg.DBTimeout,
		cfg.RedisTimeout,
		cfg.CacheTTL,
		m,
	)

	handler := httpapi.NewHandler(application, m, buildVersion)
	router := m.Instrument(httpapi.NewRouter(handler))

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:           router,
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("server starting", "addr", server.Addr, "version", buildVersion)
		if serveErr := server.ListenAndServe(); serveErr != nil && !httpapi.IsServerClosed(serveErr) {
			slog.Error("server error", "error", serveErr)
		}
	}()

	waitForSignal()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err = server.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		_ = server.Close()
	}
	pgPool.Close()
	if err = redisClient.Close(); err != nil {
		slog.Error("redis close failed", "error", err)
	}

	slog.Info("server stopped")
}

func waitForSignal() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}

func newLogger(level string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	switch strings.ToLower(level) {
	case "debug":
		opts.Level = slog.LevelDebug
	case "warn":
		opts.Level = slog.LevelWarn
	case "error":
		opts.Level = slog.LevelError
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}

func logStartupConfig(cfg config.Config) {
	slog.Info("startup config",
		"http_port", cfg.HTTPPort,
		"redis_addr", cfg.RedisAddr,
		"redis_db", cfg.RedisDB,
		"log_level", cfg.LogLevel,
		"db_timeout", cfg.DBTimeout.String(),
		"redis_timeout", cfg.RedisTimeout.String(),
		"db_max_conns", cfg.DBMaxConns,
		"db_max_idle", cfg.DBMaxIdle,
		"shutdown_timeout", cfg.ShutdownTimeout.String(),
		"cache_ttl", cfg.CacheTTL.String(),
		"startup_retries", cfg.StartupRetries,
		"startup_retry_gap", cfg.StartupRetryGap.String(),
	)
}

func newPostgresPool(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse pg config: %w", err)
	}

	poolCfg.MaxConns = int32(cfg.DBMaxConns)
	poolCfg.MinConns = int32(cfg.DBMaxIdle)
	poolCfg.MaxConnLifetime = 30 * time.Minute
	poolCfg.MaxConnIdleTime = 10 * time.Minute

	pgPool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pg pool: %w", err)
	}

	if err = retry(ctx, cfg.StartupRetries, cfg.StartupRetryGap, func() error {
		pingCtx, cancel := context.WithTimeout(ctx, cfg.DBTimeout)
		defer cancel()
		return pgPool.Ping(pingCtx)
	}); err != nil {
		pgPool.Close()
		return nil, fmt.Errorf("postgres ping failed: %w", err)
	}

	return pgPool, nil
}

func retry(ctx context.Context, attempts int, gap time.Duration, fn func() error) error {
	if attempts < 1 {
		attempts = 1
	}
	if gap <= 0 {
		gap = time.Second
	}

	var lastErr error
	for i := 1; i <= attempts; i++ {
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
			slog.Warn("retryable startup error", "attempt", i, "max_attempts", attempts, "error", err)
		}

		if i == attempts {
			break
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(gap):
		}
	}

	return lastErr
}
