package ingest

import (
	"context"

	"loglens/internal/auth"
	appsvc "loglens/internal/service"
)

type KeyLookup struct {
	repo appsvc.Repository
}

func NewKeyLookup(repo appsvc.Repository) *KeyLookup {
	return &KeyLookup{repo: repo}
}

func (l *KeyLookup) GetActiveAPIKeyByPrefix(ctx context.Context, prefix string) (*auth.APIKeyRecord, error) {
	key, err := l.repo.GetActiveAPIKeyByPrefix(ctx, prefix)
	if err != nil {
		return nil, err
	}

	return &auth.APIKeyRecord{
		ID:        key.ID,
		ServiceID: key.ServiceID,
		OrgID:     key.OrgID,
		Prefix:    key.Prefix,
		KeyHash:   key.KeyHash,
		Revoked:   key.RevokedAt != nil,
	}, nil
}
