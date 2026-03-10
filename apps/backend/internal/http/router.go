package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/config"
	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/http/handler"
	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/http/middleware"
)

type Dependencies struct {
	Readiness    handler.ReadinessChecker
	Dashboard    handler.DashboardReader
	Conversation handler.ConversationReader
	Infra        handler.InfraReader
	Audit        middleware.ReadAuditWriter
}

func NewRouter(cfg config.Config, deps Dependencies, logger *slog.Logger) http.Handler {
	api := handler.New(handler.Dependencies{
		Readiness:    deps.Readiness,
		Dashboard:    deps.Dashboard,
		Conversation: deps.Conversation,
		Infra:        deps.Infra,
	}, logger)
	mux := http.NewServeMux()

	mux.HandleFunc("GET /v1/healthz", api.Healthz)
	mux.HandleFunc("GET /v1/readyz", api.Readyz)

	registerProtected := func(pattern string, h http.HandlerFunc) {
		var wrapped http.Handler = h
		wrapped = middleware.WithReadAudit(wrapped, deps.Audit, logger)
		wrapped = middleware.WithBearerAuth(wrapped, cfg.AdminToken)
		mux.Handle(pattern, wrapped)
	}

	registerProtected("GET /v1/dashboard/summary", api.DashboardSummary)
	registerProtected("GET /v1/dashboard/timeseries", api.DashboardTimeseries)
	registerProtected("GET /v1/conversations", api.ConversationsList)
	registerProtected("GET /v1/conversations/{id}", api.ConversationDetail)
	registerProtected("GET /v1/conversations/{id}/messages", api.ConversationMessages)
	registerProtected("GET /v1/conversations/{id}/attempts", api.ConversationAttempts)
	registerProtected("GET /v1/infra/status", api.InfraStatus)
	registerProtected("GET /v1/infra/snapshots", api.InfraSnapshots)

	return mux
}
