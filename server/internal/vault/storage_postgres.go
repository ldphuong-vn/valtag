package vault

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Storage defines the blob storage interface for encrypted vault objects.
// The default implementation keeps opaque ciphertext rows in Postgres
// (secret_blobs): blobs are KV bytes, so a separate object store adds an
// extra service without adding capability. The interface stays so an
// S3-compatible driver can be added for scale-out deployments.
type Storage interface {
	Put(ctx context.Context, key string, data []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}

// PostgresStorage implements Storage backed by the secret_blobs table.
type PostgresStorage struct {
	pool *pgxpool.Pool
}

// NewPostgresStorage creates a PostgresStorage.
func NewPostgresStorage(pool *pgxpool.Pool) *PostgresStorage {
	return &PostgresStorage{pool: pool}
}

// Put stores data under the given key as an opaque binary blob.
func (s *PostgresStorage) Put(ctx context.Context, key string, data []byte) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO secret_blobs (storage_key, data) VALUES ($1, $2)
		ON CONFLICT (storage_key) DO UPDATE
		SET data = EXCLUDED.data, updated_at = NOW()`,
		key, data,
	)
	if err != nil {
		return fmt.Errorf("storing blob: %w", err)
	}
	return nil
}

// Get retrieves and returns the raw bytes stored under key.
func (s *PostgresStorage) Get(ctx context.Context, key string) ([]byte, error) {
	var data []byte
	err := s.pool.QueryRow(ctx,
		`SELECT data FROM secret_blobs WHERE storage_key = $1`, key,
	).Scan(&data)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("blob %q not found", key)
		}
		return nil, fmt.Errorf("reading blob: %w", err)
	}
	return data, nil
}

// Delete removes the blob identified by key.
func (s *PostgresStorage) Delete(ctx context.Context, key string) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM secret_blobs WHERE storage_key = $1`, key); err != nil {
		return fmt.Errorf("deleting blob: %w", err)
	}
	return nil
}
