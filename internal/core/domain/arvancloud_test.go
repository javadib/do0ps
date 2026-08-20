package domain

import "testing"

// TestArvanCloudPlanLevel checks the plan enum against the spec's Plan schema
// (0..4): a plan is sent and reported as an integer, so an off-by-one here
// would silently order the wrong subscription.
func TestArvanCloudPlanLevel(t *testing.T) {
	tests := []struct {
		plan  ArvanCloudPlanLevel
		value int
		name  string
	}{
		{ArvanCloudPlanTraffic, 0, "traffic"},
		{ArvanCloudPlanBasic, 1, "basic"},
		{ArvanCloudPlanGrowth, 2, "growth"},
		{ArvanCloudPlanProfessional, 3, "professional"},
		{ArvanCloudPlanEnterprise, 4, "enterprise"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if int(tc.plan) != tc.value {
				t.Errorf("plan value = %d, want %d", int(tc.plan), tc.value)
			}
			if got := tc.plan.String(); got != tc.name {
				t.Errorf("String() = %q, want %q", got, tc.name)
			}
			if !tc.plan.Valid() {
				t.Error("Valid() = false, want true")
			}
		})
	}

	for _, plan := range []ArvanCloudPlanLevel{-1, 5} {
		if plan.Valid() {
			t.Errorf("ArvanCloudPlanLevel(%d).Valid() = true, want false", plan)
		}
		if got := plan.String(); got != "unknown" {
			t.Errorf("ArvanCloudPlanLevel(%d).String() = %q, want %q", plan, got, "unknown")
		}
	}
}

// TestArvanCloudDomainEnums pins the type and status enums the Domain schema
// declares, so a value the API never sends is not accepted as valid input.
func TestArvanCloudDomainEnums(t *testing.T) {
	for _, s := range []string{ArvanCloudDomainTypeFull, ArvanCloudDomainTypePartial} {
		if !ValidArvanCloudDomainType(s) {
			t.Errorf("ValidArvanCloudDomainType(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"", "Full", "cname", "ns"} {
		if ValidArvanCloudDomainType(s) {
			t.Errorf("ValidArvanCloudDomainType(%q) = true, want false", s)
		}
	}

	statuses := []string{
		ArvanCloudDomainStatusInitializing,
		ArvanCloudDomainStatusPending,
		ArvanCloudDomainStatusActive,
		ArvanCloudDomainStatusMoved,
	}
	for _, s := range statuses {
		if !ValidArvanCloudDomainStatus(s) {
			t.Errorf("ValidArvanCloudDomainStatus(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"", "Active", "deleted", "suspended"} {
		if ValidArvanCloudDomainStatus(s) {
			t.Errorf("ValidArvanCloudDomainStatus(%q) = true, want false", s)
		}
	}
}
