package telemetry

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type Span struct {
	TraceID      string
	SpanID       string
	ParentSpanID string
	Name         string
	ServiceName  string
	StartTime    time.Time
	EndTime      time.Time
	Attributes   map[string]any
}

type Exporter interface {
	ExportSpan(ctx context.Context, span Span) error
}

type Tracer struct {
	exporter    Exporter
	sampleRate  float64
	serviceName string
}

type TraceContext struct {
	TraceID      string
	SpanID       string
	ParentSpanID string
	Sampled      bool
}

type nopExporter struct{}

type slogExporter struct {
	logger *slog.Logger
}

func NewTracer(exporter Exporter, sampleRate float64, serviceName string) *Tracer {
	if exporter == nil {
		exporter = nopExporter{}
	}
	return &Tracer{
		exporter:    exporter,
		sampleRate:  sampleRate,
		serviceName: serviceName,
	}
}

func NewSlogExporter(logger *slog.Logger) Exporter {
	return slogExporter{logger: logger}
}

func (nopExporter) ExportSpan(context.Context, Span) error { return nil }

func (e slogExporter) ExportSpan(_ context.Context, span Span) error {
	if e.logger == nil {
		return nil
	}
	e.logger.Info("trace span",
		"trace_id", span.TraceID,
		"span_id", span.SpanID,
		"parent_span_id", span.ParentSpanID,
		"name", span.Name,
		"service_name", span.ServiceName,
		"started_at", span.StartTime.UTC(),
		"ended_at", span.EndTime.UTC(),
		"attributes", span.Attributes,
	)
	return nil
}

func (t *Tracer) Start(r *http.Request) TraceContext {
	traceID, parentSpanID, sampled := parseTraceparent(r.Header.Get("Traceparent"))
	if traceID == "" {
		traceID = randomHex(16)
	}
	spanID := randomHex(8)
	if !sampled {
		sampled = shouldSample(traceID, t.sampleRate)
	}
	return TraceContext{
		TraceID:      traceID,
		SpanID:       spanID,
		ParentSpanID: parentSpanID,
		Sampled:      sampled,
	}
}

func (t *Tracer) Finish(ctx context.Context, traceCtx TraceContext, name string, startedAt, endedAt time.Time, attrs map[string]any) {
	if !traceCtx.Sampled {
		return
	}
	_ = t.exporter.ExportSpan(ctx, Span{
		TraceID:      traceCtx.TraceID,
		SpanID:       traceCtx.SpanID,
		ParentSpanID: traceCtx.ParentSpanID,
		Name:         name,
		ServiceName:  t.serviceName,
		StartTime:    startedAt.UTC(),
		EndTime:      endedAt.UTC(),
		Attributes:   attrs,
	})
}

func FormatTraceparent(traceCtx TraceContext) string {
	if traceCtx.TraceID == "" || traceCtx.SpanID == "" {
		return ""
	}
	flags := "00"
	if traceCtx.Sampled {
		flags = "01"
	}
	return fmt.Sprintf("00-%s-%s-%s", traceCtx.TraceID, traceCtx.SpanID, flags)
}

func parseTraceparent(raw string) (traceID, parentSpanID string, sampled bool) {
	parts := strings.Split(strings.TrimSpace(raw), "-")
	if len(parts) != 4 {
		return "", "", false
	}
	traceID = parts[1]
	parentSpanID = parts[2]
	sampled = parts[3] == "01"
	if len(traceID) != 32 || len(parentSpanID) != 16 {
		return "", "", false
	}
	return traceID, parentSpanID, sampled
}

func shouldSample(traceID string, sampleRate float64) bool {
	if sampleRate >= 1 {
		return true
	}
	if sampleRate <= 0 {
		return false
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(traceID))
	return float64(h.Sum32()%10000)/10000 <= sampleRate
}

func randomHex(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		now := time.Now().UnixNano()
		fallback := make([]byte, size)
		for i := range fallback {
			fallback[i] = byte(now >> (uint(i%8) * 8))
		}
		return hex.EncodeToString(fallback)
	}
	return hex.EncodeToString(buf)
}
