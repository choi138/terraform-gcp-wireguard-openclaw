package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/telemetry"
)

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

var numericSegmentPattern = regexp.MustCompile(`^\d+$`)

func WithObservability(next http.Handler, registry *telemetry.Registry, tracer *telemetry.Tracer, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now().UTC()
		requestID := stringsTrimOrRandom(r.Header.Get("X-Request-Id"))

		var traceCtx telemetry.TraceContext
		if tracer != nil {
			traceCtx = tracer.Start(r)
			if traceparent := telemetry.FormatTraceparent(traceCtx); traceparent != "" {
				w.Header().Set("Traceparent", traceparent)
			}
		}

		state := &requestState{
			RequestID: requestID,
			Trace:     traceCtx,
		}
		ctx := ContextWithRequestState(r.Context(), state)
		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		rec.Header().Set("X-Request-Id", requestID)

		next.ServeHTTP(rec, r.WithContext(ctx))

		finishedAt := time.Now().UTC()
		route := normalizeRoute(r.URL.Path)
		errorCode := state.ErrorCode
		if errorCode == "" && rec.status >= http.StatusBadRequest {
			errorCode = defaultErrorCode(rec.status)
		}
		if registry != nil {
			registry.ObserveRequest(r.Method, route, rec.status, finishedAt.Sub(startedAt), errorCode)
		}

		principalID := ""
		if state.HasPrincipal {
			principalID = state.Principal.ID
		}
		if tracer != nil {
			tracer.Finish(context.WithoutCancel(ctx), traceCtx, r.Method+" "+route, startedAt, finishedAt, map[string]any{
				"request_id":   requestID,
				"principal_id": principalID,
				"path":         r.URL.Path,
				"status":       rec.status,
				"error_code":   errorCode,
				"latency_ms":   finishedAt.Sub(startedAt).Milliseconds(),
			})
		}
		if logger != nil {
			attrs := []any{
				"request_id", requestID,
				"principal_id", principalID,
				"path", r.URL.Path,
				"latency_ms", finishedAt.Sub(startedAt).Milliseconds(),
				"status", rec.status,
				"error_code", errorCode,
			}
			switch {
			case rec.status >= http.StatusInternalServerError:
				logger.Error("http request completed", attrs...)
			case rec.status >= http.StatusBadRequest:
				logger.Warn("http request completed", attrs...)
			default:
				logger.Info("http request completed", attrs...)
			}
		}
	})
}

func defaultErrorCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	default:
		if status >= http.StatusInternalServerError {
			return "internal_error"
		}
		return ""
	}
}

func normalizeRoute(path string) string {
	switch {
	case path == "/metrics":
		return "/metrics"
	case path == "/v1/healthz":
		return "/v1/healthz"
	case path == "/v1/readyz":
		return "/v1/readyz"
	}

	parts := stringsSplit(path)
	for i := range parts {
		if numericSegmentPattern.MatchString(parts[i]) {
			parts[i] = "{id}"
			continue
		}
		if uuidPattern.MatchString(parts[i]) {
			parts[i] = "{id}"
		}
	}
	return "/" + stringsJoin(parts)
}

func stringsTrimOrRandom(v string) string {
	v = strings.TrimSpace(v)
	if v != "" {
		return v
	}
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return hex.EncodeToString(buf)
}

func stringsSplit(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return []string{}
	}
	return strings.Split(trimmed, "/")
}

func stringsJoin(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "/")
}
