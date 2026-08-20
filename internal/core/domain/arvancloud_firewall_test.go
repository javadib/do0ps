package domain

import "testing"

func TestValidArvanCloudFirewallAction(t *testing.T) {
	for _, tt := range []struct {
		value string
		want  bool
	}{
		{"allow", true},
		{"deny", true},
		{"bypass", true},
		{"challenge", true},
		{"drop", false}, // valid for ArvanCloudFirewallDefaultAction, not here
		{"Allow", false},
		{"", false},
	} {
		if got := ValidArvanCloudFirewallAction(tt.value); got != tt.want {
			t.Errorf("ValidArvanCloudFirewallAction(%q) = %v, want %v", tt.value, got, tt.want)
		}
	}
}

func TestValidArvanCloudFirewallDefaultAction(t *testing.T) {
	for _, tt := range []struct {
		value string
		want  bool
	}{
		{"allow", true},
		{"deny", true},
		{"drop", true}, // the extra value over ArvanCloudFirewallAction
		{"bypass", true},
		{"challenge", true},
		{"Drop", false},
		{"", false},
	} {
		if got := ValidArvanCloudFirewallDefaultAction(tt.value); got != tt.want {
			t.Errorf("ValidArvanCloudFirewallDefaultAction(%q) = %v, want %v", tt.value, got, tt.want)
		}
	}
}

// TestFirewallActionEnumsValidateIndependently is the acceptance-criteria
// test from issue #65: the two action enums must not leak into each other —
// "drop" is rejected as a per-rule action but accepted as a settings
// default_action, and this must hold in the same test run so a shared bug
// (e.g. both validators pointing at the same underlying slice) cannot pass
// each half in isolation.
func TestFirewallActionEnumsValidateIndependently(t *testing.T) {
	if ValidArvanCloudFirewallAction("drop") {
		t.Error(`ValidArvanCloudFirewallAction("drop") = true, want false — "drop" is only valid as a default_action`)
	}
	if !ValidArvanCloudFirewallDefaultAction("drop") {
		t.Error(`ValidArvanCloudFirewallDefaultAction("drop") = false, want true`)
	}

	// Every other value must still be accepted by both, unaffected by the one
	// asymmetric value.
	for _, action := range []string{"allow", "deny", "bypass", "challenge"} {
		if !ValidArvanCloudFirewallAction(action) {
			t.Errorf("ValidArvanCloudFirewallAction(%q) = false, want true", action)
		}
		if !ValidArvanCloudFirewallDefaultAction(action) {
			t.Errorf("ValidArvanCloudFirewallDefaultAction(%q) = false, want true", action)
		}
	}
}

func TestValidArvanCloudDomainSelectionType(t *testing.T) {
	for _, tt := range []struct {
		value string
		want  bool
	}{
		{"all", true},
		{"include", true},
		{"exclude", true},
		{"All", false},
		{"", false},
		{"only", false},
	} {
		if got := ValidArvanCloudDomainSelectionType(tt.value); got != tt.want {
			t.Errorf("ValidArvanCloudDomainSelectionType(%q) = %v, want %v", tt.value, got, tt.want)
		}
	}
}
