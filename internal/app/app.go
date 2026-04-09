package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"myapp/internal/domain"
	"myapp/internal/metrics"
	"myapp/internal/storage"

	"github.com/redis/go-redis/v9"
)

const notesCacheKey = "notes:all"

type Application struct {
	pg           *storage.PostgresStore
	redis        *storage.RedisStore
	dbTimeout    time.Duration
	redisTimeout time.Duration
	cacheTTL     time.Duration
	metrics      *metrics.Metrics
}

func New(
	pg *storage.PostgresStore,
	redisStore *storage.RedisStore,
	dbTimeout time.Duration,
	redisTimeout time.Duration,
	cacheTTL time.Duration,
	m *metrics.Metrics,
) *Application {
	if m == nil {
		m = metrics.New()
	}

	return &Application{
		pg:           pg,
		redis:        redisStore,
		dbTimeout:    dbTimeout,
		redisTimeout: redisTimeout,
		cacheTTL:     cacheTTL,
		metrics:      m,
	}
}

func (a *Application) Health(ctx context.Context) error {
	_ = ctx
	return nil
}

func (a *Application) Ready(ctx context.Context) error {
	dbCtx, dbCancel := context.WithTimeout(ctx, a.dbTimeout)
	defer dbCancel()
	if err := a.pg.Select1(dbCtx); err != nil {
		return fmt.Errorf("postgres not ready: %w", err)
	}

	redisCtx, redisCancel := context.WithTimeout(ctx, a.redisTimeout)
	defer redisCancel()
	if err := a.redis.Ping(redisCtx); err != nil {
		return fmt.Errorf("redis not ready: %w", err)
	}
	return nil
}

func (a *Application) CreateNote(ctx context.Context, title, content string) (domain.Note, error) {
	dbCtx, dbCancel := context.WithTimeout(ctx, a.dbTimeout)
	defer dbCancel()
	note, err := a.pg.CreateNote(dbCtx, title, content)
	if err != nil {
		a.metrics.IncDBErrors()
		return domain.Note{}, err
	}

	redisCtx, redisCancel := context.WithTimeout(ctx, a.redisTimeout)
	defer redisCancel()
	if err = a.redis.Del(redisCtx, notesCacheKey); err != nil {
		a.metrics.IncRedisErrors()
		return domain.Note{}, fmt.Errorf("invalidate cache: %w", err)
	}

	return note, nil
}

func (a *Application) ListNotes(ctx context.Context) ([]domain.Note, error) {
	redisReadCtx, redisReadCancel := context.WithTimeout(ctx, a.redisTimeout)
	defer redisReadCancel()
	cached, err := a.redis.Get(redisReadCtx, notesCacheKey)
	if err == nil {
		a.metrics.IncCacheHit()
		notes := make([]domain.Note, 0)
		if unmarshalErr := json.Unmarshal([]byte(cached), &notes); unmarshalErr == nil {
			return notes, nil
		}
	}

	if err != nil && !errors.Is(err, redis.Nil) {
		a.metrics.IncRedisErrors()
		return nil, fmt.Errorf("read cache: %w", err)
	}
	a.metrics.IncCacheMiss()

	dbCtx, dbCancel := context.WithTimeout(ctx, a.dbTimeout)
	defer dbCancel()
	notes, err := a.pg.ListNotes(dbCtx)
	if err != nil {
		a.metrics.IncDBErrors()
		return nil, err
	}

	serialized, err := json.Marshal(notes)
	if err != nil {
		return notes, nil
	}

	redisWriteCtx, redisWriteCancel := context.WithTimeout(ctx, a.redisTimeout)
	defer redisWriteCancel()
	if err = a.redis.Set(redisWriteCtx, notesCacheKey, string(serialized), a.cacheTTL); err != nil {
		a.metrics.IncRedisErrors()
		return nil, fmt.Errorf("write cache: %w", err)
	}

	return notes, nil
}
