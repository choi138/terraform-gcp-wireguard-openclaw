package handler

import (
	"net/http/httptest"
	"testing"
)

func TestNewPanicsOnMissingDependencies(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for missing dependencies")
		}
	}()

	New(Dependencies{}, nil)
}

func TestParseTimeRangeExpandsDateOnlyUpperBound(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/dashboard/summary?from=2026-03-10&to=2026-03-10", nil)

	from, to, err := parseTimeRange(req, true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got := from.Format("2006-01-02T15:04:05.999999999Z07:00"); got != "2026-03-10T00:00:00Z" {
		t.Fatalf("unexpected from value %s", got)
	}
	if got := to.Format("2006-01-02T15:04:05.999999999Z07:00"); got != "2026-03-10T23:59:59.999999999Z" {
		t.Fatalf("unexpected to value %s", got)
	}
}

func TestParsePathInt64RejectsNonPositiveValues(t *testing.T) {
	tests := []string{"0", "-1"}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := parsePathInt64(input)
			if err == nil {
				t.Fatalf("expected error for %q", input)
			}
		})
	}
}
