package org

import "time"

const (
	RoleOwner     = "owner"
	RoleAdmin     = "admin"
	RoleDeveloper = "developer"
	RoleViewer    = "viewer"
)

var inviteRoles = map[string]struct{}{
	RoleAdmin:     {},
	RoleDeveloper: {},
	RoleViewer:    {},
}

type Organization struct {
	ID        string
	Name      string
	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type OrgMember struct {
	OrgID    string
	UserID   string
	Email    string
	Role     string
	JoinedAt time.Time
}

type OrgInvite struct {
	ID         string
	OrgID      string
	InvitedBy  string
	Email      string
	Role       string
	TokenHash  string
	ExpiresAt  time.Time
	AcceptedAt *time.Time
	CreatedAt  time.Time
}

type OrgInviteCode struct {
	ID          string
	OrgID       string
	Code        string
	CreatedBy   string
	DefaultRole string
	IsActive    bool
	CreatedAt   time.Time
}

type InviteTokenPayload struct {
	InviteID string `json:"invite_id"`
	OrgID    string `json:"org_id"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

type CreateOrgResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	Role      string    `json:"role"`
}

type OrgSummary struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedBy string    `json:"created_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Role      string    `json:"role"`
}

type OrgDetailResponse struct {
	ID            string           `json:"id"`
	Name          string           `json:"name"`
	CreatedAt     time.Time        `json:"created_at"`
	Members       []MemberResponse `json:"members"`
	ServicesCount int              `json:"services_count"`
}

type MemberResponse struct {
	UserID   string    `json:"user_id"`
	Email    string    `json:"email"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

type SendInviteResponse struct {
	InviteID  string    `json:"invite_id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"expires_at"`
	Token     string    `json:"token"`
}

type JoinResponse struct {
	OrgID    string    `json:"org_id"`
	OrgName  string    `json:"org_name"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

type InviteCodeResponse struct {
	Code        string `json:"code"`
	OrgID       string `json:"org_id"`
	DefaultRole string `json:"default_role"`
	IsActive    bool   `json:"is_active"`
}

func IsValidInviteRole(role string) bool {
	_, ok := inviteRoles[role]
	return ok
}

func IsAdminRole(role string) bool {
	return role == RoleOwner || role == RoleAdmin
}
