package storage

import (
	"context"
	"fmt"

	"myapp/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *PostgresStore) Select1(ctx context.Context) error {
	var one int
	if err := s.pool.QueryRow(ctx, "SELECT 1").Scan(&one); err != nil {
		return fmt.Errorf("select 1: %w", err)
	}
	return nil
}

func (s *PostgresStore) CreateNote(ctx context.Context, title, content string) (domain.Note, error) {
	row := s.pool.QueryRow(
		ctx,
		`INSERT INTO notes (title, content) VALUES ($1, $2) RETURNING id, title, content, created_at`,
		title,
		content,
	)

	var n domain.Note
	if err := row.Scan(&n.ID, &n.Title, &n.Content, &n.CreatedAt); err != nil {
		return domain.Note{}, fmt.Errorf("create note: %w", err)
	}

	return n, nil
}

func (s *PostgresStore) ListNotes(ctx context.Context) ([]domain.Note, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, title, content, created_at FROM notes ORDER BY id DESC`)
	if err != nil {
		return nil, fmt.Errorf("query notes: %w", err)
	}
	defer rows.Close()

	notes := make([]domain.Note, 0, 32)
	for rows.Next() {
		var n domain.Note
		if scanErr := rows.Scan(&n.ID, &n.Title, &n.Content, &n.CreatedAt); scanErr != nil {
			return nil, fmt.Errorf("scan note: %w", scanErr)
		}
		notes = append(notes, n)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notes: %w", err)
	}

	return notes, nil
}
