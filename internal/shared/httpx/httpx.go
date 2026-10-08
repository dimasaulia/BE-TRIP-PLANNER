package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/open-suite/boilerplate-golang/internal/shared/apperror"
)

const MaxBodyBytes = 1 << 20

// DecodeJSON reads a size limited JSON body into dst. Errors raised by custom
// unmarshalers (for example invalid_time) are passed through as is.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)

	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		if appErr, ok := apperror.As(err); ok {
			return appErr
		}

		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return apperror.TooLarge("payload_too_large")
		}
		if errors.Is(err, io.EOF) {
			return apperror.BadRequest("invalid_json").With("reason", "empty body")
		}
		return apperror.BadRequest("invalid_json")
	}

	return nil
}

func PathUUID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		return uuid.Nil, apperror.BadRequest("invalid_id").With("param", name)
	}
	return id, nil
}

func QueryInt(r *http.Request, key string, fallback int, min int, max int) int {
	value := r.URL.Query().Get(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	if parsed < min {
		return min
	}
	if parsed > max {
		return max
	}
	return parsed
}

func QueryBool(r *http.Request, key string) bool {
	parsed, _ := strconv.ParseBool(r.URL.Query().Get(key))
	return parsed
}

// ClientIP returns the caller address; X-Forwarded-For is only honoured when
// the deployment sits behind a trusted reverse proxy.
func ClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			first := strings.TrimSpace(strings.Split(forwarded, ",")[0])
			if first != "" {
				return first
			}
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// SafeNext only lets internal absolute paths through, preventing open redirects.
func SafeNext(next string) string {
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		return "/"
	}
	if strings.ContainsAny(next, "\\\r\n\t") {
		return "/"
	}
	return next
}

type CookieOptions struct {
	Name   string
	Secure bool
	Path   string
}

func SetCookie(w http.ResponseWriter, options CookieOptions, value string, expires time.Time) {
	path := options.Path
	if path == "" {
		path = "/"
	}

	cookie := &http.Cookie{
		Name:     options.Name,
		Value:    value,
		Path:     path,
		HttpOnly: true,
		Secure:   options.Secure,
		SameSite: http.SameSiteLaxMode,
	}
	if !expires.IsZero() {
		cookie.Expires = expires
		cookie.MaxAge = int(time.Until(expires).Seconds())
	}

	http.SetCookie(w, cookie)
}

func ClearCookie(w http.ResponseWriter, options CookieOptions) {
	path := options.Path
	if path == "" {
		path = "/"
	}

	http.SetCookie(w, &http.Cookie{
		Name:     options.Name,
		Value:    "",
		Path:     path,
		HttpOnly: true,
		Secure:   options.Secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
