package security

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/domain"
)

func defaultRules() []Rule {
	return []Rule{
		openClawPublicIPRule{},
		worldAccessibleUISourceRangesRule{},
		worldAccessibleSSHSourceRangesRule{},
		osLoginDisabledRule{},
		wgEasyPlaintextSecretRule{},
		unpinnedSecretReferenceRule{},
		defaultManagementPortRule{},
	}
}

type openClawPublicIPRule struct{}

func (openClawPublicIPRule) ID() string      { return "openclaw-public-ip-enabled" }
func (openClawPublicIPRule) Version() string { return "1.0.0" }
func (r openClawPublicIPRule) Evaluate(tfvars map[string]any) []Match {
	enabled, ok := boolValue(tfvars, "openclaw_enable_public_ip")
	if !ok || !enabled {
		return nil
	}
	return []Match{{
		RuleID:      r.ID(),
		RuleVersion: r.Version(),
		Severity:    domain.SecuritySeverityCritical,
		Title:       "OpenClaw public IP is enabled",
		Description: "OpenClaw is configured with a public IP, which weakens the VPN-only security boundary.",
		FieldPath:   "openclaw_enable_public_ip",
		MatchKey:    "public-ip-enabled",
		FixHint:     "Set openclaw_enable_public_ip to false and reach OpenClaw only through the WireGuard VPN.",
		Metadata: map[string]any{
			"value": true,
		},
	}}
}

type worldAccessibleUISourceRangesRule struct{}

func (worldAccessibleUISourceRangesRule) ID() string      { return "wgeasy-ui-exposed-to-world" }
func (worldAccessibleUISourceRangesRule) Version() string { return "1.0.0" }
func (r worldAccessibleUISourceRangesRule) Evaluate(tfvars map[string]any) []Match {
	matches := exposedCIDRFinding(r.ID(), r.Version(), "ui_source_ranges", domain.SecuritySeverityHigh, "wg-easy UI is exposed to the world", "Restrict ui_source_ranges to trusted operator CIDRs instead of 0.0.0.0/0 or ::/0.", tfvars)
	return matches
}

type worldAccessibleSSHSourceRangesRule struct{}

func (worldAccessibleSSHSourceRangesRule) ID() string      { return "ssh-exposed-to-world" }
func (worldAccessibleSSHSourceRangesRule) Version() string { return "1.0.0" }
func (r worldAccessibleSSHSourceRangesRule) Evaluate(tfvars map[string]any) []Match {
	matches := exposedCIDRFinding(r.ID(), r.Version(), "ssh_source_ranges", domain.SecuritySeverityHigh, "SSH is exposed to the world", "Restrict ssh_source_ranges to trusted operator CIDRs instead of 0.0.0.0/0 or ::/0.", tfvars)
	return matches
}

type osLoginDisabledRule struct{}

func (osLoginDisabledRule) ID() string      { return "project-oslogin-disabled" }
func (osLoginDisabledRule) Version() string { return "1.0.0" }
func (r osLoginDisabledRule) Evaluate(tfvars map[string]any) []Match {
	enabled, ok := boolValue(tfvars, "enable_project_oslogin")
	if !ok || enabled {
		return nil
	}
	return []Match{{
		RuleID:      r.ID(),
		RuleVersion: r.Version(),
		Severity:    domain.SecuritySeverityMedium,
		Title:       "Project OS Login is disabled",
		Description: "SSH access is not backed by project-level OS Login, which makes centralized access control and auditability weaker.",
		FieldPath:   "enable_project_oslogin",
		MatchKey:    "oslogin-disabled",
		FixHint:     "Enable project-level OS Login unless there is a documented exception.",
		Metadata: map[string]any{
			"value": false,
		},
	}}
}

type wgEasyPlaintextSecretRule struct{}

func (wgEasyPlaintextSecretRule) ID() string      { return "wgeasy-password-secret-used" }
func (wgEasyPlaintextSecretRule) Version() string { return "1.0.0" }
func (r wgEasyPlaintextSecretRule) Evaluate(tfvars map[string]any) []Match {
	if !hasNonEmptyString(tfvars, "wgeasy_password_secret") {
		return nil
	}
	return []Match{{
		RuleID:      r.ID(),
		RuleVersion: r.Version(),
		Severity:    domain.SecuritySeverityMedium,
		Title:       "wg-easy uses a plaintext password secret",
		Description: "A plaintext password secret is configured for wg-easy. Storing a password hash secret reduces blast radius if the runtime value is exposed.",
		FieldPath:   "wgeasy_password_secret",
		MatchKey:    "plaintext-secret-used",
		FixHint:     "Prefer wgeasy_password_hash_secret over wgeasy_password_secret when possible.",
		Metadata: map[string]any{
			"value_redacted": "[REDACTED]",
		},
	}}
}

type unpinnedSecretReferenceRule struct{}

func (unpinnedSecretReferenceRule) ID() string      { return "secret-reference-not-version-pinned" }
func (unpinnedSecretReferenceRule) Version() string { return "1.0.0" }
func (r unpinnedSecretReferenceRule) Evaluate(tfvars map[string]any) []Match {
	fields := []string{
		"wgeasy_password_secret",
		"wgeasy_password_hash_secret",
		"openclaw_gateway_password_secret",
		"openclaw_anthropic_api_key_secret",
		"openclaw_openai_api_key_secret",
		"openclaw_telegram_bot_token_secret",
	}

	findings := make([]Match, 0)
	for _, field := range fields {
		value, ok := stringValue(tfvars, field)
		if !ok || value == "" {
			continue
		}
		match := secretRefPattern.FindStringSubmatch(value)
		if len(match) == 0 {
			continue
		}
		if match[2] != "" && match[2] != "latest" {
			continue
		}
		findings = append(findings, Match{
			RuleID:      r.ID(),
			RuleVersion: r.Version(),
			Severity:    domain.SecuritySeverityInfo,
			Title:       fmt.Sprintf("Secret reference is not pinned for %s", field),
			Description: "The Secret Manager reference does not pin to a specific version, which can introduce silent configuration drift.",
			FieldPath:   field,
			MatchKey:    field,
			FixHint:     "Use a Secret Manager reference with an explicit /versions/<number> suffix.",
			Metadata: map[string]any{
				"reference_redacted": redactSecretRef(value),
			},
		})
	}
	return findings
}

type defaultManagementPortRule struct{}

func (defaultManagementPortRule) ID() string      { return "default-management-port-retained" }
func (defaultManagementPortRule) Version() string { return "1.0.0" }
func (r defaultManagementPortRule) Evaluate(tfvars map[string]any) []Match {
	defaults := []struct {
		field    string
		expected float64
	}{
		{field: "openclaw_gateway_port", expected: 18789},
		{field: "wg_port", expected: 51820},
		{field: "wgeasy_ui_port", expected: 51821},
	}

	findings := make([]Match, 0)
	for _, item := range defaults {
		actual, ok := numberValue(tfvars, item.field)
		if !ok || actual != item.expected {
			continue
		}
		findings = append(findings, Match{
			RuleID:      r.ID(),
			RuleVersion: r.Version(),
			Severity:    domain.SecuritySeverityInfo,
			Title:       fmt.Sprintf("Default management port retained for %s", item.field),
			Description: "A default management or service port is retained. Defaults are easier to fingerprint during scanning and are worth reviewing for your environment.",
			FieldPath:   item.field,
			MatchKey:    item.field,
			FixHint:     "Review whether keeping the default port is intentional for this deployment.",
			Metadata: map[string]any{
				"port": int(actual),
			},
		})
	}
	return findings
}

func exposedCIDRFinding(ruleID, ruleVersion, field string, severity domain.SecuritySeverity, title, hint string, tfvars map[string]any) []Match {
	cidrs, ok := stringSliceValue(tfvars, field)
	if !ok {
		return nil
	}

	findings := make([]Match, 0)
	for _, cidr := range cidrs {
		if !isWorldCIDR(cidr) {
			continue
		}
		findings = append(findings, Match{
			RuleID:      ruleID,
			RuleVersion: ruleVersion,
			Severity:    severity,
			Title:       title,
			Description: fmt.Sprintf("%s includes %s, allowing unrestricted inbound access.", field, cidr),
			FieldPath:   field,
			MatchKey:    cidr,
			FixHint:     hint,
			Metadata: map[string]any{
				"cidr": cidr,
			},
		})
	}
	return findings
}

func boolValue(tfvars map[string]any, field string) (bool, bool) {
	raw, ok := tfvars[field]
	if !ok {
		return false, false
	}
	value, ok := raw.(bool)
	return value, ok
}

func stringValue(tfvars map[string]any, field string) (string, bool) {
	raw, ok := tfvars[field]
	if !ok {
		return "", false
	}
	value, ok := raw.(string)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(value), true
}

func hasNonEmptyString(tfvars map[string]any, field string) bool {
	value, ok := stringValue(tfvars, field)
	return ok && value != ""
}

func stringSliceValue(tfvars map[string]any, field string) ([]string, bool) {
	raw, ok := tfvars[field]
	if !ok {
		return nil, false
	}

	switch value := raw.(type) {
	case []string:
		return trimStrings(value), true
	case []any:
		out := make([]string, 0, len(value))
		for _, item := range value {
			str, ok := item.(string)
			if !ok {
				return nil, false
			}
			trimmed := strings.TrimSpace(str)
			if trimmed == "" {
				continue
			}
			out = append(out, trimmed)
		}
		return out, true
	default:
		return nil, false
	}
}

func numberValue(tfvars map[string]any, field string) (float64, bool) {
	raw, ok := tfvars[field]
	if !ok {
		return 0, false
	}
	switch value := raw.(type) {
	case float64:
		return value, true
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	default:
		return 0, false
	}
}

func isWorldCIDR(cidr string) bool {
	return slices.Contains([]string{"0.0.0.0/0", "::/0"}, strings.TrimSpace(cidr))
}

var secretRefPattern = regexp.MustCompile(`^projects/[^/]+/secrets/([^/]+)(?:/versions/([^/]+))?$`)

func redactSecretRef(value string) string {
	match := secretRefPattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(match) == 0 {
		return "[REDACTED]"
	}

	version := "latest-or-unpinned"
	if match[2] != "" {
		version = match[2]
	}
	return fmt.Sprintf("projects/***/secrets/%s/versions/%s", maskIdentifier(match[1]), maskIdentifier(version))
}

func maskIdentifier(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "***"
	}
	if len(value) <= 4 {
		return "***"
	}
	return value[:2] + "***" + value[len(value)-2:]
}
