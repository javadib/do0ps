package domain

import "testing"

func TestValidArvanCloudWafMode(t *testing.T) {
	for _, tt := range []struct {
		value string
		want  bool
	}{
		{"off", true},
		{"detect", true},
		{"protect", true},
		{"false", false}, // NOT a valid value — see ArvanCloudWafMode's doc comment on the wire-encoding finding
		{"Off", false},
		{"", false},
	} {
		if got := ValidArvanCloudWafMode(tt.value); got != tt.want {
			t.Errorf("ValidArvanCloudWafMode(%q) = %v, want %v", tt.value, got, tt.want)
		}
	}
}

func TestValidArvanCloudWafLogRedactionReplacement(t *testing.T) {
	for _, tt := range []struct {
		value string
		want  bool
	}{
		{"****", true},
		{"####", true},
		{"--REDACTED--", true},
		{"", true}, // deliberately valid — see the type's doc comment
		{"REDACTED", false},
		{"***", false},
	} {
		if got := ValidArvanCloudWafLogRedactionReplacement(tt.value); got != tt.want {
			t.Errorf("ValidArvanCloudWafLogRedactionReplacement(%q) = %v, want %v", tt.value, got, tt.want)
		}
	}
}

// TestValidArvanCloudWafRuleAction is the acceptance-criteria test from issue
// #66: WafRuleAction must accept exactly "protect" and "passthrough" and
// reject every other value, including the CDN edge Firewall's own action
// values (which must not leak into this narrower enum — see
// ArvanCloudWafRuleAction's doc comment).
func TestValidArvanCloudWafRuleAction(t *testing.T) {
	for _, tt := range []struct {
		value string
		want  bool
	}{
		{"protect", true},
		{"passthrough", true},
		{"allow", false},
		{"deny", false},
		{"bypass", false},
		{"challenge", false},
		{"drop", false},
		{"Protect", false},
		{"", false},
	} {
		if got := ValidArvanCloudWafRuleAction(tt.value); got != tt.want {
			t.Errorf("ValidArvanCloudWafRuleAction(%q) = %v, want %v", tt.value, got, tt.want)
		}
	}
}
