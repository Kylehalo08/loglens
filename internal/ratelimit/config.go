package ratelimit

import (
	"log"
	"os"
	"strconv"
)

type Config struct {
	Enabled              bool
	IngestPerOrgHour     int
	IngestPerOrgDay      int
	AuthPerIPMinute        int
	APIPerIPMinute         int
	APIPerUserMinute       int
	IngestRequestsPerIPMinute int
	AIRequestsPerOrgDay   int
}

func LoadConfig() Config {
	cfg := Config{
		Enabled:          envBool("RATE_LIMIT_ENABLED", true),
		IngestPerOrgHour: envInt("INGEST_LOGS_PER_ORG_HOUR", 500),
		IngestPerOrgDay:  envInt("INGEST_LOGS_PER_ORG_DAY", 5000),
		AuthPerIPMinute:           envInt("AUTH_REQUESTS_PER_IP_MINUTE", 15),
		APIPerIPMinute:            envInt("API_REQUESTS_PER_IP_MINUTE", 120),
		APIPerUserMinute:          envInt("API_REQUESTS_PER_USER_MINUTE", 300),
		IngestRequestsPerIPMinute: envInt("INGEST_REQUESTS_PER_IP_MINUTE", 60),
		AIRequestsPerOrgDay:       envInt("AI_REQUESTS_PER_ORG_DAY", 10),
	}

	if cfg.Enabled {
		log.Printf("rate limits: ingest %d/hr %d/day per org, ingest %d/min per IP, auth %d/min per IP, api %d/min per IP, %d/min per user, ai %d/day per org",
			cfg.IngestPerOrgHour, cfg.IngestPerOrgDay, cfg.IngestRequestsPerIPMinute, cfg.AuthPerIPMinute, cfg.APIPerIPMinute, cfg.APIPerUserMinute, cfg.AIRequestsPerOrgDay)
	}

	return cfg
}

func envInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}
