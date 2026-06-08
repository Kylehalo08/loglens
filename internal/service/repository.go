package service

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	CreateService(ctx context.Context, svc *RegisteredService) (*RegisteredService, error)
	GetServiceByID(ctx context.Context, orgID, serviceID string) (*RegisteredService, error)
	ListServicesByOrgID(ctx context.Context, orgID string) ([]RegisteredService, error)
	UpdateService(ctx context.Context, orgID, serviceID, name, description string) (*RegisteredService, error)
	SoftDeleteService(ctx context.Context, orgID, serviceID string) error
	CountActiveAPIKeysByServiceID(ctx context.Context, serviceID string) (int, error)
	CreateAPIKey(ctx context.Context, key *APIKey) (*APIKey, error)
	ListAPIKeysByServiceID(ctx context.Context, orgID, serviceID string) ([]APIKey, error)
	GetAPIKeyByID(ctx context.Context, orgID, serviceID, keyID string) (*APIKey, error)
	RevokeAPIKey(ctx context.Context, orgID, serviceID, keyID string) (*APIKey, error)
	RotateAPIKey(ctx context.Context, oldKey, newKey *APIKey) (*APIKey, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) CreateService(ctx context.Context, svc *RegisteredService) (*RegisteredService, error) {
	const query = `
		INSERT INTO services (org_id, name, description, created_by)
		VALUES ($1, $2, $3, $4)
		RETURNING id, org_id, name, COALESCE(description, ''), created_by, created_at, updated_at, deleted_at
	`

	created := &RegisteredService{}
	err := r.pool.QueryRow(
		ctx,
		query,
		svc.OrgID,
		svc.Name,
		nullString(svc.Description),
		svc.CreatedBy,
	).Scan(
		&created.ID,
		&created.OrgID,
		&created.Name,
		&created.Description,
		&created.CreatedBy,
		&created.CreatedAt,
		&created.UpdatedAt,
		&created.DeletedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDuplicateServiceName
		}
		return nil, err
	}

	return created, nil
}

func (r *PostgresRepository) GetServiceByID(ctx context.Context, orgID, serviceID string) (*RegisteredService, error) {
	const query = `
		SELECT id, org_id, name, COALESCE(description, ''), created_by, created_at, updated_at, deleted_at
		FROM services
		WHERE id = $1 AND org_id = $2 AND deleted_at IS NULL
	`

	svc := &RegisteredService{}
	err := r.pool.QueryRow(ctx, query, serviceID, orgID).Scan(
		&svc.ID,
		&svc.OrgID,
		&svc.Name,
		&svc.Description,
		&svc.CreatedBy,
		&svc.CreatedAt,
		&svc.UpdatedAt,
		&svc.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrServiceNotFound
	}
	if err != nil {
		return nil, err
	}

	return svc, nil
}

func (r *PostgresRepository) ListServicesByOrgID(ctx context.Context, orgID string) ([]RegisteredService, error) {
	const query = `
		SELECT id, org_id, name, COALESCE(description, ''), created_by, created_at, updated_at, deleted_at
		FROM services
		WHERE org_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	services := make([]RegisteredService, 0)
	for rows.Next() {
		var svc RegisteredService
		if err := rows.Scan(
			&svc.ID,
			&svc.OrgID,
			&svc.Name,
			&svc.Description,
			&svc.CreatedBy,
			&svc.CreatedAt,
			&svc.UpdatedAt,
			&svc.DeletedAt,
		); err != nil {
			return nil, err
		}
		services = append(services, svc)
	}

	return services, rows.Err()
}

func (r *PostgresRepository) UpdateService(ctx context.Context, orgID, serviceID, name, description string) (*RegisteredService, error) {
	const query = `
		UPDATE services
		SET name = $1, description = $2, updated_at = now()
		WHERE id = $3 AND org_id = $4 AND deleted_at IS NULL
		RETURNING id, org_id, name, COALESCE(description, ''), created_by, created_at, updated_at, deleted_at
	`

	svc := &RegisteredService{}
	err := r.pool.QueryRow(ctx, query, name, nullString(description), serviceID, orgID).Scan(
		&svc.ID,
		&svc.OrgID,
		&svc.Name,
		&svc.Description,
		&svc.CreatedBy,
		&svc.CreatedAt,
		&svc.UpdatedAt,
		&svc.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrServiceNotFound
	}
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDuplicateServiceName
		}
		return nil, err
	}

	return svc, nil
}

func (r *PostgresRepository) SoftDeleteService(ctx context.Context, orgID, serviceID string) error {
	const query = `
		UPDATE services
		SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND org_id = $2 AND deleted_at IS NULL
	`

	tag, err := r.pool.Exec(ctx, query, serviceID, orgID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrServiceNotFound
	}

	return nil
}

func (r *PostgresRepository) CountActiveAPIKeysByServiceID(ctx context.Context, serviceID string) (int, error) {
	const query = `
		SELECT COUNT(*)
		FROM service_api_keys
		WHERE service_id = $1 AND revoked_at IS NULL
	`

	var count int
	if err := r.pool.QueryRow(ctx, query, serviceID).Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}

func (r *PostgresRepository) CreateAPIKey(ctx context.Context, key *APIKey) (*APIKey, error) {
	const query = `
		INSERT INTO service_api_keys (
			service_id, org_id, prefix, key_hash, label, created_by, rotated_from_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, '')::uuid)
		RETURNING id, service_id, org_id, prefix, key_hash, COALESCE(label, ''),
		          created_by, created_at, revoked_at, last_used_at, COALESCE(rotated_from_id::text, '')
	`

	stored := &APIKey{}
	err := r.pool.QueryRow(
		ctx,
		query,
		key.ServiceID,
		key.OrgID,
		key.Prefix,
		key.KeyHash,
		nullString(key.Label),
		key.CreatedBy,
		key.RotatedFromID,
	).Scan(
		&stored.ID,
		&stored.ServiceID,
		&stored.OrgID,
		&stored.Prefix,
		&stored.KeyHash,
		&stored.Label,
		&stored.CreatedBy,
		&stored.CreatedAt,
		&stored.RevokedAt,
		&stored.LastUsedAt,
		&stored.RotatedFromID,
	)
	if err != nil {
		return nil, err
	}

	return stored, nil
}

func (r *PostgresRepository) ListAPIKeysByServiceID(ctx context.Context, orgID, serviceID string) ([]APIKey, error) {
	const query = `
		SELECT id, service_id, org_id, prefix, key_hash, COALESCE(label, ''),
		       created_by, created_at, revoked_at, last_used_at, COALESCE(rotated_from_id::text, '')
		FROM service_api_keys
		WHERE service_id = $1 AND org_id = $2
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, serviceID, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := make([]APIKey, 0)
	for rows.Next() {
		var key APIKey
		if err := rows.Scan(
			&key.ID,
			&key.ServiceID,
			&key.OrgID,
			&key.Prefix,
			&key.KeyHash,
			&key.Label,
			&key.CreatedBy,
			&key.CreatedAt,
			&key.RevokedAt,
			&key.LastUsedAt,
			&key.RotatedFromID,
		); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}

	return keys, rows.Err()
}

func (r *PostgresRepository) GetAPIKeyByID(ctx context.Context, orgID, serviceID, keyID string) (*APIKey, error) {
	const query = `
		SELECT id, service_id, org_id, prefix, key_hash, COALESCE(label, ''),
		       created_by, created_at, revoked_at, last_used_at, COALESCE(rotated_from_id::text, '')
		FROM service_api_keys
		WHERE id = $1 AND service_id = $2 AND org_id = $3
	`

	key := &APIKey{}
	err := r.pool.QueryRow(ctx, query, keyID, serviceID, orgID).Scan(
		&key.ID,
		&key.ServiceID,
		&key.OrgID,
		&key.Prefix,
		&key.KeyHash,
		&key.Label,
		&key.CreatedBy,
		&key.CreatedAt,
		&key.RevokedAt,
		&key.LastUsedAt,
		&key.RotatedFromID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAPIKeyNotFound
	}
	if err != nil {
		return nil, err
	}

	return key, nil
}

func (r *PostgresRepository) RevokeAPIKey(ctx context.Context, orgID, serviceID, keyID string) (*APIKey, error) {
	const query = `
		UPDATE service_api_keys
		SET revoked_at = now()
		WHERE id = $1 AND service_id = $2 AND org_id = $3 AND revoked_at IS NULL
		RETURNING id, service_id, org_id, prefix, key_hash, COALESCE(label, ''),
		          created_by, created_at, revoked_at, last_used_at, COALESCE(rotated_from_id::text, '')
	`

	key := &APIKey{}
	err := r.pool.QueryRow(ctx, query, keyID, serviceID, orgID).Scan(
		&key.ID,
		&key.ServiceID,
		&key.OrgID,
		&key.Prefix,
		&key.KeyHash,
		&key.Label,
		&key.CreatedBy,
		&key.CreatedAt,
		&key.RevokedAt,
		&key.LastUsedAt,
		&key.RotatedFromID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		existing, lookupErr := r.GetAPIKeyByID(ctx, orgID, serviceID, keyID)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if existing.RevokedAt != nil {
			return nil, ErrAPIKeyAlreadyRevoked
		}
		return nil, ErrAPIKeyNotFound
	}
	if err != nil {
		return nil, err
	}

	return key, nil
}

func (r *PostgresRepository) RotateAPIKey(ctx context.Context, oldKey, newKey *APIKey) (*APIKey, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	const revokeQuery = `
		UPDATE service_api_keys
		SET revoked_at = now()
		WHERE id = $1 AND service_id = $2 AND org_id = $3 AND revoked_at IS NULL
	`

	tag, err := tx.Exec(ctx, revokeQuery, oldKey.ID, oldKey.ServiceID, oldKey.OrgID)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrAPIKeyAlreadyRevoked
	}

	const insertQuery = `
		INSERT INTO service_api_keys (
			service_id, org_id, prefix, key_hash, label, created_by, rotated_from_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, service_id, org_id, prefix, key_hash, COALESCE(label, ''),
		          created_by, created_at, revoked_at, last_used_at, COALESCE(rotated_from_id::text, '')
	`

	stored := &APIKey{}
	err = tx.QueryRow(
		ctx,
		insertQuery,
		newKey.ServiceID,
		newKey.OrgID,
		newKey.Prefix,
		newKey.KeyHash,
		nullString(newKey.Label),
		newKey.CreatedBy,
		oldKey.ID,
	).Scan(
		&stored.ID,
		&stored.ServiceID,
		&stored.OrgID,
		&stored.Prefix,
		&stored.KeyHash,
		&stored.Label,
		&stored.CreatedBy,
		&stored.CreatedAt,
		&stored.RevokedAt,
		&stored.LastUsedAt,
		&stored.RotatedFromID,
	)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return stored, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
