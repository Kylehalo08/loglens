package user

import (
	"context"
	"errors"
	"net/mail"
	"os"
	"strconv"
	"time"
	"unicode/utf8"

	"loglens/internal/auth"
	"loglens/internal/db"

	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultRole         = "user"
	refreshTokenKeyPref = "refresh:"
)

// RefreshTokenCache defines optional fast-path storage for refresh tokens.
type RefreshTokenCache interface {
	IsAvailable() bool
	Set(ctx context.Context, tokenHash, userID string, ttl time.Duration) error
	Get(ctx context.Context, tokenHash string) (string, error)
	Delete(ctx context.Context, tokenHash string) error
}

type Service struct {
	repo              Repository
	tokens            auth.TokenManager
	cache             RefreshTokenCache
	refreshTokenTTL   time.Duration
}

func NewService(repo Repository, tokens auth.TokenManager, cache RefreshTokenCache) (*Service, error) {
	days := 7
	if raw := os.Getenv("REFRESH_TOKEN_EXPIRY_DAYS"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return nil, err
		}
		days = parsed
	}

	return &Service{
		repo:            repo,
		tokens:          tokens,
		cache:           cache,
		refreshTokenTTL: time.Duration(days) * 24 * time.Hour,
	}, nil
}

func (s *Service) Register(ctx context.Context, email, password string) (*AuthTokens, error) {
	if err := validateCredentials(email, password); err != nil {
		return nil, err
	}

	if _, err := s.repo.GetUserByEmail(ctx, email); err == nil {
		return nil, ErrEmailTaken
	} else if !errors.Is(err, ErrUserNotFound) {
		return nil, err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return nil, err
	}

	userID, err := s.repo.CreateUser(ctx, email, string(passwordHash))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrEmailTaken
		}
		return nil, err
	}

	return s.issueTokenPair(ctx, userID)
}

func (s *Service) Login(ctx context.Context, email, password string) (*AuthTokens, error) {
	if err := validateCredentials(email, password); err != nil {
		return nil, err
	}

	user, err := s.repo.GetUserByEmail(ctx, email)
	if errors.Is(err, ErrUserNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	return s.issueTokenPair(ctx, user.ID)
}

func (s *Service) Refresh(ctx context.Context, rawRefreshToken string) (*AuthTokens, error) {
	if rawRefreshToken == "" {
		return nil, ErrInvalidRefreshToken
	}

	tokenHash := s.tokens.HashToken(rawRefreshToken)
	userID, err := s.lookupRefreshToken(ctx, tokenHash)
	if err != nil {
		return nil, err
	}

	if err := s.revokeRefreshToken(ctx, tokenHash); err != nil {
		return nil, err
	}

	accessToken, err := s.tokens.GenerateAccessToken(userID, defaultRole)
	if err != nil {
		return nil, err
	}

	rawToken, hashedToken, err := s.tokens.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	if err := s.persistRefreshToken(ctx, userID, hashedToken); err != nil {
		return nil, err
	}

	return &AuthTokens{
		AccessToken:  accessToken,
		RefreshToken: rawToken,
	}, nil
}

func (s *Service) Logout(ctx context.Context, rawRefreshToken string) error {
	if rawRefreshToken == "" {
		return ErrInvalidRefreshToken
	}

	tokenHash := s.tokens.HashToken(rawRefreshToken)
	return s.revokeRefreshToken(ctx, tokenHash)
}

func (s *Service) issueTokenPair(ctx context.Context, userID string) (*AuthTokens, error) {
	accessToken, err := s.tokens.GenerateAccessToken(userID, defaultRole)
	if err != nil {
		return nil, err
	}

	rawToken, hashedToken, err := s.tokens.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	if err := s.persistRefreshToken(ctx, userID, hashedToken); err != nil {
		return nil, err
	}

	return &AuthTokens{
		AccessToken:  accessToken,
		RefreshToken: rawToken,
	}, nil
}

func (s *Service) persistRefreshToken(ctx context.Context, userID, tokenHash string) error {
	expiresAt := time.Now().Add(s.refreshTokenTTL)

	s.storeRefreshTokenInCache(ctx, tokenHash, userID)
	return s.repo.StoreRefreshToken(ctx, userID, tokenHash, expiresAt)
}

func (s *Service) lookupRefreshToken(ctx context.Context, tokenHash string) (string, error) {
	if s.cache != nil && s.cache.IsAvailable() {
		userID, err := s.cache.Get(ctx, tokenHash)
		if err == nil && userID != "" {
			return userID, nil
		}
	}

	stored, err := s.repo.GetRefreshToken(ctx, tokenHash)
	if err != nil {
		return "", err
	}

	if time.Now().After(stored.ExpiresAt) {
		_ = s.revokeRefreshToken(ctx, tokenHash)
		return "", ErrExpiredRefreshToken
	}

	return stored.UserID, nil
}

func (s *Service) revokeRefreshToken(ctx context.Context, tokenHash string) error {
	if s.cache != nil && s.cache.IsAvailable() {
		_ = s.cache.Delete(ctx, tokenHash)
	}
	return s.repo.DeleteRefreshToken(ctx, tokenHash)
}

func (s *Service) storeRefreshTokenInCache(ctx context.Context, tokenHash, userID string) {
	if s.cache == nil || !s.cache.IsAvailable() {
		return
	}
	_ = s.cache.Set(ctx, tokenHash, userID, s.refreshTokenTTL)
}

func validateCredentials(email, password string) error {
	if _, err := mail.ParseAddress(email); err != nil {
		return ErrInvalidEmail
	}
	if utf8.RuneCountInString(password) < 8 {
		return ErrInvalidPassword
	}
	return nil
}

// RedisRefreshCache adapts db.RedisStore to RefreshTokenCache (adapter pattern).
type RedisRefreshCache struct {
	store *db.RedisStore
}

func NewRedisRefreshCache(store *db.RedisStore) *RedisRefreshCache {
	return &RedisRefreshCache{store: store}
}

func (c *RedisRefreshCache) IsAvailable() bool {
	return c.store != nil && c.store.IsAvailable()
}

func (c *RedisRefreshCache) Set(ctx context.Context, tokenHash, userID string, ttl time.Duration) error {
	return c.store.Client().Set(ctx, refreshTokenKeyPref+tokenHash, userID, ttl).Err()
}

func (c *RedisRefreshCache) Get(ctx context.Context, tokenHash string) (string, error) {
	return c.store.Client().Get(ctx, refreshTokenKeyPref+tokenHash).Result()
}

func (c *RedisRefreshCache) Delete(ctx context.Context, tokenHash string) error {
	return c.store.Client().Del(ctx, refreshTokenKeyPref+tokenHash).Err()
}
