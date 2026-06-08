package service

import (
	"context"
	"errors"
	"strings"

	"loglens/internal/audit"
	"loglens/internal/auth"
	"loglens/internal/org"
)

const maxServiceNameLen = 255

type KeyInvalidator interface {
	Invalidate(ctx context.Context, prefix string) error
}

type Service struct {
	repo           Repository
	audit          audit.Writer
	orgAccess      OrgAccess
	keyInvalidator KeyInvalidator
}

func NewService(repo Repository, auditWriter audit.Writer, orgAccess OrgAccess, keyInvalidator KeyInvalidator) *Service {
	return &Service{
		repo:           repo,
		audit:          auditWriter,
		orgAccess:      orgAccess,
		keyInvalidator: keyInvalidator,
	}
}

func (s *Service) CreateService(ctx context.Context, orgID, userID, name, description, ip string) (*ServiceResponse, error) {
	if err := s.requireDeveloper(ctx, orgID, userID); err != nil {
		return nil, err
	}

	normalizedName, err := normalizeServiceName(name)
	if err != nil {
		return nil, err
	}

	created, err := s.repo.CreateService(ctx, &RegisteredService{
		OrgID:       orgID,
		Name:        normalizedName,
		Description: strings.TrimSpace(description),
		CreatedBy:   userID,
	})
	if err != nil {
		return nil, err
	}

	s.writeAudit(ctx, audit.Event{
		OrgID:        orgID,
		ActorID:      userID,
		Action:       "service.created",
		ResourceType: "service",
		ResourceID:   created.ID,
		Metadata:     map[string]any{"name": created.Name},
		IPAddress:    ip,
	})

	count := 0
	return toServiceResponse(created, &count), nil
}

func (s *Service) ListServices(ctx context.Context, orgID, userID string) ([]ServiceResponse, error) {
	role, err := s.requireMember(ctx, orgID, userID)
	if err != nil {
		return nil, err
	}

	services, err := s.repo.ListServicesByOrgID(ctx, orgID)
	if err != nil {
		return nil, err
	}

	showKeyCounts := IsDeveloperRole(role)
	results := make([]ServiceResponse, 0, len(services))
	for i := range services {
		var activeCount *int
		if showKeyCounts {
			count, err := s.repo.CountActiveAPIKeysByServiceID(ctx, services[i].ID)
			if err != nil {
				return nil, err
			}
			activeCount = &count
		}
		results = append(results, *toServiceResponse(&services[i], activeCount))
	}

	return results, nil
}

func (s *Service) GetService(ctx context.Context, orgID, serviceID, userID string) (*ServiceResponse, error) {
	role, err := s.requireMember(ctx, orgID, userID)
	if err != nil {
		return nil, err
	}

	svc, err := s.repo.GetServiceByID(ctx, orgID, serviceID)
	if err != nil {
		return nil, err
	}

	var activeCount *int
	if IsDeveloperRole(role) {
		count, err := s.repo.CountActiveAPIKeysByServiceID(ctx, serviceID)
		if err != nil {
			return nil, err
		}
		activeCount = &count
	}

	return toServiceResponse(svc, activeCount), nil
}

func (s *Service) UpdateService(ctx context.Context, orgID, serviceID, userID, name, description, ip string) (*ServiceResponse, error) {
	if err := s.requireDeveloper(ctx, orgID, userID); err != nil {
		return nil, err
	}

	normalizedName, err := normalizeServiceName(name)
	if err != nil {
		return nil, err
	}

	updated, err := s.repo.UpdateService(ctx, orgID, serviceID, normalizedName, strings.TrimSpace(description))
	if err != nil {
		return nil, err
	}

	s.writeAudit(ctx, audit.Event{
		OrgID:        orgID,
		ActorID:      userID,
		Action:       "service.updated",
		ResourceType: "service",
		ResourceID:   updated.ID,
		Metadata:     map[string]any{"name": updated.Name},
		IPAddress:    ip,
	})

	count, err := s.repo.CountActiveAPIKeysByServiceID(ctx, serviceID)
	if err != nil {
		return nil, err
	}

	return toServiceResponse(updated, &count), nil
}

func (s *Service) DeleteService(ctx context.Context, orgID, serviceID, userID, ip string) error {
	if err := s.requireDeveloper(ctx, orgID, userID); err != nil {
		return err
	}

	if err := s.repo.SoftDeleteService(ctx, orgID, serviceID); err != nil {
		return err
	}

	s.writeAudit(ctx, audit.Event{
		OrgID:        orgID,
		ActorID:      userID,
		Action:       "service.deleted",
		ResourceType: "service",
		ResourceID:   serviceID,
		IPAddress:    ip,
	})

	return nil
}

func (s *Service) GenerateAPIKey(ctx context.Context, orgID, serviceID, userID, label, ip string) (*CreateAPIKeyResponse, error) {
	if err := s.requireDeveloper(ctx, orgID, userID); err != nil {
		return nil, err
	}

	if _, err := s.repo.GetServiceByID(ctx, orgID, serviceID); err != nil {
		return nil, err
	}

	raw, prefix, err := auth.GenerateAPIKey()
	if err != nil {
		return nil, err
	}

	hash, err := auth.HashAPIKey(raw)
	if err != nil {
		return nil, err
	}

	stored, err := s.repo.CreateAPIKey(ctx, &APIKey{
		ServiceID: serviceID,
		OrgID:     orgID,
		Prefix:    prefix,
		KeyHash:   hash,
		Label:     strings.TrimSpace(label),
		CreatedBy: userID,
	})
	if err != nil {
		return nil, err
	}

	s.writeAudit(ctx, audit.Event{
		OrgID:        orgID,
		ActorID:      userID,
		Action:       "api_key.created",
		ResourceType: "api_key",
		ResourceID:   stored.ID,
		Metadata:     map[string]any{"service_id": serviceID, "prefix": stored.Prefix},
		IPAddress:    ip,
	})

	return &CreateAPIKeyResponse{
		ID:        stored.ID,
		ServiceID: stored.ServiceID,
		Prefix:    stored.Prefix,
		APIKey:    raw,
		Label:     stored.Label,
		CreatedAt: stored.CreatedAt,
		CreatedBy: stored.CreatedBy,
	}, nil
}

func (s *Service) ListAPIKeys(ctx context.Context, orgID, serviceID, userID string) ([]APIKeyResponse, error) {
	if err := s.requireDeveloper(ctx, orgID, userID); err != nil {
		return nil, err
	}

	if _, err := s.repo.GetServiceByID(ctx, orgID, serviceID); err != nil {
		return nil, err
	}

	keys, err := s.repo.ListAPIKeysByServiceID(ctx, orgID, serviceID)
	if err != nil {
		return nil, err
	}

	results := make([]APIKeyResponse, 0, len(keys))
	for i := range keys {
		results = append(results, toAPIKeyResponse(&keys[i]))
	}

	return results, nil
}

func (s *Service) RevokeAPIKey(ctx context.Context, orgID, serviceID, keyID, userID, ip string) (*APIKeyResponse, error) {
	if err := s.requireDeveloper(ctx, orgID, userID); err != nil {
		return nil, err
	}

	if _, err := s.repo.GetServiceByID(ctx, orgID, serviceID); err != nil {
		return nil, err
	}

	revoked, err := s.repo.RevokeAPIKey(ctx, orgID, serviceID, keyID)
	if err != nil {
		return nil, err
	}

	s.writeAudit(ctx, audit.Event{
		OrgID:        orgID,
		ActorID:      userID,
		Action:       "api_key.revoked",
		ResourceType: "api_key",
		ResourceID:   revoked.ID,
		Metadata:     map[string]any{"service_id": serviceID, "prefix": revoked.Prefix},
		IPAddress:    ip,
	})

	s.invalidateKeyCache(ctx, revoked.Prefix)

	resp := toAPIKeyResponse(revoked)
	return &resp, nil
}

func (s *Service) RotateAPIKey(ctx context.Context, orgID, serviceID, keyID, userID, ip string) (*CreateAPIKeyResponse, error) {
	if err := s.requireDeveloper(ctx, orgID, userID); err != nil {
		return nil, err
	}

	if _, err := s.repo.GetServiceByID(ctx, orgID, serviceID); err != nil {
		return nil, err
	}

	oldKey, err := s.repo.GetAPIKeyByID(ctx, orgID, serviceID, keyID)
	if err != nil {
		return nil, err
	}
	if oldKey.RevokedAt != nil {
		return nil, ErrAPIKeyAlreadyRevoked
	}

	raw, prefix, err := auth.GenerateAPIKey()
	if err != nil {
		return nil, err
	}

	hash, err := auth.HashAPIKey(raw)
	if err != nil {
		return nil, err
	}

	stored, err := s.repo.RotateAPIKey(ctx, oldKey, &APIKey{
		ServiceID: serviceID,
		OrgID:     orgID,
		Prefix:    prefix,
		KeyHash:   hash,
		Label:     oldKey.Label,
		CreatedBy: userID,
	})
	if err != nil {
		return nil, err
	}

	s.writeAudit(ctx, audit.Event{
		OrgID:        orgID,
		ActorID:      userID,
		Action:       "api_key.rotated",
		ResourceType: "api_key",
		ResourceID:   stored.ID,
		Metadata: map[string]any{
			"service_id":      serviceID,
			"prefix":          stored.Prefix,
			"rotated_from_id": oldKey.ID,
		},
		IPAddress: ip,
	})

	s.invalidateKeyCache(ctx, oldKey.Prefix)

	return &CreateAPIKeyResponse{
		ID:        stored.ID,
		ServiceID: stored.ServiceID,
		Prefix:    stored.Prefix,
		APIKey:    raw,
		Label:     stored.Label,
		CreatedAt: stored.CreatedAt,
		CreatedBy: stored.CreatedBy,
	}, nil
}

func (s *Service) requireMember(ctx context.Context, orgID, userID string) (string, error) {
	role, err := s.orgAccess.GetOrgMemberRole(ctx, orgID, userID)
	if err != nil {
		if errors.Is(err, org.ErrNotOrgMember) {
			return "", ErrNotOrgMember
		}
		return "", err
	}
	return role, nil
}

func (s *Service) requireDeveloper(ctx context.Context, orgID, userID string) error {
	role, err := s.requireMember(ctx, orgID, userID)
	if err != nil {
		return err
	}
	if !IsDeveloperRole(role) {
		return ErrInsufficientPermissions
	}
	return nil
}

func (s *Service) writeAudit(ctx context.Context, event audit.Event) {
	if s.audit == nil {
		return
	}
	_ = s.audit.Write(ctx, event)
}

func (s *Service) invalidateKeyCache(ctx context.Context, prefix string) {
	if s.keyInvalidator == nil {
		return
	}
	_ = s.keyInvalidator.Invalidate(ctx, prefix)
}

func normalizeServiceName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", ErrInvalidServiceName
	}
	if len(trimmed) > maxServiceNameLen {
		return "", ErrServiceNameTooLong
	}
	return trimmed, nil
}
