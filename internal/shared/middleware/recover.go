package middleware

import (
	"net/http"

	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
	"github.com/open-suite/boilerplate-golang/internal/shared/response"
)

func Recover(appLogger *logger.Logger, sender *response.Sender) Middleware {
	log := appLogger.Layer("middleware.recover")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					log.Error(r.Context(), "panic", nil, "panic", recovered)
					sender.Error(w, r, http.StatusInternalServerError, "error.internal", nil)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
