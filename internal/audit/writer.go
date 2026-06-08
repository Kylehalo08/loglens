package audit

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
)

const ActorTypeUser = "user"

type Event struct {
	OrgID        string
	ActorID      string
	ActorType    string
	Action       string
	ResourceType string
	ResourceID   string
	Metadata     map[string]any
	IPAddress    string
}

type Writer interface {
	Write(ctx context.Context, event Event) error
}

type PostgresWriter struct {
	pool *pgxpool.Pool
}

func NewPostgresWriter(pool *pgxpool.Pool) *PostgresWriter {
	return &PostgresWriter{pool: pool}
}

func (w *PostgresWriter) Write(ctx context.Context, event Event) error {
	metadata := event.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	actorType := event.ActorType
	if actorType == "" {
		actorType = ActorTypeUser
	}

	const query = `
		INSERT INTO audit_events (
			org_id, actor_id, actor_type, action, resource_type, resource_id, metadata, ip_address
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, '')::inet)
	`

	_, err = w.pool.Exec(
		ctx,
		query,
		nullUUID(event.OrgID),
		nullUUID(event.ActorID),
		actorType,
		event.Action,
		event.ResourceType,
		nullUUID(event.ResourceID),
		metadataJSON,
		event.IPAddress,
	)
	return err
}

func nullUUID(value string) any {
	if value == "" {
		return nil
	}
	return value
}
