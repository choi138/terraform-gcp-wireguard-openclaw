package middleware

import (
	"context"

	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/telemetry"
)

type contextKey string

const (
	requestStateKey contextKey = "request_state"
	principalKey    contextKey = "principal"
)

type Role string

const (
	RoleAdmin   Role = "admin"
	RoleViewer  Role = "viewer"
	RoleAuditor Role = "auditor"
	RoleIngest  Role = "ingest"
)

type PrincipalType string

const (
	PrincipalTypeOIDC        PrincipalType = "oidc"
	PrincipalTypeLegacyToken PrincipalType = "legacy_token"
	PrincipalTypeBreakGlass  PrincipalType = "break_glass"
	PrincipalTypeService     PrincipalType = "service"
)

type Principal struct {
	ID      string
	Subject string
	Issuer  string
	Type    PrincipalType
	Roles   []Role
}

type requestState struct {
	RequestID    string
	ErrorCode    string
	Trace        telemetry.TraceContext
	Principal    Principal
	HasPrincipal bool
}

func ContextWithRequestState(ctx context.Context, state *requestState) context.Context {
	return context.WithValue(ctx, requestStateKey, state)
}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	if state, ok := ctx.Value(requestStateKey).(*requestState); ok && state != nil {
		state.Principal = principal
		state.HasPrincipal = true
	}
	return context.WithValue(ctx, principalKey, principal)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	if state, ok := ctx.Value(requestStateKey).(*requestState); ok && state != nil && state.HasPrincipal {
		return state.Principal, true
	}
	principal, ok := ctx.Value(principalKey).(Principal)
	return principal, ok
}

func ActorFromContext(ctx context.Context) string {
	principal, ok := PrincipalFromContext(ctx)
	if !ok {
		return ""
	}
	return principal.ID
}

func RequestIDFromContext(ctx context.Context) string {
	if state, ok := ctx.Value(requestStateKey).(*requestState); ok && state != nil {
		return state.RequestID
	}
	return ""
}

func TraceContextFromContext(ctx context.Context) telemetry.TraceContext {
	if state, ok := ctx.Value(requestStateKey).(*requestState); ok && state != nil {
		return state.Trace
	}
	return telemetry.TraceContext{}
}

func SetErrorCode(ctx context.Context, code string) {
	if state, ok := ctx.Value(requestStateKey).(*requestState); ok && state != nil {
		state.ErrorCode = code
	}
}

func ErrorCodeFromContext(ctx context.Context) string {
	if state, ok := ctx.Value(requestStateKey).(*requestState); ok && state != nil {
		return state.ErrorCode
	}
	return ""
}

func HasAnyRole(principal Principal, allowed []Role) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, have := range principal.Roles {
		for _, want := range allowed {
			if have == want {
				return true
			}
		}
	}
	return false
}
