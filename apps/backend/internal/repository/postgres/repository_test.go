package postgres

import (
	"errors"
	"testing"
	"time"

	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/repository"
)

func TestBucketExpressionUsesUTC(t *testing.T) {
	tests := []struct {
		name   string
		bucket string
		want   string
	}{
		{
			name:   "1m",
			bucket: "1m",
			want:   "date_trunc('minute', (created_at AT TIME ZONE 'UTC')) AT TIME ZONE 'UTC'",
		},
		{
			name:   "5m",
			bucket: "5m",
			want:   "((date_trunc('hour', (created_at AT TIME ZONE 'UTC')) + (floor(date_part('minute', (created_at AT TIME ZONE 'UTC')) / 5) * interval '5 minutes')) AT TIME ZONE 'UTC')",
		},
		{
			name:   "1h",
			bucket: "1h",
			want:   "date_trunc('hour', (created_at AT TIME ZONE 'UTC')) AT TIME ZONE 'UTC'",
		},
		{
			name:   "day",
			bucket: "day",
			want:   "date_trunc('day', (created_at AT TIME ZONE 'UTC')) AT TIME ZONE 'UTC'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := bucketExpression(tt.bucket)
			if !ok {
				t.Fatalf("bucketExpression(%q) returned ok=false", tt.bucket)
			}
			if got != tt.want {
				t.Fatalf("bucketExpression(%q) = %q, want %q", tt.bucket, got, tt.want)
			}
		})
	}
}

func TestBucketExpressionRejectsUnknownBucket(t *testing.T) {
	if expr, ok := bucketExpression("unknown"); ok || expr != "" {
		t.Fatalf("bucketExpression returned (%q, %t), want (\"\", false)", expr, ok)
	}
}

func TestGetTimeseriesRejectsUnsupportedMetric(t *testing.T) {
	store := &Store{}

	_, err := store.GetTimeseries(t.Context(), "invalid", "1h", tZero(), tZero())
	if !errors.Is(err, repository.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestGetTimeseriesRejectsUnsupportedBucket(t *testing.T) {
	store := &Store{}

	_, err := store.GetTimeseries(t.Context(), "requests", "invalid", tZero(), tZero())
	if !errors.Is(err, repository.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func tZero() time.Time {
	return time.Time{}
}
