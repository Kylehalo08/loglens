package org

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"net/mail"
	"time"

	"loglens/internal/auth"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	inviteTokenTTL  = 24 * time.Hour
	inviteCodeChars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	inviteCodeLen   = 6
)

type Service struct {
	repo   Repository
	tokens auth.TokenManager
	cache  InviteTokenCache
}

func NewService(repo Repository, tokens auth.TokenManager, cache InviteTokenCache) *Service {
	return &Service{
		repo:   repo,
		tokens: tokens,
		cache:  cache,
	}
}

func (s *Service) CreateOrganization(ctx context.Context, userID, name string) (*CreateOrgResponse, error) {
	if name == "" {
		return nil, ErrInvalidOrgName
	}

	org, err := s.repo.CreateOrg(ctx, name, userID)
	if err != nil {
		return nil, err
	}

	return &CreateOrgResponse{
		ID:        org.ID,
		Name:      org.Name,
		CreatedBy: org.CreatedBy,
		CreatedAt: org.CreatedAt,
		Role:      RoleOwner,
	}, nil
}

func (s *Service) GetOrganization(ctx context.Context, orgID, userID string) (*OrgDetailResponse, error) {
	if _, err := s.repo.GetOrgMemberRole(ctx, orgID, userID); err != nil {
		return nil, err
	}

	org, err := s.repo.GetOrgByID(ctx, orgID)
	if err != nil {
		return nil, err
	}

	members, err := s.repo.GetOrgMembers(ctx, orgID)
	if err != nil {
		return nil, err
	}

	servicesCount, err := s.repo.CountServicesByOrgID(ctx, orgID)
	if err != nil {
		return nil, err
	}

	memberResponses := make([]MemberResponse, 0, len(members))
	for _, member := range members {
		memberResponses = append(memberResponses, MemberResponse{
			UserID:   member.UserID,
			Email:    member.Email,
			Role:     member.Role,
			JoinedAt: member.JoinedAt,
		})
	}

	return &OrgDetailResponse{
		ID:            org.ID,
		Name:          org.Name,
		CreatedAt:     org.CreatedAt,
		Members:       memberResponses,
		ServicesCount: servicesCount,
	}, nil
}

func (s *Service) ListMyOrgs(ctx context.Context, userID string) ([]OrgSummary, error) {
	return s.repo.GetOrgsByUserID(ctx, userID)
}

func (s *Service) SendEmailInvite(ctx context.Context, orgID, invitedBy, email, role string) (*SendInviteResponse, error) {
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, ErrInvalidEmail
	}
	if !IsValidInviteRole(role) {
		return nil, ErrInvalidInviteRole
	}

	memberRole, err := s.repo.GetOrgMemberRole(ctx, orgID, invitedBy)
	if err != nil {
		return nil, err
	}
	if !IsAdminRole(memberRole) {
		return nil, ErrInsufficientPermissions
	}

	rawToken := uuid.NewString()
	tokenHash := s.tokens.HashToken(rawToken)
	expiresAt := time.Now().Add(inviteTokenTTL)

	stored, err := s.repo.CreateInviteToken(ctx, &OrgInvite{
		OrgID:     orgID,
		InvitedBy: invitedBy,
		Email:     email,
		Role:      role,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return nil, err
	}

	payload := InviteTokenPayload{
		InviteID: stored.ID,
		OrgID:    orgID,
		Email:    email,
		Role:     role,
	}
	s.storeInviteTokenInCache(ctx, rawToken, payload)

	return &SendInviteResponse{
		InviteID:  stored.ID,
		Email:     stored.Email,
		Role:      stored.Role,
		ExpiresAt: stored.ExpiresAt,
		Token:     rawToken,
	}, nil
}

func (s *Service) JoinViaToken(ctx context.Context, userID, rawToken string) (*JoinResponse, error) {
	if rawToken == "" {
		return nil, ErrInviteNotFound
	}

	var (
		orgID  string
		role   string
		invite *OrgInvite
	)

	if payload, err := s.lookupInviteTokenInCache(ctx, rawToken); err == nil && payload != nil {
		orgID = payload.OrgID
		role = payload.Role
		invite, err = s.repo.GetInviteByToken(ctx, s.tokens.HashToken(rawToken))
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		invite, err = s.repo.GetInviteByToken(ctx, s.tokens.HashToken(rawToken))
		if err != nil {
			return nil, err
		}
		orgID = invite.OrgID
		role = invite.Role
	}

	if invite.AcceptedAt != nil {
		return nil, ErrInviteAlreadyAccepted
	}
	if time.Now().After(invite.ExpiresAt) {
		return nil, ErrInviteExpired
	}

	if _, err := s.repo.GetOrgMemberRole(ctx, orgID, userID); err == nil {
		return nil, ErrAlreadyOrgMember
	} else if !errors.Is(err, ErrNotOrgMember) {
		return nil, err
	}

	member, err := s.repo.AddOrgMember(ctx, orgID, userID, role)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrAlreadyOrgMember
		}
		return nil, err
	}

	if err := s.repo.MarkInviteAccepted(ctx, invite.ID); err != nil && !errors.Is(err, ErrInviteAlreadyAccepted) {
		return nil, err
	}

	s.deleteInviteTokenFromCache(ctx, rawToken)

	org, err := s.repo.GetOrgByID(ctx, orgID)
	if err != nil {
		return nil, err
	}

	return &JoinResponse{
		OrgID:    org.ID,
		OrgName:  org.Name,
		Role:     member.Role,
		JoinedAt: member.JoinedAt,
	}, nil
}

func (s *Service) GenerateInviteCode(ctx context.Context, orgID, createdBy string) (*InviteCodeResponse, error) {
	memberRole, err := s.repo.GetOrgMemberRole(ctx, orgID, createdBy)
	if err != nil {
		return nil, err
	}
	if !IsAdminRole(memberRole) {
		return nil, ErrInsufficientPermissions
	}

	code, err := generateInviteCode()
	if err != nil {
		return nil, err
	}

	stored, err := s.repo.CreateInviteCode(ctx, &OrgInviteCode{
		OrgID:       orgID,
		Code:        code,
		CreatedBy:   createdBy,
		DefaultRole: RoleDeveloper,
		IsActive:    true,
	})
	if err != nil {
		return nil, err
	}

	return &InviteCodeResponse{
		Code:        stored.Code,
		OrgID:       stored.OrgID,
		DefaultRole: stored.DefaultRole,
		IsActive:    stored.IsActive,
	}, nil
}

func (s *Service) JoinViaCode(ctx context.Context, userID, code string) (*JoinResponse, error) {
	if code == "" {
		return nil, ErrInviteCodeNotFound
	}

	inviteCode, err := s.repo.GetInviteCodeByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if !inviteCode.IsActive {
		return nil, ErrInviteCodeInactive
	}

	if _, err := s.repo.GetOrgMemberRole(ctx, inviteCode.OrgID, userID); err == nil {
		return nil, ErrAlreadyOrgMember
	} else if !errors.Is(err, ErrNotOrgMember) {
		return nil, err
	}

	member, err := s.repo.AddOrgMember(ctx, inviteCode.OrgID, userID, inviteCode.DefaultRole)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrAlreadyOrgMember
		}
		return nil, err
	}

	org, err := s.repo.GetOrgByID(ctx, inviteCode.OrgID)
	if err != nil {
		return nil, err
	}

	return &JoinResponse{
		OrgID:    org.ID,
		OrgName:  org.Name,
		Role:     member.Role,
		JoinedAt: member.JoinedAt,
	}, nil
}

func (s *Service) GetOrgMemberRole(ctx context.Context, orgID, userID string) (string, error) {
	return s.repo.GetOrgMemberRole(ctx, orgID, userID)
}

func (s *Service) storeInviteTokenInCache(ctx context.Context, token string, payload InviteTokenPayload) {
	if s.cache == nil || !s.cache.IsAvailable() {
		return
	}
	_ = s.cache.Set(ctx, token, payload, inviteTokenTTL)
}

func (s *Service) lookupInviteTokenInCache(ctx context.Context, token string) (*InviteTokenPayload, error) {
	if s.cache == nil || !s.cache.IsAvailable() {
		return nil, errors.New("cache unavailable")
	}
	return s.cache.Get(ctx, token)
}

func (s *Service) deleteInviteTokenFromCache(ctx context.Context, token string) {
	if s.cache == nil || !s.cache.IsAvailable() {
		return
	}
	_ = s.cache.Delete(ctx, token)
}

func generateInviteCode() (string, error) {
	code := make([]byte, inviteCodeLen)
	max := big.NewInt(int64(len(inviteCodeChars)))

	for i := range code {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		code[i] = inviteCodeChars[n.Int64()]
	}

	return string(code), nil
}
