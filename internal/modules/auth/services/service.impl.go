package services

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"

	"github.com/open-suite/boilerplate-golang/internal/entities"
	"github.com/open-suite/boilerplate-golang/internal/modules/auth/dto"
	"github.com/open-suite/boilerplate-golang/internal/modules/auth/repositories"
	"github.com/open-suite/boilerplate-golang/internal/platform/config"
	"github.com/open-suite/boilerplate-golang/internal/platform/crypto"
	"github.com/open-suite/boilerplate-golang/internal/platform/google"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
	"github.com/open-suite/boilerplate-golang/internal/shared/apperror"
	"github.com/open-suite/boilerplate-golang/internal/shared/httpx"
	"github.com/open-suite/boilerplate-golang/internal/shared/requestctx"
	"github.com/open-suite/boilerplate-golang/internal/shared/validation"
)

const (
	stateTTL = 10 * time.Minute
	// A session is only rewritten when it has aged this much, which keeps the
	// sliding expiry from turning every request into a write.
	slideThreshold = time.Hour
)

type AuthServiceImpl struct {
	repo   repositories.AuthRepository
	google *google.Client
	keys   *crypto.Keys
	cfg    config.Config
	log    *logger.LayerLogger
}

func NewAuthService(repo repositories.AuthRepository, googleClient *google.Client, keys *crypto.Keys, cfg config.Config, appLogger *logger.Logger) AuthService {
	return &AuthServiceImpl{
		repo:   repo,
		google: googleClient,
		keys:   keys,
		cfg:    cfg,
		log:    appLogger.Layer("service.auth"),
	}
}

// statePayload is what the short lived, signed cookie carries across the redirect.
type statePayload struct {
	State    string `json:"s"`
	Verifier string `json:"v"`
	Next     string `json:"n"`
	Expires  int64  `json:"e"`
}

func (s *AuthServiceImpl) StartLogin(ctx context.Context, next string) (*LoginStart, error) {
	end := s.log.Start(ctx, "StartLogin")

	if !s.google.Configured() {
		end(google.ErrNotConfigured)
		return nil, apperror.Unavailable("google_not_configured")
	}

	state, err := crypto.RandomToken(24)
	if err != nil {
		end(err)
		return nil, err
	}
	verifier := oauth2.GenerateVerifier()

	payload, err := json.Marshal(statePayload{
		State:    state,
		Verifier: verifier,
		Next:     httpx.SafeNext(next),
		Expires:  time.Now().Add(stateTTL).Unix(),
	})
	if err != nil {
		end(err)
		return nil, err
	}

	redirect := s.google.LoginConfig().AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))

	end(nil)
	return &LoginStart{RedirectURL: redirect, StateCookie: s.keys.Sign(payload)}, nil
}

func (s *AuthServiceImpl) FinishLogin(ctx context.Context, code string, state string, stateCookie string) (*LoginResult, error) {
	end := s.log.Start(ctx, "FinishLogin")

	raw, ok := s.keys.Verify(stateCookie)
	if !ok {
		end(nil)
		return nil, apperror.BadRequest("invalid_oauth_state")
	}

	var payload statePayload
	if err := json.Unmarshal(raw, &payload); err != nil || payload.State == "" || payload.State != state || time.Now().Unix() > payload.Expires {
		end(nil)
		return nil, apperror.BadRequest("invalid_oauth_state")
	}
	if code == "" {
		end(nil)
		return nil, apperror.BadRequest("invalid_oauth_state")
	}

	token, err := s.google.LoginConfig().Exchange(ctx, code, oauth2.VerifierOption(payload.Verifier))
	if err != nil {
		end(err)
		return nil, apperror.BadGateway("oauth_exchange_failed", err)
	}

	idToken, _ := token.Extra("id_token").(string)
	claims, err := s.google.VerifyIDToken(ctx, idToken)
	if err != nil {
		end(err)
		return nil, apperror.BadGateway("oauth_exchange_failed", err)
	}
	if !claims.EmailVerified {
		end(nil)
		return nil, apperror.Forbidden("email_not_verified")
	}

	var avatar *string
	if claims.Picture != "" {
		avatar = &claims.Picture
	}
	name := claims.Name
	if name == "" {
		name = strings.Split(claims.Email, "@")[0]
	}

	user, err := s.repo.UpsertUser(ctx, claims.Subject, strings.ToLower(claims.Email), name, avatar)
	if err != nil {
		end(err)
		return nil, err
	}

	result, err := s.startSession(ctx, user.ID)
	if err != nil {
		end(err)
		return nil, err
	}
	result.Next = payload.Next

	end(nil, "user_id", user.ID)
	return result, nil
}

// DevLogin signs in without Google. It exists for local API testing and is
// only routed when DEV_LOGIN_ENABLED=true.
func (s *AuthServiceImpl) DevLogin(ctx context.Context, request dto.DevLoginRequest) (*LoginResult, error) {
	end := s.log.Start(ctx, "DevLogin")

	email := strings.ToLower(strings.TrimSpace(request.Email))
	name := strings.TrimSpace(request.Name)

	validator := validation.New()
	validator.Check(strings.Contains(email, "@") && len(email) <= 254, "email", "must be a valid email address")
	validator.Check(len(name) <= 120, "name", "must be at most 120 characters")
	if err := validator.Err(); err != nil {
		end(err)
		return nil, err
	}
	if name == "" {
		name = strings.Split(email, "@")[0]
	}

	user, err := s.repo.UpsertUser(ctx, "dev:"+email, email, name, nil)
	if err != nil {
		end(err)
		return nil, err
	}

	result, err := s.startSession(ctx, user.ID)
	if err != nil {
		end(err)
		return nil, err
	}
	result.Next = "/"
	result.User = &dto.UserResponse{ID: user.ID, Email: user.Email, Name: user.Name, AvatarURL: user.AvatarURL}

	end(nil, "user_id", user.ID)
	return result, nil
}

func (s *AuthServiceImpl) startSession(ctx context.Context, userID uuid.UUID) (*LoginResult, error) {
	token, err := crypto.RandomToken(32)
	if err != nil {
		return nil, err
	}

	expires := time.Now().Add(s.cfg.Auth.SessionTTL)
	if err := s.repo.CreateSession(ctx, userID, crypto.HashToken(token), expires); err != nil {
		return nil, err
	}

	return &LoginResult{Token: token, Expires: expires}, nil
}

func (s *AuthServiceImpl) Resolve(ctx context.Context, token string) (*requestctx.User, *time.Time, error) {
	record, err := s.repo.FindSession(ctx, crypto.HashToken(token))
	if err != nil {
		return nil, nil, err
	}

	var extended *time.Time
	ttl := s.cfg.Auth.SessionTTL
	if time.Until(record.ExpiresAt) < ttl-slideThreshold {
		expires := time.Now().Add(ttl)
		if err := s.repo.ExtendSession(ctx, record.SessionID, expires); err != nil {
			s.log.Warn(ctx, "session.extend.failed", "error", err.Error())
		} else {
			extended = &expires
		}
	}

	return &requestctx.User{
		ID:        record.User.ID,
		Email:     record.User.Email,
		Name:      record.User.Name,
		AvatarURL: record.User.AvatarURL,
	}, extended, nil
}

func (s *AuthServiceImpl) Logout(ctx context.Context, token string) error {
	end := s.log.Start(ctx, "Logout")
	err := s.repo.DeleteSession(ctx, crypto.HashToken(token))
	end(err)
	return err
}

func (s *AuthServiceImpl) Me(ctx context.Context, userID uuid.UUID) (*dto.MeResponse, error) {
	end := s.log.Start(ctx, "Me")

	user, err := s.repo.FindUser(ctx, userID)
	if err != nil {
		end(err)
		return nil, err
	}

	connection, err := s.repo.FindCalendarConnection(ctx, userID)
	if err != nil {
		end(err)
		return nil, err
	}

	calendar := dto.CalendarStatus{Status: "not_connected"}
	if connection != nil {
		email := connection.GoogleEmail
		calendar = dto.CalendarStatus{
			Connected:   connection.Status == entities.CalendarStatusActive,
			Status:      connection.Status,
			GoogleEmail: &email,
		}
	}

	end(nil)
	return &dto.MeResponse{
		User:     dto.UserResponse{ID: user.ID, Email: user.Email, Name: user.Name, AvatarURL: user.AvatarURL},
		Calendar: calendar,
	}, nil
}

func (s *AuthServiceImpl) PurgeExpired(ctx context.Context) (int64, error) {
	return s.repo.DeleteExpiredSessions(ctx)
}
