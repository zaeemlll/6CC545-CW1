package security

import "net/http"

// PublicAPI protects the public API while leaving the health endpoint public.
func PublicAPI(v TokenValidator, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}

		requiredRole := "reader"
		if r.Method != http.MethodGet || r.URL.Path == "/v1/events" {
			requiredRole = "writer"
		}

		Require(v, requiredRole, next).ServeHTTP(w, r)
	})
}
