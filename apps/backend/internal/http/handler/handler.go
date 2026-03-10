package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/domain"
	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/repository"
)

type ReadinessChecker interface {
	Ping(ctx context.Context) error
}

type DashboardReader interface {
	GetSummary(ctx context.Context, from, to time.Time) (domain.DashboardSummary, error)
	GetTimeseries(ctx context.Context, metric, bucket string, from, to time.Time) ([]domain.DashboardPoint, error)
}

type ConversationReader interface {
	ListConversations(ctx context.Context, filter domain.ConversationFilter, pagination domain.Pagination) ([]domain.Conversation, error)
	GetConversation(ctx context.Context, conversationID int64) (domain.Conversation, error)
	ListMessages(ctx context.Context, conversationID int64, pagination domain.Pagination) ([]domain.Message, error)
	ListAttempts(ctx context.Context, conversationID int64, pagination domain.Pagination) ([]domain.RequestAttempt, error)
}

type InfraReader interface {
	GetLatestStatus(ctx context.Context) (domain.InfraSnapshot, error)
	ListSnapshots(ctx context.Context, from, to time.Time, pagination domain.Pagination) ([]domain.InfraSnapshot, error)
}

type Dependencies struct {
	Readiness    ReadinessChecker
	Dashboard    DashboardReader
	Conversation ConversationReader
	Infra        InfraReader
}

type API struct {
	readiness    ReadinessChecker
	dashboard    DashboardReader
	conversation ConversationReader
	infra        InfraReader
	logger       *slog.Logger
}

var dateOnlyPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func New(deps Dependencies, logger *slog.Logger) *API {
	if deps.Readiness == nil {
		panic("handler.New requires Readiness")
	}
	if deps.Dashboard == nil {
		panic("handler.New requires Dashboard")
	}
	if deps.Conversation == nil {
		panic("handler.New requires Conversation")
	}
	if deps.Infra == nil {
		panic("handler.New requires Infra")
	}

	return &API{
		readiness:    deps.Readiness,
		dashboard:    deps.Dashboard,
		conversation: deps.Conversation,
		infra:        deps.Infra,
		logger:       logger,
	}
}

func (a *API) Healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) Readyz(w http.ResponseWriter, r *http.Request) {
	if a.readiness == nil {
		writeError(w, http.StatusServiceUnavailable, "readiness repository is not configured")
		return
	}
	if err := a.readiness.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "dependency is not ready")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (a *API) DashboardSummary(w http.ResponseWriter, r *http.Request) {
	from, to, err := parseTimeRange(r, true)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	summary, err := a.dashboard.GetSummary(r.Context(), from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load dashboard summary")
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func (a *API) DashboardTimeseries(w http.ResponseWriter, r *http.Request) {
	metric := r.URL.Query().Get("metric")
	bucket := r.URL.Query().Get("bucket")
	if metric == "" {
		metric = "requests"
	}
	if bucket == "" {
		bucket = "1h"
	}

	if !domain.IsAllowedDashboardMetric(metric) {
		writeError(w, http.StatusBadRequest, "metric must be one of: requests,tokens,cost,errors")
		return
	}
	if !domain.IsAllowedDashboardBucket(bucket) {
		writeError(w, http.StatusBadRequest, "bucket must be one of: 1m,5m,1h,day")
		return
	}

	from, to, err := parseTimeRange(r, true)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	points, err := a.dashboard.GetTimeseries(r.Context(), metric, bucket, from, to)
	if err != nil {
		if errors.Is(err, repository.ErrInvalidInput) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load dashboard timeseries")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"metric": metric,
		"bucket": bucket,
		"from":   from,
		"to":     to,
		"points": points,
	})
}

func (a *API) ConversationsList(w http.ResponseWriter, r *http.Request) {
	pagination := parsePagination(r)
	filter := domain.ConversationFilter{
		Channel: r.URL.Query().Get("channel"),
		Status:  r.URL.Query().Get("status"),
	}

	items, err := a.conversation.ListConversations(r.Context(), filter, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list conversations")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"page":      pagination.Page,
		"page_size": pagination.PageSize,
		"items":     items,
	})
}

func (a *API) ConversationDetail(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathInt64(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid conversation id")
		return
	}

	item, err := a.conversation.GetConversation(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "conversation not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get conversation")
		return
	}

	writeJSON(w, http.StatusOK, item)
}

func (a *API) ConversationMessages(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathInt64(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid conversation id")
		return
	}

	pagination := parsePagination(r)
	items, err := a.conversation.ListMessages(r.Context(), id, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list messages")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"page":      pagination.Page,
		"page_size": pagination.PageSize,
		"items":     items,
	})
}

func (a *API) ConversationAttempts(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathInt64(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid conversation id")
		return
	}

	pagination := parsePagination(r)
	items, err := a.conversation.ListAttempts(r.Context(), id, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list attempts")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"page":      pagination.Page,
		"page_size": pagination.PageSize,
		"items":     items,
	})
}

func (a *API) InfraStatus(w http.ResponseWriter, r *http.Request) {
	item, err := a.infra.GetLatestStatus(r.Context())
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "infra status not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to read infra status")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) InfraSnapshots(w http.ResponseWriter, r *http.Request) {
	from, to, err := parseTimeRange(r, true)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	pagination := parsePagination(r)
	items, err := a.infra.ListSnapshots(r.Context(), from, to, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read infra snapshots")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"from":      from,
		"to":        to,
		"page":      pagination.Page,
		"page_size": pagination.PageSize,
		"items":     items,
	})
}

func parseTimeRange(r *http.Request, required bool) (time.Time, time.Time, error) {
	fromRaw := r.URL.Query().Get("from")
	toRaw := r.URL.Query().Get("to")

	if fromRaw == "" || toRaw == "" {
		if required {
			return time.Time{}, time.Time{}, errors.New("from and to are required")
		}
		now := time.Now().UTC()
		return now.Add(-24 * time.Hour), now, nil
	}

	from, err := parseTime(fromRaw)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("from must be RFC3339 or YYYY-MM-DD")
	}
	to, err := parseTime(toRaw)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("to must be RFC3339 or YYYY-MM-DD")
	}
	if dateOnlyPattern.MatchString(toRaw) {
		to = to.Add(24*time.Hour - time.Nanosecond)
	}

	if to.Before(from) {
		return time.Time{}, time.Time{}, errors.New("to must be greater than or equal to from")
	}
	return from.UTC(), to.UTC(), nil
}

func parseTime(v string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", v); err == nil {
		return t, nil
	}
	return time.Time{}, errors.New("invalid time")
}

func parsePagination(r *http.Request) domain.Pagination {
	page := parseIntOrDefault(r.URL.Query().Get("page"), 1)
	pageSize := parseIntOrDefault(r.URL.Query().Get("page_size"), 50)

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}

	return domain.Pagination{Page: page, PageSize: pageSize}
}

func parseIntOrDefault(v string, fallback int) int {
	if v == "" {
		return fallback
	}
	out, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return out
}

func parsePathInt64(v string) (int64, error) {
	if v == "" {
		return 0, errors.New("empty id")
	}
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, err
	}
	if id < 1 {
		return 0, errors.New("id must be greater than or equal to 1")
	}
	return id, nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
