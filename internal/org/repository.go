package org

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	CreateOrg(ctx context.Context, name, createdBy string) (*Organization, error)
	GetOrgByID(ctx context.Context, orgID string) (*Organization, error)
	GetOrgsByUserID(ctx context.Context, userID string) ([]OrgSummary, error)
	AddOrgMember(ctx context.Context, orgID, userID, role string) (*OrgMember, error)
	GetOrgMembers(ctx context.Context, orgID string) ([]OrgMember, error)
	GetOrgMemberRole(ctx context.Context, orgID, userID string) (string, error)
	CreateInviteToken(ctx context.Context, invite *OrgInvite) (*OrgInvite, error)
	GetInviteByToken(ctx context.Context, tokenHash string) (*OrgInvite, error)
	MarkInviteAccepted(ctx context.Context, inviteID string) error
	CreateInviteCode(ctx context.Context, code *OrgInviteCode) (*OrgInviteCode, error)
	GetInviteCodeByCode(ctx context.Context, code string) (*OrgInviteCode, error)
	CountServicesByOrgID(ctx context.Context, orgID string) (int, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) CreateOrg(ctx context.Context, name, createdBy string) (*Organization, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	const orgQuery = `
		INSERT INTO organizations (name, created_by)
		VALUES ($1, $2)
		RETURNING id, name, created_by, created_at, updated_at
	`

	org := &Organization{}
	err = tx.QueryRow(ctx, orgQuery, name, createdBy).Scan(
		&org.ID,
		&org.Name,
		&org.CreatedBy,
		&org.CreatedAt,
		&org.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	const memberQuery = `
		INSERT INTO org_members (org_id, user_id, role)
		VALUES ($1, $2, $3)
	`

	if _, err := tx.Exec(ctx, memberQuery, org.ID, createdBy, RoleOwner); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return org, nil
}

func (r *PostgresRepository) GetOrgByID(ctx context.Context, orgID string) (*Organization, error) {
	const query = `
		SELECT id, name, created_by, created_at, updated_at
		FROM organizations
		WHERE id = $1
	`

	org := &Organization{}
	err := r.pool.QueryRow(ctx, query, orgID).Scan(
		&org.ID,
		&org.Name,
		&org.CreatedBy,
		&org.CreatedAt,
		&org.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOrgNotFound
	}
	if err != nil {
		return nil, err
	}

	return org, nil
}

func (r *PostgresRepository) GetOrgsByUserID(ctx context.Context, userID string) ([]OrgSummary, error) {
	const query = `
		SELECT o.id, o.name, o.created_by, o.created_at, om.role
		FROM organizations o
		INNER JOIN org_members om ON om.org_id = o.id
		WHERE om.user_id = $1
		ORDER BY o.created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orgs := make([]OrgSummary, 0)
	for rows.Next() {
		var summary OrgSummary
		if err := rows.Scan(
			&summary.ID,
			&summary.Name,
			&summary.CreatedBy,
			&summary.CreatedAt,
			&summary.Role,
		); err != nil {
			return nil, err
		}
		orgs = append(orgs, summary)
	}

	return orgs, rows.Err()
}

func (r *PostgresRepository) AddOrgMember(ctx context.Context, orgID, userID, role string) (*OrgMember, error) {
	const query = `
		INSERT INTO org_members (org_id, user_id, role)
		VALUES ($1, $2, $3)
		RETURNING org_id, user_id, role, joined_at
	`

	member := &OrgMember{}
	err := r.pool.QueryRow(ctx, query, orgID, userID, role).Scan(
		&member.OrgID,
		&member.UserID,
		&member.Role,
		&member.JoinedAt,
	)
	if err != nil {
		return nil, err
	}

	return member, nil
}

func (r *PostgresRepository) GetOrgMembers(ctx context.Context, orgID string) ([]OrgMember, error) {
	const query = `
		SELECT om.org_id, om.user_id, u.email, om.role, om.joined_at
		FROM org_members om
		INNER JOIN users u ON u.id = om.user_id
		WHERE om.org_id = $1
		ORDER BY om.joined_at ASC
	`

	rows, err := r.pool.Query(ctx, query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := make([]OrgMember, 0)
	for rows.Next() {
		var member OrgMember
		if err := rows.Scan(
			&member.OrgID,
			&member.UserID,
			&member.Email,
			&member.Role,
			&member.JoinedAt,
		); err != nil {
			return nil, err
		}
		members = append(members, member)
	}

	return members, rows.Err()
}

func (r *PostgresRepository) GetOrgMemberRole(ctx context.Context, orgID, userID string) (string, error) {
	const query = `
		SELECT role
		FROM org_members
		WHERE org_id = $1 AND user_id = $2
	`

	var role string
	err := r.pool.QueryRow(ctx, query, orgID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotOrgMember
	}
	if err != nil {
		return "", err
	}

	return role, nil
}

func (r *PostgresRepository) CreateInviteToken(ctx context.Context, invite *OrgInvite) (*OrgInvite, error) {
	const query = `
		INSERT INTO org_invites (org_id, invited_by, email, role, token_hash, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, org_id, invited_by, email, role, token_hash, expires_at, accepted_at, created_at
	`

	stored := &OrgInvite{}
	err := r.pool.QueryRow(
		ctx,
		query,
		invite.OrgID,
		invite.InvitedBy,
		invite.Email,
		invite.Role,
		invite.TokenHash,
		invite.ExpiresAt,
	).Scan(
		&stored.ID,
		&stored.OrgID,
		&stored.InvitedBy,
		&stored.Email,
		&stored.Role,
		&stored.TokenHash,
		&stored.ExpiresAt,
		&stored.AcceptedAt,
		&stored.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return stored, nil
}

func (r *PostgresRepository) GetInviteByToken(ctx context.Context, tokenHash string) (*OrgInvite, error) {
	const query = `
		SELECT id, org_id, invited_by, email, role, token_hash, expires_at, accepted_at, created_at
		FROM org_invites
		WHERE token_hash = $1
	`

	invite := &OrgInvite{}
	err := r.pool.QueryRow(ctx, query, tokenHash).Scan(
		&invite.ID,
		&invite.OrgID,
		&invite.InvitedBy,
		&invite.Email,
		&invite.Role,
		&invite.TokenHash,
		&invite.ExpiresAt,
		&invite.AcceptedAt,
		&invite.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInviteNotFound
	}
	if err != nil {
		return nil, err
	}

	return invite, nil
}

func (r *PostgresRepository) MarkInviteAccepted(ctx context.Context, inviteID string) error {
	const query = `
		UPDATE org_invites
		SET accepted_at = now()
		WHERE id = $1 AND accepted_at IS NULL
	`

	tag, err := r.pool.Exec(ctx, query, inviteID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrInviteAlreadyAccepted
	}

	return nil
}

func (r *PostgresRepository) CreateInviteCode(ctx context.Context, code *OrgInviteCode) (*OrgInviteCode, error) {
	const query = `
		INSERT INTO org_invite_codes (org_id, code, created_by, default_role, is_active)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, org_id, code, created_by, default_role, is_active, created_at
	`

	stored := &OrgInviteCode{}
	err := r.pool.QueryRow(
		ctx,
		query,
		code.OrgID,
		code.Code,
		code.CreatedBy,
		code.DefaultRole,
		code.IsActive,
	).Scan(
		&stored.ID,
		&stored.OrgID,
		&stored.Code,
		&stored.CreatedBy,
		&stored.DefaultRole,
		&stored.IsActive,
		&stored.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return stored, nil
}

func (r *PostgresRepository) GetInviteCodeByCode(ctx context.Context, code string) (*OrgInviteCode, error) {
	const query = `
		SELECT id, org_id, code, created_by, default_role, is_active, created_at
		FROM org_invite_codes
		WHERE code = $1
	`

	inviteCode := &OrgInviteCode{}
	err := r.pool.QueryRow(ctx, query, code).Scan(
		&inviteCode.ID,
		&inviteCode.OrgID,
		&inviteCode.Code,
		&inviteCode.CreatedBy,
		&inviteCode.DefaultRole,
		&inviteCode.IsActive,
		&inviteCode.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInviteCodeNotFound
	}
	if err != nil {
		return nil, err
	}

	return inviteCode, nil
}

func (r *PostgresRepository) CountServicesByOrgID(_ context.Context, _ string) (int, error) {
	return 0, nil
}
