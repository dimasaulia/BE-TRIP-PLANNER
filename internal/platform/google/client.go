// Package google wraps Google OAuth2/OIDC and the Calendar REST API.
package google

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	googleoauth "golang.org/x/oauth2/google"

	"github.com/open-suite/boilerplate-golang/internal/platform/config"
)

const (
	issuer = "https://accounts.google.com"

	// ScopeCalendar only reaches calendars this app created. Verify against
	// Google's current scope list before shipping (see TRD section 9).
	ScopeCalendar = "https://www.googleapis.com/auth/calendar.app.created"
)

var ErrNotConfigured = errors.New("google oauth is not configured")

type Claims struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	Picture       string
}

type Client struct {
	cfg config.GoogleConfig

	mu       sync.Mutex
	provider *oidc.Provider
}

func New(cfg config.Config) *Client {
	return &Client{cfg: cfg.Google}
}

func (c *Client) Configured() bool {
	return c.cfg.ClientID != "" && c.cfg.ClientSecret != ""
}

// LoginConfig asks only for identity; Calendar access is requested separately.
func (c *Client) LoginConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     c.cfg.ClientID,
		ClientSecret: c.cfg.ClientSecret,
		Endpoint:     googleoauth.Endpoint,
		RedirectURL:  c.cfg.RedirectURL,
		Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
	}
}

func (c *Client) CalendarConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     c.cfg.ClientID,
		ClientSecret: c.cfg.ClientSecret,
		Endpoint:     googleoauth.Endpoint,
		RedirectURL:  c.cfg.CalendarRedirectURL,
		Scopes:       []string{oidc.ScopeOpenID, "email", ScopeCalendar},
	}
}

// VerifyIDToken checks signature, issuer, audience and expiry.
func (c *Client) VerifyIDToken(ctx context.Context, raw string) (*Claims, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}

	provider, err := c.oidcProvider(ctx)
	if err != nil {
		return nil, err
	}

	token, err := provider.Verifier(&oidc.Config{ClientID: c.cfg.ClientID}).Verify(ctx, raw)
	if err != nil {
		return nil, fmt.Errorf("verify id token: %w", err)
	}

	var body struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := token.Claims(&body); err != nil {
		return nil, err
	}

	return &Claims{
		Subject:       token.Subject,
		Email:         body.Email,
		EmailVerified: body.EmailVerified,
		Name:          body.Name,
		Picture:       body.Picture,
	}, nil
}

// oidcProvider discovers lazily so the server can boot without network access.
func (c *Client) oidcProvider(ctx context.Context) (*oidc.Provider, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.provider != nil {
		return c.provider, nil
	}

	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("discover google oidc: %w", err)
	}

	c.provider = provider
	return provider, nil
}
