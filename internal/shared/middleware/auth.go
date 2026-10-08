package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/open-suite/boilerplate-golang/internal/platform/config"
	"github.com/open-suite/boilerplate-golang/internal/shared/apperror"
	"github.com/open-suite/boilerplate-golang/internal/shared/httpx"
	"github.com/open-suite/boilerplate-golang/internal/shared/requestctx"
	"github.com/open-suite/boilerplate-golang/internal/shared/response"
)

// SessionResolver turns the raw cookie value into a user. When the sliding
// expiry was pushed forward it returns the new expiry so the cookie can follow.
type SessionResolver interface {
	Resolve(ctx context.Context, token string) (*requestctx.User, *time.Time, error)
}

type Auth struct {
	resolver SessionResolver
	sender   *response.Sender
	cookie   httpx.CookieOptions
}

func NewAuth(resolver SessionResolver, sender *response.Sender, cfg config.Config) *Auth {
	return &Auth{
		resolver: resolver,
		sender:   sender,
		cookie: httpx.CookieOptions{
			Name:   cfg.Auth.SessionCookieName(),
			Secure: cfg.Auth.CookieSecure,
		},
	}
}

// Require rejects requests without a valid session and stores the user in the
// request context for downstream handlers.
func (a *Auth) Require(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(a.cookie.Name)
		if err != nil || cookie.Value == "" {
			a.sender.Fail(w, r, apperror.Unauthorized())
			return
		}

		user, extended, err := a.resolver.Resolve(r.Context(), cookie.Value)
		if err != nil {
			a.sender.Fail(w, r, err)
			return
		}
		if extended != nil {
			httpx.SetCookie(w, a.cookie, cookie.Value, *extended)
		}

		next(w, r.WithContext(requestctx.WithUser(r.Context(), *user)))
	}
}
