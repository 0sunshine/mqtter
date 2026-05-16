package service

import (
	"context"
	"strings"
	"time"

	"mqtter/internal/domain"
	"mqtter/internal/ports"
)

type AuthService struct {
	repo   ports.AuthRepository
	hasher ports.PasswordHasher
	ids    ports.IDGenerator
	clock  ports.Clock
	ttl    time.Duration
}

func NewAuthService(repo ports.AuthRepository, hasher ports.PasswordHasher, ids ports.IDGenerator, clock ports.Clock, ttl time.Duration) *AuthService {
	if hasher == nil {
		hasher = BcryptHasher{}
	}
	if ids == nil {
		ids = RandomIDGenerator{}
	}
	if clock == nil {
		clock = SystemClock{}
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &AuthService{repo: repo, hasher: hasher, ids: ids, clock: clock, ttl: ttl}
}

func (s *AuthService) Login(ctx context.Context, username, password string) (domain.AdminUserDTO, string, time.Time, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return domain.AdminUserDTO{}, "", time.Time{}, domain.InvalidInput("invalid_credentials", "username and password are required")
	}
	user, err := s.repo.FindAdminByUsername(ctx, username)
	if err != nil || user.Disabled {
		return domain.AdminUserDTO{}, "", time.Time{}, domain.InvalidInput("invalid_credentials", "username or password is incorrect")
	}
	if err := s.hasher.Compare(user.PasswordHash, password); err != nil {
		return domain.AdminUserDTO{}, "", time.Time{}, domain.InvalidInput("invalid_credentials", "username or password is incorrect")
	}
	token := s.ids.NewID()
	expiresAt := s.clock.Now().Add(s.ttl)
	if err := s.repo.CreateSession(ctx, domain.AdminSession{Token: token, UserID: user.ID, ExpiresAt: expiresAt}); err != nil {
		return domain.AdminUserDTO{}, "", time.Time{}, err
	}
	return domain.AdminUserDTO{ID: user.ID, Username: user.Username, Role: user.Role}, token, expiresAt, nil
}

func (s *AuthService) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.repo.DeleteSession(ctx, token)
}

func (s *AuthService) ValidateSession(ctx context.Context, token string) (domain.AdminUserDTO, error) {
	if token == "" {
		return domain.AdminUserDTO{}, domain.InvalidInput("unauthorized", "authentication required")
	}
	session, err := s.repo.FindSession(ctx, token, s.clock.Now())
	if err != nil {
		return domain.AdminUserDTO{}, domain.InvalidInput("unauthorized", "authentication required")
	}
	return s.repo.GetAdminByID(ctx, session.UserID)
}

func (s *AuthService) BootstrapAdmin(ctx context.Context, username, password string) error {
	if strings.TrimSpace(username) == "" || password == "" {
		return nil
	}
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return err
	}
	return s.repo.BootstrapAdmin(ctx, s.ids.NewID(), username, hash, s.clock.Now())
}
