package domain_test

import (
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

// TestValidArvanCloudCertificatePrivateKeySize pins the spec's fixed
// private_key_size enum (2048, 4096) — CertificateOrderIssueRequest, issue
// #74's explicit acceptance criterion.
func TestValidArvanCloudCertificatePrivateKeySize(t *testing.T) {
	tests := []struct {
		size int
		want bool
	}{
		{2048, true},
		{4096, true},
		{1024, false},
		{0, false},
		{-1, false},
	}
	for _, tt := range tests {
		if got := domain.ValidArvanCloudCertificatePrivateKeySize(tt.size); got != tt.want {
			t.Errorf("ValidArvanCloudCertificatePrivateKeySize(%d) = %v, want %v", tt.size, got, tt.want)
		}
	}
}

// TestArvanCloudAccountCertificateOrderTerminal pins the two inferred
// terminal status values (see ArvanCloudAccountCertificateOrderStatus's own
// doc comment for why "inferred") and proves every other status is treated
// as still in flight.
func TestArvanCloudAccountCertificateOrderTerminal(t *testing.T) {
	tests := []struct {
		status domain.ArvanCloudAccountCertificateOrderStatus
		want   bool
	}{
		{domain.ArvanCloudAccountCertificateOrderStatusValid, true},
		{domain.ArvanCloudAccountCertificateOrderStatusKilled, true},
		{"pending", false},
		{"processing", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := domain.ArvanCloudAccountCertificateOrderTerminal(tt.status); got != tt.want {
			t.Errorf("ArvanCloudAccountCertificateOrderTerminal(%q) = %v, want %v", tt.status, got, tt.want)
		}
	}
}
