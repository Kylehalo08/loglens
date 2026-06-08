package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrLogNotFound = errors.New("log not found")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) InsertLogs(ctx context.Context, entries []LogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	const query = `
		INSERT INTO logs (id, org_id, service_id, timestamp, severity, message, metadata, ingested_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	for _, entry := range entries {
		metadata := entry.Metadata
		if metadata == nil {
			metadata = map[string]any{}
		}
		metadataJSON, err := json.Marshal(metadata)
		if err != nil {
			return err
		}
		batch.Queue(
			query,
			entry.ID,
			entry.OrgID,
			entry.ServiceID,
			entry.Timestamp,
			entry.Severity,
			entry.Message,
			metadataJSON,
			entry.IngestedAt,
		)
	}

	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()

	for range entries {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}

	return nil
}

func (r *Repository) GetLogByID(ctx context.Context, orgID, serviceID, logID string) (*LogEntry, error) {
	const query = `
		SELECT id, org_id, service_id, timestamp, severity, message, metadata, ingested_at
		FROM logs
		WHERE id = $1 AND org_id = $2 AND service_id = $3
	`

	entry := &LogEntry{}
	var metadataJSON []byte
	err := r.pool.QueryRow(ctx, query, logID, orgID, serviceID).Scan(
		&entry.ID,
		&entry.OrgID,
		&entry.ServiceID,
		&entry.Timestamp,
		&entry.Severity,
		&entry.Message,
		&metadataJSON,
		&entry.IngestedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrLogNotFound
	}
	if err != nil {
		return nil, err
	}

	if len(metadataJSON) > 0 {
		_ = json.Unmarshal(metadataJSON, &entry.Metadata)
	}

	return entry, nil
}

func (r *Repository) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	const query = `DELETE FROM logs WHERE timestamp < $1`
	tag, err := r.pool.Exec(ctx, query, cutoff)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
