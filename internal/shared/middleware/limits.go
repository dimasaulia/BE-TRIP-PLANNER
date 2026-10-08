package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/open-suite/boilerplate-golang/internal/platform/config"
	"github.com/open-suite/boilerplate-golang/internal/shared/apperror"
	"github.com/open-suite/boilerplate-golang/internal/shared/httpx"
	"github.com/open-suite/boilerplate-golang/internal/shared/ratelimit"
	"github.com/open-suite/boilerplate-golang/internal/shared/requestctx"
	"github.com/open-suite/boilerplate-golang/internal/shared/response"
)

// Limits groups the rate limiters: per IP for auth, invite and upload, per
// user for schedule mutations.
type Limits struct {
	auth       *ratelimit.Limiter
	invite     *ratelimit.Limiter
	upload     *ratelimit.Limiter
	schedule   *ratelimit.Limiter
	trustProxy bool
	sender     *response.Sender
}

func NewLimits(cfg config.Config, sender *response.Sender) *Limits {
	return &Limits{
		auth:       ratelimit.New(cfg.RateLimit.AuthPerMin, time.Minute),
		invite:     ratelimit.New(cfg.RateLimit.InvitePerMin, time.Minute),
		upload:     ratelimit.New(cfg.RateLimit.UploadPerMin, time.Minute),
		schedule:   ratelimit.New(cfg.RateLimit.SchedulePerMin, time.Minute),
		trustProxy: cfg.Auth.TrustProxy,
		sender:     sender,
	}
}

func (l *Limits) Auth(next http.HandlerFunc) http.HandlerFunc { return l.byIP(l.auth, "auth", next) }
func (l *Limits) Invite(next http.HandlerFunc) http.HandlerFunc {
	return l.byIP(l.invite, "invite", next)
}
func (l *Limits) Upload(next http.HandlerFunc) http.HandlerFunc {
	return l.byIP(l.upload, "upload", next)
}
func (l *Limits) Schedule(next http.HandlerFunc) http.HandlerFunc { return l.byUser(l.schedule, next) }

func (l *Limits) byIP(limiter *ratelimit.Limiter, scope string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !l.check(w, r, limiter, scope+":"+httpx.ClientIP(r, l.trustProxy)) {
			return
		}
		next(w, r)
	}
}

// byUser must run after Auth.Require.
func (l *Limits) byUser(limiter *ratelimit.Limiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := httpx.ClientIP(r, l.trustProxy)
		if user, ok := requestctx.UserFrom(r.Context()); ok {
			key = user.ID.String()
		}
		if !l.check(w, r, limiter, "user:"+key) {
			return
		}
		next(w, r)
	}
}

func (l *Limits) check(w http.ResponseWriter, r *http.Request, limiter *ratelimit.Limiter, key string) bool {
	allowed, retryAfter := limiter.Allow(key)
	if allowed {
		return true
	}

	seconds := int(retryAfter.Seconds()) + 1
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	l.sender.Fail(w, r, apperror.TooManyRequests("rate_limited").With("retry_after_seconds", seconds))
	return false
}
