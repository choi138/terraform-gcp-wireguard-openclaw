package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/config"
	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/http/handler"
	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/http/middleware"
	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/telemetry"
)

type AuditWriter interface {
	middleware.ReadAuditWriter
	handler.AuditWriter
}

type Dependencies struct {
	Readiness    handler.ReadinessChecker
	Dashboard    handler.DashboardReader
	Conversation handler.ConversationReader
	Infra        handler.InfraReader
	Security     handler.SecurityAnalyzer
	Ingest       handler.IngestWriter
	Audit        AuditWriter
}

func NewRouter(cfg config.Config, deps Dependencies, logger *slog.Logger) http.Handler {
	api := handler.New(handler.Dependencies{
		Readiness:            deps.Readiness,
		Dashboard:            deps.Dashboard,
		Conversation:         deps.Conversation,
		Infra:                deps.Infra,
		Security:             deps.Security,
		Ingest:               deps.Ingest,
		Audit:                deps.Audit,
		IngestMaxBodyBytes:   cfg.IngestMaxBodyBytes,
		SecurityMaxBodyBytes: cfg.SecurityMaxBodyBytes,
	}, logger)
	mux := http.NewServeMux()
	authorizer := middleware.NewAuthorizer(cfg, deps.Audit, logger)
	metricsRegistry := telemetry.NewRegistry(deps.Ingest)

	var exporter telemetry.Exporter
	if cfg.Trace.Exporter == "stdout" {
		exporter = telemetry.NewSlogExporter(logger)
	}
	tracer := telemetry.NewTracer(exporter, cfg.Trace.SampleRate, cfg.Trace.ServiceName)

	mux.HandleFunc("GET /v1/healthz", api.Healthz)
	mux.HandleFunc("GET /v1/readyz", api.Readyz)

	registerRead := func(pattern string, roles []middleware.Role, h http.HandlerFunc) {
		var wrapped http.Handler = h
		wrapped = middleware.WithReadAudit(wrapped, deps.Audit, logger)
		wrapped = authorizer.WithAuthorization(wrapped, middleware.AuthPolicy{
			Name:         pattern,
			AllowedRoles: roles,
		})
		mux.Handle(pattern, wrapped)
	}

	registerAdmin := func(pattern string, h http.HandlerFunc) {
		var wrapped http.Handler = h
		wrapped = authorizer.WithAuthorization(wrapped, middleware.AuthPolicy{
			Name:         pattern,
			AllowedRoles: []middleware.Role{middleware.RoleAdmin},
		})
		mux.Handle(pattern, wrapped)
	}

	registerIngest := func(pattern string, h http.HandlerFunc) {
		var wrapped http.Handler = h
		wrapped = authorizer.WithAuthorization(wrapped, middleware.AuthPolicy{
			Name:         pattern,
			AllowedRoles: []middleware.Role{middleware.RoleIngest},
		})
		mux.Handle(pattern, wrapped)
	}

	viewerRoles := []middleware.Role{middleware.RoleAdmin, middleware.RoleViewer, middleware.RoleAuditor}
	auditorRoles := []middleware.Role{middleware.RoleAdmin, middleware.RoleAuditor}

	registerRead("GET /v1/dashboard/summary", viewerRoles, api.DashboardSummary)
	registerRead("GET /v1/dashboard/timeseries", viewerRoles, api.DashboardTimeseries)
	registerRead("GET /v1/conversations", viewerRoles, api.ConversationsList)
	registerRead("GET /v1/conversations/{id}", viewerRoles, api.ConversationDetail)
	registerRead("GET /v1/conversations/{id}/messages", viewerRoles, api.ConversationMessages)
	registerRead("GET /v1/conversations/{id}/attempts", viewerRoles, api.ConversationAttempts)
	registerRead("GET /v1/infra/status", viewerRoles, api.InfraStatus)
	registerRead("GET /v1/infra/snapshots", viewerRoles, api.InfraSnapshots)
	registerRead("GET /v1/ingest/status", viewerRoles, api.IngestStatus)
	registerRead("GET /v1/security/findings", auditorRoles, api.SecurityFindings)
	registerAdmin("POST /v1/security/analyze-tfvars", api.AnalyzeSecurityTfvars)
	registerIngest("POST /v1/ingest/conversation-events", api.IngestConversationEvents)
	registerIngest("POST /v1/ingest/infra-snapshot", api.IngestInfraSnapshot)
	registerIngest("POST /v1/ingest/request-attempt", api.IngestRequestAttempt)

	metricsHandler := authorizer.WithAuthorization(metricsRegistry, middleware.AuthPolicy{
		Name:         "GET /metrics",
		AllowedRoles: viewerRoles,
	})
	mux.Handle("GET /metrics", metricsHandler)

	return middleware.WithObservability(mux, metricsRegistry, tracer, logger)
}
