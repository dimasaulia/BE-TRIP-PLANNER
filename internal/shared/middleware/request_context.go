package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/open-suite/boilerplate-golang/internal/shared/requestctx"
)

const requestIDHeader = "X-Request-ID"

func RequestContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(requestIDHeader)
		if strings.TrimSpace(requestID) == "" {
			requestID = newRequestID()
		}

		ctx := requestctx.WithRequestID(r.Context(), requestID)
		ctx = requestctx.WithLanguage(ctx, parseLanguage(r.Header.Get("Accept-Language")))
		ctx = requestctx.WithStartTime(ctx, time.Now())

		w.Header().Set(requestIDHeader, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func parseLanguage(header string) string {
	header = strings.ToLower(strings.TrimSpace(header))
	if header == "" {
		return "id"
	}

	for _, part := range strings.Split(header, ",") {
		language := strings.TrimSpace(strings.Split(part, ";")[0])
		if strings.HasPrefix(language, "id") {
			return "id"
		}
		if strings.HasPrefix(language, "en") {
			return "en"
		}
	}

	return "id"
}

func newRequestID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}

	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80

	encoded := hex.EncodeToString(bytes)
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32]
}
