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

func (r *Repository) GetLogByOrgID(ctx context.Context, orgID, logID string) (*LogEntry, error) {
	const query = `
		SELECT id, org_id, service_id, timestamp, severity, message, metadata, ingested_at
		FROM logs
		WHERE id = $1 AND org_id = $2
	`

	entry := &LogEntry{}
	var metadataJSON []byte
	err := r.pool.QueryRow(ctx, query, logID, orgID).Scan(
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

func (r *Repository) SearchLogs(ctx context.Context, filters SearchFilters) (*SearchResult, error) {
	const whereClause = `
		WHERE org_id = $1
		  AND ($2::uuid[] IS NULL OR service_id = ANY($2))
		  AND ($3::text[] IS NULL OR severity = ANY($3))
		  AND ($4::timestamptz IS NULL OR timestamp >= $4)
		  AND ($5::timestamptz IS NULL OR timestamp <= $5)
		  AND ($6::text IS NULL OR message_tsv @@ plainto_tsquery('english', $6))
	`

	countQuery := `SELECT COUNT(*) FROM logs` + whereClause
	selectQuery := `
		SELECT id, org_id, service_id, timestamp, severity, message, metadata, ingested_at
		FROM logs` + whereClause + `
		ORDER BY timestamp DESC, id DESC
		LIMIT $7 OFFSET $8
	`

	var serviceIDs []string
	if len(filters.ServiceIDs) > 0 {
		serviceIDs = filters.ServiceIDs
	}

	var severities []string
	if len(filters.Severities) > 0 {
		severities = filters.Severities
	}

	var keyword *string
	if filters.Query != "" {
		keyword = &filters.Query
	}

	var total int64
	err := r.pool.QueryRow(
		ctx,
		countQuery,
		filters.OrgID,
		serviceIDs,
		severities,
		filters.From,
		filters.To,
		keyword,
	).Scan(&total)
	if err != nil {
		return nil, err
	}

	offset := (filters.Page - 1) * filters.Limit
	rows, err := r.pool.Query(
		ctx,
		selectQuery,
		filters.OrgID,
		serviceIDs,
		severities,
		filters.From,
		filters.To,
		keyword,
		filters.Limit,
		offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := make([]LogEntry, 0)
	for rows.Next() {
		entry := LogEntry{}
		var metadataJSON []byte
		if err := rows.Scan(
			&entry.ID,
			&entry.OrgID,
			&entry.ServiceID,
			&entry.Timestamp,
			&entry.Severity,
			&entry.Message,
			&metadataJSON,
			&entry.IngestedAt,
		); err != nil {
			return nil, err
		}
		if len(metadataJSON) > 0 {
			_ = json.Unmarshal(metadataJSON, &entry.Metadata)
		}
		logs = append(logs, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(filters.Limit) - 1) / int64(filters.Limit))
	}

	return &SearchResult{
		Logs: logs,
		Pagination: Pagination{
			Page:       filters.Page,
			Limit:      filters.Limit,
			Total:      total,
			TotalPages: totalPages,
		},
	}, nil
}

func (r *Repository) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	const query = `DELETE FROM logs WHERE timestamp < $1`
	tag, err := r.pool.Exec(ctx, query, cutoff)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
