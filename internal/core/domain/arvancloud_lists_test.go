package domain

import "testing"

func TestValidArvanCloudDynamicFieldType(t *testing.T) {
	for _, tt := range []struct {
		value string
		want  bool
	}{
		{"ip", true},
		{"number", true},
		{"byte", true},
		{"bytes", false}, // the list-filter query param's enum typo, not a real type
		{"IP", false},
		{"", false},
		{"string", false},
	} {
		if got := ValidArvanCloudDynamicFieldType(tt.value); got != tt.want {
			t.Errorf("ValidArvanCloudDynamicFieldType(%q) = %v, want %v", tt.value, got, tt.want)
		}
	}
}

func TestValidArvanCloudDynamicFieldScope(t *testing.T) {
	for _, tt := range []struct {
		value string
		want  bool
	}{
		{"public", true},
		{"private", true},
		{"Public", false},
		{"", false},
	} {
		if got := ValidArvanCloudDynamicFieldScope(tt.value); got != tt.want {
			t.Errorf("ValidArvanCloudDynamicFieldScope(%q) = %v, want %v", tt.value, got, tt.want)
		}
	}
}
