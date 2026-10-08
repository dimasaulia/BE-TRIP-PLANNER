package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"

	"github.com/open-suite/boilerplate-golang/internal/entities"
	"github.com/open-suite/boilerplate-golang/internal/modules/calendar/dto"
	"github.com/open-suite/boilerplate-golang/internal/modules/calendar/repositories"
	"github.com/open-suite/boilerplate-golang/internal/platform/crypto"
	"github.com/open-suite/boilerplate-golang/internal/platform/database"
	"github.com/open-suite/boilerplate-golang/internal/platform/google"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
	"github.com/open-suite/boilerplate-golang/internal/shared/access"
	"github.com/open-suite/boilerplate-golang/internal/shared/apperror"
	"github.com/open-suite/boilerplate-golang/internal/shared/httpx"
)

const stateTTL = 10 * time.Minute

type CalendarServiceImpl struct {
	repo   repositories.CalendarRepository
	db     *database.Database
	access access.Service
	google *google.Client
	keys   *crypto.Keys
	log    *logger.LayerLogger
}

func NewCalendarService(
	repo repositories.CalendarRepository,
	db *database.Database,
	accessService access.Service,
	googleClient *google.Client,
	keys *crypto.Keys,
	appLogger *logger.Logger,
) CalendarService {
	return &CalendarServiceImpl{
		repo:   repo,
		db:     db,
		access: accessService,
		google: googleClient,
		keys:   keys,
		log:    appLogger.Layer("service.calendar"),
	}
}

// connectState is the signed cookie that survives the round trip to Google;
// it is bound to the user who started the flow.
type connectState struct {
	State    string `json:"s"`
	Verifier string `json:"v"`
	Next     string `json:"n"`
	UserID   string `json:"u"`
	Expires  int64  `json:"e"`
}

func (s *CalendarServiceImpl) StartConnect(ctx context.Context, userID uuid.UUID, next string) (*ConnectStart, error) {
	end := s.log.Start(ctx, "StartConnect")

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

	payload, err := json.Marshal(connectState{
		State:    state,
		Verifier: verifier,
		Next:     httpx.SafeNext(next),
		UserID:   userID.String(),
		Expires:  time.Now().Add(stateTTL).Unix(),
	})
	if err != nil {
		end(err)
		return nil, err
	}

	// offline + consent make Google return a refresh token every time, and
	// include_granted_scopes keeps the identity scopes granted at login.
	redirect := s.google.CalendarConfig().AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
		oauth2.SetAuthURLParam("include_granted_scopes", "true"),
		oauth2.S256ChallengeOption(verifier),
	)

	end(nil)
	return &ConnectStart{RedirectURL: redirect, StateCookie: s.keys.Sign(payload)}, nil
}

func (s *CalendarServiceImpl) FinishConnect(ctx context.Context, userID uuid.UUID, code string, state string, stateCookie string) (string, error) {
	end := s.log.Start(ctx, "FinishConnect")

	raw, ok := s.keys.Verify(stateCookie)
	if !ok {
		end(nil)
		return "", apperror.BadRequest("invalid_oauth_state")
	}

	var payload connectState
	if err := json.Unmarshal(raw, &payload); err != nil ||
		payload.State == "" || payload.State != state ||
		payload.UserID != userID.String() || time.Now().Unix() > payload.Expires || code == "" {
		end(nil)
		return "", apperror.BadRequest("invalid_oauth_state")
	}

	token, err := s.google.CalendarConfig().Exchange(ctx, code, oauth2.VerifierOption(payload.Verifier))
	if err != nil {
		end(err)
		return "", apperror.BadGateway("oauth_exchange_failed", err)
	}

	granted, _ := token.Extra("scope").(string)
	if !strings.Contains(granted, google.ScopeCalendar) {
		end(nil)
		return "", apperror.Forbidden("calendar_scope_denied")
	}
	if token.RefreshToken == "" {
		end(nil)
		return "", apperror.Unprocessable("refresh_token_missing")
	}

	idToken, _ := token.Extra("id_token").(string)
	claims, err := s.google.VerifyIDToken(ctx, idToken)
	if err != nil {
		end(err)
		return "", apperror.BadGateway("oauth_exchange_failed", err)
	}

	encrypted, err := s.keys.Encrypt([]byte(token.RefreshToken))
	if err != nil {
		end(err)
		if errors.Is(err, crypto.ErrKeyMissing) {
			return "", apperror.Unavailable("encryption_not_configured")
		}
		return "", err
	}

	if err := s.repo.UpsertConnection(ctx, userID, claims.Email, encrypted, granted); err != nil {
		end(err)
		return "", err
	}

	end(nil)
	return payload.Next, nil
}

func (s *CalendarServiceImpl) Disconnect(ctx context.Context, userID uuid.UUID) error {
	end := s.log.Start(ctx, "Disconnect")

	connection, err := s.repo.FindConnection(ctx, userID)
	if err != nil {
		end(err)
		return err
	}
	if connection == nil {
		end(nil)
		return nil
	}

	// Revoking is best effort; the local connection goes away either way.
	if refreshToken, err := s.keys.Decrypt(connection.RefreshTokenEnc); err == nil {
		if err := s.google.RevokeToken(ctx, string(refreshToken)); err != nil {
			s.log.Warn(ctx, "revoke.failed", "error", err.Error())
		}
	}

	err = s.db.InTx(ctx, func(ctx context.Context) error {
		return s.repo.DeleteConnection(ctx, userID)
	})
	end(err)
	return err
}

// activeAPI returns a Google API for the user, or the right 422 when there is none.
func (s *CalendarServiceImpl) activeAPI(ctx context.Context, userID uuid.UUID) (google.CalendarAPI, error) {
	connection, err := s.repo.FindConnection(ctx, userID)
	if err != nil {
		return nil, err
	}
	if connection == nil {
		return nil, apperror.Unprocessable("calendar_not_connected")
	}
	if connection.Status != entities.CalendarStatusActive {
		return nil, apperror.Unprocessable("calendar_reconnect_required")
	}

	refreshToken, err := s.keys.Decrypt(connection.RefreshTokenEnc)
	if err != nil {
		return nil, apperror.Unprocessable("calendar_reconnect_required").Wrap(err)
	}
	return s.google.Calendar(ctx, string(refreshToken)), nil
}

func (s *CalendarServiceImpl) status(ctx context.Context, sync *entities.CalendarSync) (*dto.SyncStatusResponse, error) {
	pending, err := s.repo.PendingJobs(ctx, sync.ID)
	if err != nil {
		return nil, err
	}

	return &dto.SyncStatusResponse{
		Enabled:      true,
		CalendarID:   sync.GoogleCalendarID,
		LastSyncedAt: sync.LastSyncedAt,
		LastError:    sync.LastError,
		PendingJobs:  pending,
	}, nil
}

// EnableSync is idempotent: a second call returns the existing sync.
func (s *CalendarServiceImpl) EnableSync(ctx context.Context, userID uuid.UUID, tripID uuid.UUID) (*dto.SyncStatusResponse, error) {
	end := s.log.Start(ctx, "EnableSync", "trip_id", tripID)

	if _, err := s.access.Require(ctx, tripID, userID, access.Viewer); err != nil {
		end(err)
		return nil, err
	}

	existing, err := s.repo.FindSync(ctx, tripID, userID)
	if err != nil {
		end(err)
		return nil, err
	}
	if existing != nil {
		status, err := s.status(ctx, existing)
		end(err)
		return status, err
	}

	trip, err := s.access.Trip(ctx, tripID)
	if err != nil {
		end(err)
		return nil, err
	}

	api, err := s.activeAPI(ctx, userID)
	if err != nil {
		end(err)
		return nil, err
	}

	calendarID, err := api.CreateCalendar(ctx, trip.Name, trip.Timezone)
	if err != nil {
		end(err)
		return nil, s.googleError(ctx, userID, err)
	}

	var created *entities.CalendarSync
	err = s.db.InTx(ctx, func(ctx context.Context) error {
		sync, err := s.repo.CreateSync(ctx, tripID, userID, calendarID)
		if err != nil {
			return err
		}
		created = sync
		return s.repo.EnqueueJob(ctx, sync.ID, uuid.Nil, entities.SyncOpFullResync, 0)
	})
	if err != nil {
		// Do not leave an orphan calendar behind in the user's Google account.
		if cleanupErr := api.DeleteCalendar(context.WithoutCancel(ctx), calendarID); cleanupErr != nil {
			s.log.Warn(ctx, "calendar.cleanup.failed", "error", cleanupErr.Error())
		}
		end(err)
		return nil, err
	}

	end(nil, "sync_id", created.ID)
	return s.status(ctx, created)
}

func (s *CalendarServiceImpl) DisableSync(ctx context.Context, userID uuid.UUID, tripID uuid.UUID, keepEvents bool) error {
	end := s.log.Start(ctx, "DisableSync", "trip_id", tripID, "keep_events", keepEvents)

	if _, err := s.access.Require(ctx, tripID, userID, access.Viewer); err != nil {
		end(err)
		return err
	}

	sync, err := s.repo.FindSync(ctx, tripID, userID)
	if err != nil {
		end(err)
		return err
	}
	if sync == nil {
		err := apperror.NotFound("calendar_sync_not_found")
		end(err)
		return err
	}

	// Deleting the calendar removes its events with it; with keep_events only
	// the link is cut and the calendar stays in the user's Google account.
	if !keepEvents {
		if api, err := s.activeAPI(ctx, userID); err == nil {
			if err := api.DeleteCalendar(ctx, sync.GoogleCalendarID); err != nil {
				var apiErr *google.APIError
				if !errors.As(err, &apiErr) || !apiErr.Gone() {
					s.log.Warn(ctx, "calendar.delete.failed", "error", err.Error())
				}
			}
		}
	}

	err = s.repo.DeleteSync(ctx, sync.ID)
	end(err)
	return err
}

func (s *CalendarServiceImpl) Resync(ctx context.Context, userID uuid.UUID, tripID uuid.UUID) (*dto.SyncStatusResponse, error) {
	end := s.log.Start(ctx, "Resync", "trip_id", tripID)

	if _, err := s.access.Require(ctx, tripID, userID, access.Viewer); err != nil {
		end(err)
		return nil, err
	}

	sync, err := s.repo.FindSync(ctx, tripID, userID)
	if err != nil {
		end(err)
		return nil, err
	}
	if sync == nil {
		err := apperror.NotFound("calendar_sync_not_found")
		end(err)
		return nil, err
	}

	if err := s.repo.EnqueueJob(ctx, sync.ID, uuid.Nil, entities.SyncOpFullResync, 0); err != nil {
		end(err)
		return nil, err
	}

	status, err := s.status(ctx, sync)
	end(err)
	return status, err
}

// googleError turns a Google failure into an API error; a dead grant also
// flips the connection to revoked so /me asks the user to reconnect.
func (s *CalendarServiceImpl) googleError(ctx context.Context, userID uuid.UUID, err error) error {
	if errors.Is(err, google.ErrInvalidGrant) {
		_ = s.repo.MarkRevoked(ctx, userID, "Google Calendar access was revoked; reconnect to resume syncing")
		return apperror.Unprocessable("calendar_reconnect_required")
	}
	return apperror.BadGateway("google_error", err)
}
