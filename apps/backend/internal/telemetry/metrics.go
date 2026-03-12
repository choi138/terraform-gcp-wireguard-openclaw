package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/domain"
)

type IngestStatusReader interface {
	GetStatus(ctx context.Context) (domain.IngestStatus, error)
}

type Registry struct {
	mu             sync.RWMutex
	requests       map[requestKey]int64
	errors         map[errorKey]int64
	histograms     map[requestKey]*histogram
	ingest         IngestStatusReader
	latencyBuckets []float64
}

type requestKey struct {
	Method string
	Route  string
	Status int
}

type errorKey struct {
	Method    string
	Route     string
	ErrorCode string
}

type histogram struct {
	Counts []int64
	SumMS  float64
	Count  int64
}

func NewRegistry(ingest IngestStatusReader) *Registry {
	return &Registry{
		requests:       make(map[requestKey]int64),
		errors:         make(map[errorKey]int64),
		histograms:     make(map[requestKey]*histogram),
		ingest:         ingest,
		latencyBuckets: []float64{5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000},
	}
}

func (r *Registry) ObserveRequest(method, route string, status int, duration time.Duration, errorCode string) {
	if route == "" {
		route = "/unknown"
	}

	key := requestKey{
		Method: method,
		Route:  route,
		Status: status,
	}
	durationMS := float64(duration) / float64(time.Millisecond)

	r.mu.Lock()
	defer r.mu.Unlock()

	r.requests[key]++
	h, ok := r.histograms[key]
	if !ok {
		h = &histogram{Counts: make([]int64, len(r.latencyBuckets))}
		r.histograms[key] = h
	}
	h.Count++
	h.SumMS += durationMS
	for i, bucket := range r.latencyBuckets {
		if durationMS <= bucket {
			h.Counts[i]++
		}
	}

	if errorCode != "" {
		r.errors[errorKey{
			Method:    method,
			Route:     route,
			ErrorCode: errorCode,
		}]++
	}
}

func (r *Registry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write([]byte(r.Render(req.Context())))
}

func (r *Registry) Render(ctx context.Context) string {
	var b strings.Builder
	writeMetricHeader(&b, "ops_api_service_up", "gauge", "Whether the ops API process is up.")
	b.WriteString("ops_api_service_up 1\n")

	requestKeys, requestCounts, errorKeys, errorCounts, histograms, latencyBuckets := r.snapshot()

	writeMetricHeader(&b, "ops_api_http_requests_total", "counter", "Total HTTP requests handled by the ops API.")
	for _, key := range requestKeys {
		fmt.Fprintf(&b, "ops_api_http_requests_total{method=%q,path=%q,status=%q} %d\n",
			key.Method,
			key.Route,
			strconv.Itoa(key.Status),
			requestCounts[key],
		)
	}

	writeMetricHeader(&b, "ops_api_http_request_errors_total", "counter", "Total HTTP requests that completed with an error code.")
	for _, key := range errorKeys {
		fmt.Fprintf(&b, "ops_api_http_request_errors_total{method=%q,path=%q,error_code=%q} %d\n",
			key.Method,
			key.Route,
			key.ErrorCode,
			errorCounts[key],
		)
	}

	writeMetricHeader(&b, "ops_api_http_request_duration_ms", "histogram", "Latency of HTTP requests in milliseconds.")
	for _, key := range requestKeys {
		h, ok := histograms[key]
		if !ok {
			continue
		}
		var cumulative int64
		for i, bucket := range latencyBuckets {
			cumulative += h.Counts[i]
			fmt.Fprintf(&b, "ops_api_http_request_duration_ms_bucket{method=%q,path=%q,status=%q,le=%q} %d\n",
				key.Method,
				key.Route,
				strconv.Itoa(key.Status),
				strconv.FormatFloat(bucket, 'f', -1, 64),
				cumulative,
			)
		}
		fmt.Fprintf(&b, "ops_api_http_request_duration_ms_bucket{method=%q,path=%q,status=%q,le=%q} %d\n",
			key.Method,
			key.Route,
			strconv.Itoa(key.Status),
			"+Inf",
			h.Count,
		)
		fmt.Fprintf(&b, "ops_api_http_request_duration_ms_sum{method=%q,path=%q,status=%q} %s\n",
			key.Method,
			key.Route,
			strconv.Itoa(key.Status),
			strconv.FormatFloat(h.SumMS, 'f', -1, 64),
		)
		fmt.Fprintf(&b, "ops_api_http_request_duration_ms_count{method=%q,path=%q,status=%q} %d\n",
			key.Method,
			key.Route,
			strconv.Itoa(key.Status),
			h.Count,
		)
	}

	writeMetricHeader(&b, "ops_api_ingest_status_up", "gauge", "Whether ingest status metrics were collected successfully.")
	status, err := r.ingestStatus(ctx)
	if err != nil {
		b.WriteString("ops_api_ingest_status_up 0\n")
		return b.String()
	}
	b.WriteString("ops_api_ingest_status_up 1\n")
	writeMetricHeader(&b, "ops_api_ingest_queue_depth", "gauge", "Number of ingest events waiting or being processed.")
	fmt.Fprintf(&b, "ops_api_ingest_queue_depth %d\n", status.QueueDepth)
	writeMetricHeader(&b, "ops_api_ingest_retry_scheduled", "gauge", "Number of ingest events scheduled for retry.")
	fmt.Fprintf(&b, "ops_api_ingest_retry_scheduled %d\n", status.RetryScheduled)
	writeMetricHeader(&b, "ops_api_ingest_processing", "gauge", "Number of ingest events currently processing.")
	fmt.Fprintf(&b, "ops_api_ingest_processing %d\n", status.Processing)
	writeMetricHeader(&b, "ops_api_ingest_dead_letter", "gauge", "Number of ingest events in dead-letter state.")
	fmt.Fprintf(&b, "ops_api_ingest_dead_letter %d\n", status.DeadLetter)
	writeMetricHeader(&b, "ops_api_ingest_oldest_pending_age_seconds", "gauge", "Age of the oldest pending ingest event in seconds.")
	fmt.Fprintf(&b, "ops_api_ingest_oldest_pending_age_seconds %s\n", strconv.FormatFloat(status.OldestPendingAgeSeconds, 'f', -1, 64))
	writeMetricHeader(&b, "ops_api_ingest_oldest_retry_age_seconds", "gauge", "Lag of the oldest retryable ingest event in seconds.")
	fmt.Fprintf(&b, "ops_api_ingest_oldest_retry_age_seconds %s\n", strconv.FormatFloat(status.OldestRetryAgeSeconds, 'f', -1, 64))
	return b.String()
}

func (r *Registry) snapshot() ([]requestKey, map[requestKey]int64, []errorKey, map[errorKey]int64, map[requestKey]histogram, []float64) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	requestKeys := make([]requestKey, 0, len(r.requests))
	requestCounts := make(map[requestKey]int64, len(r.requests))
	for key := range r.requests {
		requestKeys = append(requestKeys, key)
		requestCounts[key] = r.requests[key]
	}
	sort.Slice(requestKeys, func(i, j int) bool {
		if requestKeys[i].Route == requestKeys[j].Route {
			if requestKeys[i].Method == requestKeys[j].Method {
				return requestKeys[i].Status < requestKeys[j].Status
			}
			return requestKeys[i].Method < requestKeys[j].Method
		}
		return requestKeys[i].Route < requestKeys[j].Route
	})

	errorKeys := make([]errorKey, 0, len(r.errors))
	errorCounts := make(map[errorKey]int64, len(r.errors))
	for key := range r.errors {
		errorKeys = append(errorKeys, key)
		errorCounts[key] = r.errors[key]
	}
	sort.Slice(errorKeys, func(i, j int) bool {
		if errorKeys[i].Route == errorKeys[j].Route {
			if errorKeys[i].Method == errorKeys[j].Method {
				return errorKeys[i].ErrorCode < errorKeys[j].ErrorCode
			}
			return errorKeys[i].Method < errorKeys[j].Method
		}
		return errorKeys[i].Route < errorKeys[j].Route
	})

	histograms := make(map[requestKey]histogram, len(r.histograms))
	for key, value := range r.histograms {
		histograms[key] = histogram{
			Counts: append([]int64(nil), value.Counts...),
			SumMS:  value.SumMS,
			Count:  value.Count,
		}
	}

	return requestKeys, requestCounts, errorKeys, errorCounts, histograms, append([]float64(nil), r.latencyBuckets...)
}

func (r *Registry) ingestStatus(ctx context.Context) (domain.IngestStatus, error) {
	if r.ingest == nil {
		return domain.IngestStatus{}, nil
	}
	return r.ingest.GetStatus(ctx)
}

func writeMetricHeader(b *strings.Builder, name, kind, help string) {
	fmt.Fprintf(b, "# HELP %s %s\n", name, help)
	fmt.Fprintf(b, "# TYPE %s %s\n", name, kind)
}
