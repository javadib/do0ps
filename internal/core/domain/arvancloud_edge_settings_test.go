package domain

import "testing"

// TestValidArvanCloudCustomPageStatusCodeRejects404 is the acceptance-
// criteria test from issue #72: CustomPageStatusCode must accept exactly the
// spec's 8-value fixed set and reject every other integer status code,
// including 404 — an otherwise unremarkable HTTP status that the spec's own
// enum genuinely omits (confirmed directly against
// docs/api-specs/arvancloud-cdn-4.0.yml, not "fixed" by adding it here).
func TestValidArvanCloudCustomPageStatusCodeRejects404(t *testing.T) {
	for _, tt := range []struct {
		code ArvanCloudCustomPageStatusCode
		want bool
	}{
		{200, true},
		{302, true},
		{481, true},
		{403, true},
		{482, true},
		{483, true},
		{484, true},
		{500, true},
		{0, true}, // optional/unset
		{404, false},
		{401, false},
		{503, false},
		{201, false},
	} {
		if got := ValidArvanCloudCustomPageStatusCode(tt.code); got != tt.want {
			t.Errorf("ValidArvanCloudCustomPageStatusCode(%d) = %v, want %v", tt.code, got, tt.want)
		}
	}
}

func TestValidArvanCloudCustomPageType(t *testing.T) {
	for _, tt := range []struct {
		value string
		want  bool
	}{
		{"off", true},
		{"url", true},
		{"file", true},
		{"", true}, // optional on read
		{"false", false},
		{"disabled", false},
		{"Off", false},
	} {
		if got := ValidArvanCloudCustomPageType(tt.value); got != tt.want {
			t.Errorf("ValidArvanCloudCustomPageType(%q) = %v, want %v", tt.value, got, tt.want)
		}
	}
}

func TestValidArvanCloudCustomPageName(t *testing.T) {
	for _, tt := range []struct {
		value string
		want  bool
	}{
		{"under_construction", true},
		{"firewall_error", true},
		{"waf_protection", true},
		{"rate_limit_exceeded", true},
		{"secure_link_expired", true},
		{"secure_link_invalid", true},
		{"error_500", true},
		{"ddos_js", true},
		{"ddos_captcha", true},
		{"", false},
		{"not_a_page", false},
	} {
		if got := ValidArvanCloudCustomPageName(tt.value); got != tt.want {
			t.Errorf("ValidArvanCloudCustomPageName(%q) = %v, want %v", tt.value, got, tt.want)
		}
	}
}

func TestValidArvanCloudCachePurgeMode(t *testing.T) {
	for _, tt := range []struct {
		value string
		want  bool
	}{
		{"all", true},
		{"individual", true},
		{"tags", true},
		{"", false},
		{"everything", false},
	} {
		if got := ValidArvanCloudCachePurgeMode(tt.value); got != tt.want {
			t.Errorf("ValidArvanCloudCachePurgeMode(%q) = %v, want %v", tt.value, got, tt.want)
		}
	}
}

func TestValidArvanCloudImageResizeStatus(t *testing.T) {
	for _, tt := range []struct {
		value string
		want  bool
	}{
		{"on", true},
		{"off", true},
		{"", true},         // optional, provider default applies
		{"inherit", false}, // valid for PageRuleImageResize, NOT for the base ImageResize schema
		{"On", false},
	} {
		if got := ValidArvanCloudImageResizeStatus(tt.value); got != tt.want {
			t.Errorf("ValidArvanCloudImageResizeStatus(%q) = %v, want %v", tt.value, got, tt.want)
		}
	}
}
