package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/domain"
)

type ReadAuditWriter interface {
	InsertReadAudit(ctx context.Context, event domain.AuditEvent) error
}

type readAuditRecorder struct {
	http.ResponseWriter
	status int
}

var uuidPattern = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[1-5][a-fA-F0-9]{3}-[89abAB][a-fA-F0-9]{3}-[a-fA-F0-9]{12}$`)

func (s *readAuditRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func WithReadAudit(next http.Handler, auditWriter ReadAuditWriter, logger *slog.Logger) http.Handler {
	if auditWriter == nil {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &readAuditRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		if r.Method != http.MethodGet {
			return
		}

		resourceType, resourceID := parseResource(r.URL.Path)
		event := domain.AuditEvent{
			Actor:        ActorFromContext(r.Context()),
			Action:       "read",
			ResourceType: resourceType,
			ResourceID:   resourceID,
			Metadata: map[string]any{
				"path":   r.URL.Path,
				"method": r.Method,
				"status": rec.status,
			},
			CreatedAt: time.Now().UTC(),
		}

		if event.Actor == "" {
			event.Actor = "unknown"
		}
		if event.ResourceType == "" {
			event.ResourceType = "unknown"
		}

		if err := auditWriter.InsertReadAudit(r.Context(), event); err != nil && logger != nil {
			logger.Warn("failed to persist audit event", "error", err, "path", r.URL.Path)
		}
	})
}

func parseResource(path string) (string, string) {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return "", ""
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) < 2 {
		return "", ""
	}

	resourceType := parts[1]
	resourceID := ""
	if len(parts) >= 3 && looksLikeIdentifier(parts[2]) {
		resourceID = parts[2]
	}
	return resourceType, resourceID
}

func looksLikeIdentifier(v string) bool {
	if v == "" {
		return false
	}
	if _, err := strconv.ParseInt(v, 10, 64); err == nil {
		return true
	}
	return uuidPattern.MatchString(v)
}
