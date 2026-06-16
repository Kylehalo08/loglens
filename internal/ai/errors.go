package ai

import "errors"

var (
	ErrRateLimited     = errors.New("ai rate limited")
	ErrBadResponse     = errors.New("ai bad response")
	ErrProviderFailure = errors.New("ai provider failure")
)

