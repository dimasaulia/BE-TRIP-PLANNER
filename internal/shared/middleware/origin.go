package middleware

import (
	"net/http"
	"strings"

	"github.com/open-suite/boilerplate-golang/internal/shared/apperror"
	"github.com/open-suite/boilerplate-golang/internal/shared/response"
)

// OriginAllowed reports whether a browser supplied Origin is on the allow list.
// A missing Origin means a non browser client, which has no CSRF exposure.
func OriginAllowed(allowed []string, origin string) bool {
	origin = strings.ToLower(strings.TrimRight(strings.TrimSpace(origin), "/"))
	if origin == "" {
		return true
	}

	for _, candidate := range allowed {
		if candidate == origin {
			return true
		}
	}
	return false
}

// Origin rejects state changing requests whose Origin header is not allowed.
func Origin(allowed []string, sender *response.Sender) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
				if !OriginAllowed(allowed, r.Header.Get("Origin")) {
					sender.Fail(w, r, apperror.Forbidden("origin_not_allowed"))
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
