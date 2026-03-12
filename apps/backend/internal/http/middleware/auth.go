package middleware

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/config"
	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/domain"
)

type AuthAuditWriter interface {
	InsertAuditEvent(ctx context.Context, event domain.AuditEvent) error
}

type AuthPolicy struct {
	Name         string
	AllowedRoles []Role
}

type Authorizer struct {
	adminToken              string
	adminTokenCompatibility bool
	ingestToken             string
	oidc                    *oidcValidator
	breakGlass              config.BreakGlassConfig
	audit                   AuthAuditWriter
	logger                  *slog.Logger
}

func NewAuthorizer(cfg config.Config, audit AuthAuditWriter, logger *slog.Logger) *Authorizer {
	var validator *oidcValidator
	if cfg.OIDC.Enabled() {
		validator = newOIDCValidator(cfg.OIDC)
	}
	return &Authorizer{
		adminToken:              cfg.AdminToken,
		adminTokenCompatibility: cfg.AdminTokenCompatibility,
		ingestToken:             cfg.IngestToken,
		oidc:                    validator,
		breakGlass:              cfg.BreakGlass,
		audit:                   audit,
		logger:                  logger,
	}
}

func (a *Authorizer) WithAuthorization(next http.Handler, policy AuthPolicy) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerTokenFromRequest(r)
		if !ok {
			writeAuthError(r.Context(), w, http.StatusUnauthorized, "auth_missing_bearer", "missing bearer token")
			return
		}

		principal, status, code, message := a.authorize(r.Context(), token, r.URL.Path, policy)
		if status != 0 {
			writeAuthError(r.Context(), w, status, code, message)
			return
		}

		ctx := WithPrincipal(r.Context(), principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *Authorizer) authorize(ctx context.Context, token, path string, policy AuthPolicy) (Principal, int, string, string) {
	if hasOnlyRole(policy.AllowedRoles, RoleIngest) {
		if subtle.ConstantTimeCompare([]byte(token), []byte(a.ingestToken)) != 1 {
			return Principal{}, http.StatusUnauthorized, "auth_invalid_token", "invalid bearer token"
		}
		return Principal{
			ID:    "ingest",
			Type:  PrincipalTypeService,
			Roles: []Role{RoleIngest},
		}, 0, "", ""
	}

	oidcEnabled := a.oidc != nil
	if oidcEnabled {
		if isJWT(token) {
			principal, err := a.oidc.Validate(ctx, token)
			switch {
			case err == nil:
				if !HasAnyRole(principal, policy.AllowedRoles) {
					return Principal{}, http.StatusForbidden, "authorization_forbidden", "principal is not authorized for this route"
				}
				return principal, 0, "", ""
			default:
				return Principal{}, http.StatusUnauthorized, "auth_invalid_oidc_token", "invalid OIDC token"
			}
		}

		if a.breakGlass.Enabled && subtle.ConstantTimeCompare([]byte(token), []byte(a.breakGlass.Token)) == 1 {
			principal, status, code, message := a.authorizeBreakGlass(ctx, path, policy)
			if status != 0 {
				return Principal{}, status, code, message
			}
			return principal, 0, "", ""
		}

		if a.adminTokenCompatibility && a.adminToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(a.adminToken)) == 1 {
			principal := Principal{
				ID:    "admin",
				Type:  PrincipalTypeLegacyToken,
				Roles: []Role{RoleAdmin},
			}
			if !HasAnyRole(principal, policy.AllowedRoles) {
				return Principal{}, http.StatusForbidden, "authorization_forbidden", "principal is not authorized for this route"
			}
			return principal, 0, "", ""
		}

		return Principal{}, http.StatusUnauthorized, "auth_oidc_required", "OIDC token is required for this route"
	}

	if a.adminToken == "" {
		return Principal{}, http.StatusInternalServerError, "auth_not_configured", "operator authentication is not configured"
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(a.adminToken)) != 1 {
		return Principal{}, http.StatusUnauthorized, "auth_invalid_token", "invalid bearer token"
	}
	principal := Principal{
		ID:    "admin",
		Type:  PrincipalTypeLegacyToken,
		Roles: []Role{RoleAdmin},
	}
	if !HasAnyRole(principal, policy.AllowedRoles) {
		return Principal{}, http.StatusForbidden, "authorization_forbidden", "principal is not authorized for this route"
	}
	return principal, 0, "", ""
}

func (a *Authorizer) authorizeBreakGlass(ctx context.Context, path string, policy AuthPolicy) (Principal, int, string, string) {
	if !a.breakGlass.ExpiresAt.IsZero() && time.Now().UTC().After(a.breakGlass.ExpiresAt) {
		return Principal{}, http.StatusUnauthorized, "auth_break_glass_expired", "break-glass token has expired"
	}
	if !pathAllowed(path, a.breakGlass.AllowedPaths) {
		return Principal{}, http.StatusForbidden, "authorization_break_glass_path_denied", "break-glass token is not allowed on this route"
	}

	principal := Principal{
		ID:    "break-glass",
		Type:  PrincipalTypeBreakGlass,
		Roles: []Role{Role(strings.TrimSpace(a.breakGlass.Role))},
	}
	if !HasAnyRole(principal, policy.AllowedRoles) {
		return Principal{}, http.StatusForbidden, "authorization_forbidden", "principal is not authorized for this route"
	}

	if a.audit == nil {
		return Principal{}, http.StatusServiceUnavailable, "auth_break_glass_audit_unavailable", "break-glass audit writer is not configured"
	}
	event := domain.AuditEvent{
		Actor:        principal.ID,
		Action:       "auth.break_glass",
		ResourceType: "auth_session",
		Metadata: map[string]any{
			"path":        path,
			"role":        a.breakGlass.Role,
			"approver":    a.breakGlass.Approver,
			"reason":      a.breakGlass.Reason,
			"expires_at":  a.breakGlass.ExpiresAt.UTC().Format(time.RFC3339),
			"request_id":  RequestIDFromContext(ctx),
			"policy_name": policy.Name,
		},
		CreatedAt: time.Now().UTC(),
	}
	if err := a.audit.InsertAuditEvent(ctx, event); err != nil {
		if a.logger != nil {
			a.logger.Error("failed to persist break-glass audit event", "error", err, "path", path)
		}
		return Principal{}, http.StatusServiceUnavailable, "auth_break_glass_audit_failed", "failed to audit break-glass token use"
	}
	if a.logger != nil {
		a.logger.Warn("break-glass token used",
			"path", path,
			"role", a.breakGlass.Role,
			"request_id", RequestIDFromContext(ctx),
			"approver", a.breakGlass.Approver,
		)
	}
	return principal, 0, "", ""
}

func WithBearerAuth(next http.Handler, token, actor string) http.Handler {
	effectiveActor := strings.TrimSpace(actor)
	if effectiveActor == "" {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeAuthError(r.Context(), w, http.StatusInternalServerError, "auth_actor_not_configured", "bearer actor is not configured")
		})
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			writeAuthError(r.Context(), w, http.StatusUnauthorized, "auth_token_not_configured", "bearer token is not configured")
			return
		}
		provided, ok := bearerTokenFromRequest(r)
		if !ok {
			writeAuthError(r.Context(), w, http.StatusUnauthorized, "auth_missing_bearer", "missing bearer token")
			return
		}
		if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			writeAuthError(r.Context(), w, http.StatusUnauthorized, "auth_invalid_token", "invalid bearer token")
			return
		}

		role := Role(effectiveActor)
		ctx := WithPrincipal(r.Context(), Principal{
			ID:    effectiveActor,
			Type:  PrincipalTypeLegacyToken,
			Roles: []Role{role},
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func writeAuthError(ctx context.Context, w http.ResponseWriter, code int, errorCode, message string) {
	SetErrorCode(ctx, errorCode)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func bearerTokenFromRequest(r *http.Request) (string, bool) {
	authz := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(authz, "Bearer ") {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
	if token == "" {
		return "", false
	}
	return token, true
}

func hasOnlyRole(roles []Role, role Role) bool {
	return len(roles) == 1 && roles[0] == role
}

func pathAllowed(path string, allowed []string) bool {
	for _, prefix := range allowed {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func isJWT(token string) bool {
	return strings.Count(token, ".") == 2
}
