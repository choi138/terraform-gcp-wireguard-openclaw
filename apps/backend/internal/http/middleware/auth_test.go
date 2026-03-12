package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/http/middleware"
)

func TestWithBearerAuthRejectsInvalidToken(t *testing.T) {
	h := middleware.WithBearerAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), "expected-token", "admin")

	req := httptest.NewRequest(http.MethodGet, "/v1/dashboard/summary", nil)
	req.Header.Set("Authorization", "Bearer invalid")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestWithBearerAuthPassesValidToken(t *testing.T) {
	h := middleware.WithBearerAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor := middleware.ActorFromContext(r.Context())
		if actor != "admin" {
			t.Fatalf("expected actor admin in context, got %q", actor)
		}
		w.WriteHeader(http.StatusNoContent)
	}), "expected-token", "admin")

	req := httptest.NewRequest(http.MethodGet, "/v1/dashboard/summary", nil)
	req.Header.Set("Authorization", "Bearer expected-token")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rr.Code)
	}
}

func TestWithBearerAuthRejectsEmptyActor(t *testing.T) {
	h := middleware.WithBearerAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("expected handler not to be invoked")
	}), "expected-token", "")

	req := httptest.NewRequest(http.MethodGet, "/v1/dashboard/summary", nil)
	req.Header.Set("Authorization", "Bearer expected-token")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
}

func TestWithBearerAuthRejectsMissingAuthorizationHeader(t *testing.T) {
	h := middleware.WithBearerAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), "expected-token", "admin")

	req := httptest.NewRequest(http.MethodGet, "/v1/dashboard/summary", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestWithBearerAuthRejectsTokenWithoutBearerPrefix(t *testing.T) {
	h := middleware.WithBearerAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), "expected-token", "admin")

	req := httptest.NewRequest(http.MethodGet, "/v1/dashboard/summary", nil)
	req.Header.Set("Authorization", "expected-token")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestWithBearerAuthIgnoresActorHeader(t *testing.T) {
	h := middleware.WithBearerAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor := middleware.ActorFromContext(r.Context())
		if actor != "admin" {
			t.Fatalf("expected actor admin in context, got %q", actor)
		}
		w.WriteHeader(http.StatusNoContent)
	}), "expected-token", "admin")

	req := httptest.NewRequest(http.MethodGet, "/v1/dashboard/summary", nil)
	req.Header.Set("Authorization", "Bearer expected-token")
	req.Header.Set("X-Actor-ID", "spoofed")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rr.Code)
	}
}

func TestWithBearerAuthRejectsWhenTokenEmpty(t *testing.T) {
	h := middleware.WithBearerAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if actor := middleware.ActorFromContext(r.Context()); actor != "" {
			t.Fatalf("expected empty actor, got %q", actor)
		}
		w.WriteHeader(http.StatusNoContent)
	}), "", "admin")

	req := httptest.NewRequest(http.MethodGet, "/v1/dashboard/summary", nil)
	req.Header.Set("Authorization", "Bearer expected-token")
	req.Header.Set("X-Actor-ID", "spoofed")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}
