package services

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/open-suite/boilerplate-golang/internal/platform/config"
	"github.com/open-suite/boilerplate-golang/internal/platform/crypto"
	"github.com/open-suite/boilerplate-golang/internal/platform/google"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
	"github.com/open-suite/boilerplate-golang/internal/shared/apperror"
)

func newService(t *testing.T, clientID string) (*AuthServiceImpl, *crypto.Keys) {
	t.Helper()

	cfg := config.Config{
		Google: config.GoogleConfig{
			ClientID: clientID, ClientSecret: "secret",
			RedirectURL: "http://localhost:8080/api/v1/auth/google/callback",
		},
		Auth: config.AuthConfig{TokenEncKey: strings.Repeat("cd", 32), SessionTTL: time.Hour},
	}

	keys, err := crypto.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	appLogger, err := logger.New(logger.Config{Level: "error", LogDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}

	return NewAuthService(nil, google.New(cfg), keys, cfg, appLogger).(*AuthServiceImpl), keys
}

func TestStartLoginBuildsAPKCERedirect(t *testing.T) {
	service, keys := newService(t, "client-id")

	start, err := service.StartLogin(context.Background(), "/trips/abc")
	if err != nil {
		t.Fatal(err)
	}

	redirect, err := url.Parse(start.RedirectURL)
	if err != nil || redirect.Host != "accounts.google.com" {
		t.Fatalf("must redirect to Google, got %q", start.RedirectURL)
	}

	query := redirect.Query()
	if query.Get("code_challenge_method") != "S256" || query.Get("code_challenge") == "" {
		t.Errorf("PKCE S256 challenge is required: %v", query)
	}
	if query.Get("scope") != "openid email profile" {
		t.Errorf("login must only ask for identity scopes, got %q", query.Get("scope"))
	}
	if query.Get("client_id") != "client-id" || query.Get("response_type") != "code" ||
		query.Get("redirect_uri") != "http://localhost:8080/api/v1/auth/google/callback" {
		t.Errorf("unexpected query: %v", query)
	}

	// The cookie carries state + verifier + next, signed, and the state matches the URL.
	raw, ok := keys.Verify(start.StateCookie)
	if !ok {
		t.Fatal("state cookie must be signed")
	}
	var payload statePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.State != query.Get("state") || payload.Verifier == "" || payload.Next != "/trips/abc" {
		t.Errorf("unexpected state payload: %+v", payload)
	}
	if strings.Contains(start.RedirectURL, payload.Verifier) {
		t.Error("the PKCE verifier must never be sent to Google")
	}
}

func TestStartLoginNeutralisesOpenRedirects(t *testing.T) {
	service, keys := newService(t, "client-id")

	for _, next := range []string{"//evil.example", "https://evil.example", "/\\evil.example", "evil", ""} {
		start, err := service.StartLogin(context.Background(), next)
		if err != nil {
			t.Fatal(err)
		}

		raw, _ := keys.Verify(start.StateCookie)
		var payload statePayload
		_ = json.Unmarshal(raw, &payload)
		if payload.Next != "/" {
			t.Errorf("next %q must collapse to /, got %q", next, payload.Next)
		}
	}
}

func TestFinishLoginRejectsBadState(t *testing.T) {
	service, keys := newService(t, "client-id")

	signed := func(payload statePayload) string {
		raw, _ := json.Marshal(payload)
		return keys.Sign(raw)
	}
	valid := statePayload{State: "state-1", Verifier: "v", Next: "/", Expires: time.Now().Add(time.Minute).Unix()}

	tests := map[string]struct{ code, state, cookie string }{
		"no cookie":        {"code", "state-1", ""},
		"forged cookie":    {"code", "state-1", "e30.AAAA"},
		"state mismatch":   {"code", "other", signed(valid)},
		"missing code":     {"", "state-1", signed(valid)},
		"expired":          {"code", "state-1", signed(statePayload{State: "state-1", Verifier: "v", Expires: time.Now().Add(-time.Minute).Unix()})},
		"empty state":      {"code", "", signed(statePayload{Verifier: "v", Expires: time.Now().Add(time.Minute).Unix()})},
		"tampered payload": {"code", "state-1", strings.Replace(signed(valid), ".", "x.", 1)},
	}

	for name, test := range tests {
		_, err := service.FinishLogin(context.Background(), test.code, test.state, test.cookie)
		if appErr, ok := apperror.As(err); !ok || appErr.Code != "invalid_oauth_state" {
			t.Errorf("%s: expected invalid_oauth_state, got %v", name, err)
		}
	}
}

func TestStartLoginNeedsGoogleCredentials(t *testing.T) {
	service, _ := newService(t, "")

	_, err := service.StartLogin(context.Background(), "/")
	if appErr, ok := apperror.As(err); !ok || appErr.Status != 503 {
		t.Fatalf("expected 503 google_not_configured, got %v", err)
	}
}
