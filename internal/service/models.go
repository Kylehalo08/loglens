package service

import "time"

type RegisteredService struct {
	ID          string
	OrgID       string
	Name        string
	Description string
	CreatedBy   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

type APIKey struct {
	ID            string
	ServiceID     string
	OrgID         string
	Prefix        string
	KeyHash       string
	Label         string
	CreatedBy     string
	CreatedAt     time.Time
	RevokedAt     *time.Time
	LastUsedAt    *time.Time
	RotatedFromID string
}

type ServiceResponse struct {
	ID                  string    `json:"id"`
	OrgID               string    `json:"org_id"`
	Name                string    `json:"name"`
	Description         string    `json:"description,omitempty"`
	CreatedBy           string    `json:"created_by"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
	ActiveAPIKeysCount  *int      `json:"active_api_keys_count,omitempty"`
}

type CreateAPIKeyResponse struct {
	ID        string    `json:"id"`
	ServiceID string    `json:"service_id"`
	Prefix    string    `json:"prefix"`
	APIKey    string    `json:"api_key"`
	Label     string    `json:"label,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by"`
}

type APIKeyResponse struct {
	ID         string     `json:"id"`
	ServiceID  string     `json:"service_id"`
	Prefix     string     `json:"prefix"`
	Label      string     `json:"label,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	CreatedBy  string     `json:"created_by"`
	RevokedAt  *time.Time `json:"revoked_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

func toServiceResponse(svc *RegisteredService, activeKeys *int) *ServiceResponse {
	resp := &ServiceResponse{
		ID:          svc.ID,
		OrgID:       svc.OrgID,
		Name:        svc.Name,
		Description: svc.Description,
		CreatedBy:   svc.CreatedBy,
		CreatedAt:   svc.CreatedAt,
		UpdatedAt:   svc.UpdatedAt,
	}
	if activeKeys != nil {
		resp.ActiveAPIKeysCount = activeKeys
	}
	return resp
}

func toAPIKeyResponse(key *APIKey) APIKeyResponse {
	return APIKeyResponse{
		ID:         key.ID,
		ServiceID:  key.ServiceID,
		Prefix:     key.Prefix,
		Label:      key.Label,
		CreatedAt:  key.CreatedAt,
		CreatedBy:  key.CreatedBy,
		RevokedAt:  key.RevokedAt,
		LastUsedAt: key.LastUsedAt,
	}
}
