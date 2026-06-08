package telemetry

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

const (
	DefaultSearchLimit = 100
	MaxSearchLimit     = 1000
	MaxSearchRangeDays = 30
)

var (
	ErrInvalidSearchParam = errors.New("invalid search parameter")
)

func ParseSearchFilters(c echo.Context, orgID string) (SearchFilters, error) {
	if _, err := uuid.Parse(orgID); err != nil {
		return SearchFilters{}, fmt.Errorf("%w: invalid organization id", ErrInvalidSearchParam)
	}

	filters := SearchFilters{
		OrgID: orgID,
		Page:  1,
		Limit: DefaultSearchLimit,
	}

	if raw := strings.TrimSpace(c.QueryParam("service_id")); raw != "" {
		ids, err := parseUUIDList(raw)
		if err != nil {
			return SearchFilters{}, fmt.Errorf("%w: %v", ErrInvalidSearchParam, err)
		}
		filters.ServiceIDs = ids
	}

	if raw := strings.TrimSpace(c.QueryParam("severity")); raw != "" {
		severities, err := parseSeverityList(raw)
		if err != nil {
			return SearchFilters{}, fmt.Errorf("%w: %v", ErrInvalidSearchParam, err)
		}
		filters.Severities = severities
	}

	if raw := strings.TrimSpace(c.QueryParam("from")); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return SearchFilters{}, fmt.Errorf("%w: from must be RFC3339", ErrInvalidSearchParam)
		}
		filters.From = &t
	}

	if raw := strings.TrimSpace(c.QueryParam("to")); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return SearchFilters{}, fmt.Errorf("%w: to must be RFC3339", ErrInvalidSearchParam)
		}
		filters.To = &t
	}

	if filters.From != nil && filters.To != nil {
		if filters.From.After(*filters.To) {
			return SearchFilters{}, fmt.Errorf("%w: from must be before to", ErrInvalidSearchParam)
		}
		if filters.To.Sub(*filters.From) > MaxSearchRangeDays*24*time.Hour {
			return SearchFilters{}, fmt.Errorf("%w: time range cannot exceed %d days", ErrInvalidSearchParam, MaxSearchRangeDays)
		}
	}

	filters.Query = strings.TrimSpace(c.QueryParam("q"))

	if raw := strings.TrimSpace(c.QueryParam("page")); raw != "" {
		page, err := strconv.Atoi(raw)
		if err != nil || page < 1 {
			return SearchFilters{}, fmt.Errorf("%w: page must be a positive integer", ErrInvalidSearchParam)
		}
		filters.Page = page
	}

	if raw := strings.TrimSpace(c.QueryParam("limit")); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 {
			return SearchFilters{}, fmt.Errorf("%w: limit must be a positive integer", ErrInvalidSearchParam)
		}
		if limit > MaxSearchLimit {
			return SearchFilters{}, fmt.Errorf("%w: limit cannot exceed %d", ErrInvalidSearchParam, MaxSearchLimit)
		}
		filters.Limit = limit
	}

	return filters, nil
}

func parseUUIDList(raw string) ([]string, error) {
	parts := strings.Split(raw, ",")
	ids := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, err := uuid.Parse(part); err != nil {
			return nil, fmt.Errorf("invalid service_id %q", part)
		}
		ids = append(ids, part)
	}
	if len(ids) == 0 {
		return nil, errors.New("service_id cannot be empty")
	}
	return ids, nil
}

func parseSeverityList(raw string) ([]string, error) {
	parts := strings.Split(raw, ",")
	severities := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(strings.ToUpper(part))
		if part == "" {
			continue
		}
		if !IsValidSeverity(part) {
			return nil, fmt.Errorf("invalid severity %q", part)
		}
		severities = append(severities, part)
	}
	if len(severities) == 0 {
		return nil, errors.New("severity cannot be empty")
	}
	return severities, nil
}
