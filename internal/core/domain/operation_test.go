package domain_test

import (
	"errors"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

func TestOperationClassString(t *testing.T) {
	tests := []struct {
		class domain.OperationClass
		want  string
	}{
		{domain.OperationClassFast, "fast"},
		{domain.OperationClassLong, "long"},
		{domain.OperationClassUnknown, "unknown"},
		{domain.OperationClass(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.class.String(); got != tt.want {
			t.Errorf("OperationClass(%d).String() = %q, want %q", tt.class, got, tt.want)
		}
	}
}

func TestParseOperationClass(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    domain.OperationClass
		wantErr bool
	}{
		{"fast", "fast", domain.OperationClassFast, false},
		{"long", "long", domain.OperationClassLong, false},
		{"empty", "", domain.OperationClassUnknown, true},
		{"unknown value", "medium", domain.OperationClassUnknown, true},
		{"wrong case", "Fast", domain.OperationClassUnknown, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.ParseOperationClass(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseOperationClass(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("ParseOperationClass(%q) error = %v, want wrapping domain.ErrInvalidInput", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseOperationClass(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// Parse and String must round-trip for every class the parser accepts, so a
// class persisted to the job store can always be read back unchanged.
func TestOperationClassRoundTrip(t *testing.T) {
	for _, class := range []domain.OperationClass{domain.OperationClassFast, domain.OperationClassLong} {
		parsed, err := domain.ParseOperationClass(class.String())
		if err != nil {
			t.Fatalf("ParseOperationClass(%q): %v", class.String(), err)
		}
		if parsed != class {
			t.Errorf("round trip of %v produced %v", class, parsed)
		}
	}
}
