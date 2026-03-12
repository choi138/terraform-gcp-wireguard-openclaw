package security

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/domain"
	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/repository/memory"
)

func TestAnalyzeProducesDeterministicRedactedFindings(t *testing.T) {
	store := memory.NewStore()
	service := NewService(store)
	fixedNow := time.Date(2026, 3, 11, 9, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }

	input := domain.SecurityAnalysisInput{
		SchemaVersion: domain.SupportedSecurityAnalysisSchemaVersion,
		Tfvars:        insecureTfvarsFixture(),
	}

	first, err := service.Analyze(context.Background(), input)
	if err != nil {
		t.Fatalf("expected analysis to succeed, got %v", err)
	}
	second, err := service.Analyze(context.Background(), input)
	if err != nil {
		t.Fatalf("expected second analysis to succeed, got %v", err)
	}

	if len(first.Findings) < 6 {
		t.Fatalf("expected at least 6 findings, got %d", len(first.Findings))
	}
	if len(first.Findings) != len(second.Findings) {
		t.Fatalf("expected stable finding count, got %d and %d", len(first.Findings), len(second.Findings))
	}

	firstFingerprints := make([]string, 0, len(first.Findings))
	secondFingerprints := make([]string, 0, len(second.Findings))
	for i, finding := range first.Findings {
		if !domain.IsAllowedSecuritySeverity(string(finding.Severity)) {
			t.Fatalf("unexpected severity %q", finding.Severity)
		}
		if finding.Status != domain.SecurityFindingStatusOpen {
			t.Fatalf("expected default status open, got %q", finding.Status)
		}
		firstFingerprints = append(firstFingerprints, finding.Fingerprint)
		secondFingerprints = append(secondFingerprints, second.Findings[i].Fingerprint)
	}
	if !slices.Equal(firstFingerprints, secondFingerprints) {
		t.Fatalf("expected deterministic fingerprints, got %v and %v", firstFingerprints, secondFingerprints)
	}

	encoded, err := json.Marshal(first.Findings)
	if err != nil {
		t.Fatalf("expected findings to marshal, got %v", err)
	}
	output := string(encoded)
	rawSecretRef := "projects/demo/secrets/plain-openclaw-password/versions/latest"
	if strings.Contains(output, rawSecretRef) {
		t.Fatalf("expected findings to redact raw secret reference, got %s", output)
	}
}

func TestListFindingsFiltersBySeverityAndStatus(t *testing.T) {
	store := memory.NewStore()
	service := NewService(store)

	if _, err := service.Analyze(context.Background(), domain.SecurityAnalysisInput{
		SchemaVersion: domain.SupportedSecurityAnalysisSchemaVersion,
		Tfvars:        insecureTfvarsFixture(),
	}); err != nil {
		t.Fatalf("expected analysis to succeed, got %v", err)
	}

	findings, err := service.ListFindings(context.Background(), domain.SecurityFindingFilter{
		Statuses:   []domain.SecurityFindingStatus{domain.SecurityFindingStatusOpen},
		Severities: []domain.SecuritySeverity{domain.SecuritySeverityCritical},
		Pagination: domain.Pagination{Page: 1, PageSize: 20},
		Order:      "desc",
	})
	if err != nil {
		t.Fatalf("expected list to succeed, got %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("expected at least one critical finding")
	}
	for _, finding := range findings {
		if finding.Status != domain.SecurityFindingStatusOpen {
			t.Fatalf("expected open status, got %q", finding.Status)
		}
		if finding.Severity != domain.SecuritySeverityCritical {
			t.Fatalf("expected critical severity, got %q", finding.Severity)
		}
	}
}

func TestComputeFingerprintIgnoresMutablePresentationFields(t *testing.T) {
	base := Match{
		RuleID:      "ssh-exposed-to-world",
		RuleVersion: "1.0.0",
		FieldPath:   "ssh_source_ranges",
		MatchKey:    "0.0.0.0/0",
		Title:       "SSH exposed",
		Description: "first description",
		Metadata: map[string]any{
			"cidr": "0.0.0.0/0",
		},
	}

	mutated := base
	mutated.RuleVersion = "2.0.0"
	mutated.Title = "SSH is exposed to the world"
	mutated.Description = "second description"
	mutated.Metadata = map[string]any{
		"cidr":   "0.0.0.0/0",
		"notice": "changed copy",
	}

	if got, want := computeFingerprint(mutated), computeFingerprint(base); got != want {
		t.Fatalf("expected fingerprint to remain stable, got %q want %q", got, want)
	}
}

func TestOSLoginRuleIgnoresMissingTfvarsKey(t *testing.T) {
	findings := osLoginDisabledRule{}.Evaluate(map[string]any{})
	if len(findings) != 0 {
		t.Fatalf("expected missing enable_project_oslogin to be treated as unknown, got %+v", findings)
	}
}

func TestUnpinnedSecretRuleIgnoresNonSecretManagerValues(t *testing.T) {
	findings := unpinnedSecretReferenceRule{}.Evaluate(map[string]any{
		"wgeasy_password_secret": "plain-text-placeholder",
	})
	if len(findings) != 0 {
		t.Fatalf("expected non Secret Manager values to be ignored, got %+v", findings)
	}
}

func insecureTfvarsFixture() map[string]any {
	return map[string]any{
		"openclaw_enable_public_ip":      true,
		"ui_source_ranges":               []any{"0.0.0.0/0"},
		"ssh_source_ranges":              []any{"0.0.0.0/0"},
		"enable_project_oslogin":         false,
		"wgeasy_password_secret":         "projects/demo/secrets/plain-openclaw-password/versions/latest",
		"openclaw_openai_api_key_secret": "projects/demo/secrets/openai-api-token/versions/latest",
		"wg_port":                        51820,
		"wgeasy_ui_port":                 51821,
		"openclaw_gateway_port":          18789,
	}
}
