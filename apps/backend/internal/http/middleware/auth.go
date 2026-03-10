package middleware

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
)

type contextKey string

const actorKey contextKey = "actor"

func ActorFromContext(ctx context.Context) string {
	v, ok := ctx.Value(actorKey).(string)
	if !ok {
		return ""
	}
	return v
}

func WithBearerAuth(next http.Handler, token string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			writeAuthError(w, http.StatusUnauthorized, "admin token is not configured")
			return
		}

		authz := strings.TrimSpace(r.Header.Get("Authorization"))
		if !strings.HasPrefix(authz, "Bearer ") {
			writeAuthError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}

		provided := strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
		if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			writeAuthError(w, http.StatusUnauthorized, "invalid bearer token")
			return
		}

		ctx := context.WithValue(r.Context(), actorKey, "admin")
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func writeAuthError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
