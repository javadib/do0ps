package domain_test

import (
	"errors"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

// TestArvanCloudDNSRecordTypeRoundTrip proves String()/ParseArvanCloudDNSRecordType
// round-trip for all 13 record types the CDN API accepts.
func TestArvanCloudDNSRecordTypeRoundTrip(t *testing.T) {
	types := []domain.ArvanCloudDNSRecordType{
		domain.ArvanCloudDNSRecordTypeA, domain.ArvanCloudDNSRecordTypeAAAA, domain.ArvanCloudDNSRecordTypeCNAME,
		domain.ArvanCloudDNSRecordTypeANAME, domain.ArvanCloudDNSRecordTypeMX, domain.ArvanCloudDNSRecordTypeSRV,
		domain.ArvanCloudDNSRecordTypeTXT, domain.ArvanCloudDNSRecordTypeSPF, domain.ArvanCloudDNSRecordTypeDKIM,
		domain.ArvanCloudDNSRecordTypeNS, domain.ArvanCloudDNSRecordTypePTR, domain.ArvanCloudDNSRecordTypeTLSA,
		domain.ArvanCloudDNSRecordTypeCAA,
	}
	if len(types) != 13 {
		t.Fatalf("len(types) = %d, want 13", len(types))
	}

	for _, want := range types {
		t.Run(want.String(), func(t *testing.T) {
			got, err := domain.ParseArvanCloudDNSRecordType(want.String())
			if err != nil {
				t.Fatalf("ParseArvanCloudDNSRecordType(%q) error = %v", want.String(), err)
			}
			if got != want {
				t.Errorf("ParseArvanCloudDNSRecordType(%q) = %v, want %v", want.String(), got, want)
			}

			// Case-insensitive: an uppercase form (as a chatbot might send)
			// must parse identically.
			upper, err := domain.ParseArvanCloudDNSRecordType(upperCase(want.String()))
			if err != nil || upper != want {
				t.Errorf("ParseArvanCloudDNSRecordType(uppercase) = %v, err = %v, want %v, nil", upper, err, want)
			}
		})
	}
}

func upperCase(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - ('a' - 'A')
		}
	}
	return string(b)
}

// TestParseArvanCloudDNSRecordTypeRejectsUnknown is the AC #63 pin: a record
// type outside the confirmed 13-value set must fail fast with
// domain.ErrInvalidInput.
func TestParseArvanCloudDNSRecordTypeRejectsUnknown(t *testing.T) {
	for _, s := range []string{"", "bogus", "A ", "cname;drop table"} {
		t.Run(s, func(t *testing.T) {
			got, err := domain.ParseArvanCloudDNSRecordType(s)
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("ParseArvanCloudDNSRecordType(%q) error = %v, want domain.ErrInvalidInput", s, err)
			}
			if got != domain.ArvanCloudDNSRecordTypeUnknown {
				t.Errorf("ParseArvanCloudDNSRecordType(%q) = %v, want ArvanCloudDNSRecordTypeUnknown", s, got)
			}
		})
	}
}

// TestValidArvanCloudDNSRecordTTL is the AC #63 pin: the TTL enum validator
// must accept every one of the CDN API's fixed values and reject an
// arbitrary one like 121.
func TestValidArvanCloudDNSRecordTTL(t *testing.T) {
	valid := []int{120, 180, 300, 600, 900, 1800, 3600, 7200, 18000, 43200, 86400, 172800, 432000}
	for _, ttl := range valid {
		if !domain.ValidArvanCloudDNSRecordTTL(ttl) {
			t.Errorf("ValidArvanCloudDNSRecordTTL(%d) = false, want true", ttl)
		}
	}

	invalid := []int{0, 1, 60, 121, 3601, 999999, -1}
	for _, ttl := range invalid {
		if domain.ValidArvanCloudDNSRecordTTL(ttl) {
			t.Errorf("ValidArvanCloudDNSRecordTTL(%d) = true, want false", ttl)
		}
	}
}

// TestValidArvanCloudUpstreamHTTPS pins the upstream_https enum.
func TestValidArvanCloudUpstreamHTTPS(t *testing.T) {
	for _, s := range []string{"default", "auto", "http", "https"} {
		if !domain.ValidArvanCloudUpstreamHTTPS(s) {
			t.Errorf("ValidArvanCloudUpstreamHTTPS(%q) = false, want true", s)
		}
	}
	if domain.ValidArvanCloudUpstreamHTTPS("bogus") {
		t.Error(`ValidArvanCloudUpstreamHTTPS("bogus") = true, want false`)
	}
}

// TestValidArvanCloudCAATag pins the CAA tag enum.
func TestValidArvanCloudCAATag(t *testing.T) {
	for _, s := range []string{"issue", "issuewild", "iodef"} {
		if !domain.ValidArvanCloudCAATag(s) {
			t.Errorf("ValidArvanCloudCAATag(%q) = false, want true", s)
		}
	}
	if domain.ValidArvanCloudCAATag("bogus") {
		t.Error(`ValidArvanCloudCAATag("bogus") = true, want false`)
	}
}
