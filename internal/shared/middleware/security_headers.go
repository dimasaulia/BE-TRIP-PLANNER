package middleware

import "net/http"

// SecurityHeaders adds the response headers every reply should carry. HSTS is
// only sent when the deployment is HTTPS, where the TRD makes it mandatory.
func SecurityHeaders(https bool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := w.Header()
			header.Set("X-Content-Type-Options", "nosniff")
			header.Set("Referrer-Policy", "same-origin")
			if https {
				header.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}

			next.ServeHTTP(w, r)
		})
	}
}
