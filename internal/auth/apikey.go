package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	apiKeyPrefixLen = 8
	apiKeySecretLen = 16
	apiKeyCost      = 12
	apiKeyChars     = "abcdefghijklmnopqrstuvwxyz0123456789"
	apiKeyScheme    = "ll_"
)

var (
	ErrInvalidAPIKeyFormat = errors.New("invalid api key format")
	ErrInvalidAPIKey       = errors.New("invalid api key")
)

// GenerateAPIKey creates a raw API key and its lookup prefix.
// Format: ll_<8-char-prefix>_<32-char-hex-secret>
func GenerateAPIKey() (raw string, prefix string, err error) {
	prefixPart, err := randomString(apiKeyPrefixLen)
	if err != nil {
		return "", "", err
	}

	secretBytes := make([]byte, apiKeySecretLen)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", "", err
	}

	prefix = apiKeyScheme + prefixPart
	raw = fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(secretBytes))
	return raw, prefix, nil
}

func HashAPIKey(raw string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(raw), apiKeyCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func ValidateAPIKey(raw, hash string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(raw)); err != nil {
		return ErrInvalidAPIKey
	}
	return nil
}

func ExtractAPIKeyPrefix(raw string) (string, error) {
	if !strings.HasPrefix(raw, apiKeyScheme) {
		return "", ErrInvalidAPIKeyFormat
	}

	rest := strings.TrimPrefix(raw, apiKeyScheme)
	underscore := strings.Index(rest, "_")
	if underscore != apiKeyPrefixLen {
		return "", ErrInvalidAPIKeyFormat
	}

	return apiKeyScheme + rest[:underscore], nil
}

func randomString(length int) (string, error) {
	result := make([]byte, length)
	max := big.NewInt(int64(len(apiKeyChars)))

	for i := range result {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		result[i] = apiKeyChars[n.Int64()]
	}

	return string(result), nil
}
