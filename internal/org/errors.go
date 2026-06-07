package org

import "errors"

var (
	ErrOrgNotFound             = errors.New("organization not found")
	ErrNotOrgMember            = errors.New("not an organization member")
	ErrInsufficientPermissions = errors.New("insufficient permissions")
	ErrInvalidOrgName          = errors.New("organization name is required")
	ErrInvalidInviteRole       = errors.New("invalid invite role")
	ErrInvalidEmail            = errors.New("invalid email format")
	ErrInviteNotFound          = errors.New("invite not found")
	ErrInviteExpired           = errors.New("invite expired")
	ErrInviteAlreadyAccepted   = errors.New("invite already accepted")
	ErrAlreadyOrgMember        = errors.New("already an organization member")
	ErrInviteCodeNotFound      = errors.New("invite code not found")
	ErrInviteCodeInactive      = errors.New("invite code is inactive")
)
