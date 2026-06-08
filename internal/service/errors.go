package service

import "errors"

var (
	ErrServiceNotFound       = errors.New("service not found")
	ErrAPIKeyNotFound        = errors.New("api key not found")
	ErrAPIKeyAlreadyRevoked  = errors.New("api key already revoked")
	ErrInvalidServiceName    = errors.New("service name is required")
	ErrServiceNameTooLong    = errors.New("service name must be at most 255 characters")
	ErrDuplicateServiceName  = errors.New("service name already exists in this organization")
	ErrNotOrgMember          = errors.New("not an organization member")
	ErrInsufficientPermissions = errors.New("insufficient permissions")
)
