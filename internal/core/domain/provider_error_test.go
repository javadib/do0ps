package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestProviderErrorMessage(t *testing.T) {
	tests := []struct {
		name string
		err  *ProviderError
		want string
	}{
		{
			name: "status and message",
			err: &ProviderError{
				Provider:   "parspack",
				Operation:  "GET api/zones",
				StatusCode: 403,
				Message:    "IP not whitelisted",
			},
			want: "parspack GET api/zones: HTTP 403: IP not whitelisted",
		},
		{
			name: "sentinel is named alongside the provider's answer",
			err: &ProviderError{
				Provider:   "parspack",
				Operation:  "GET api/zones",
				StatusCode: 401,
				Message:    "Unauthenticated.",
				Err:        ErrInvalidCredentials,
			},
			want: "parspack GET api/zones: HTTP 401: Unauthenticated. (invalid provider credentials)",
		},
		{
			name: "validation details render in a stable order",
			err: &ProviderError{
				Provider:   "parspack",
				Operation:  "POST api/zones",
				StatusCode: 422,
				Message:    "The given data was invalid.",
				Details: map[string][]string{
					"origin": {"The origin is required."},
					"domain": {"The domain is invalid.", "The domain is taken."},
				},
				Err: ErrInvalidInput,
			},
			want: "parspack POST api/zones: HTTP 422: The given data was invalid.: " +
				"domain: The domain is invalid.; The domain is taken., origin: The origin is required. " +
				"(invalid input)",
		},
		{
			name: "unparseable body is shown when nothing else was extracted",
			err: &ProviderError{
				Provider:   "parspack",
				Operation:  "GET api/zones",
				StatusCode: 502,
				Body:       "<html>Bad Gateway</html>",
				Err:        ErrProviderUnavailable,
			},
			want: "parspack GET api/zones: HTTP 502: <html>Bad Gateway</html> (provider unavailable)",
		},
		{
			name: "no response at all",
			err: &ProviderError{
				Provider:  "parspack",
				Operation: "GET api/zones",
				Message:   "dial tcp: connection refused",
				Err:       ErrProviderUnavailable,
			},
			want: "parspack GET api/zones: no response: dial tcp: connection refused (provider unavailable)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.err.Error(); got != tc.want {
				t.Errorf("Error() =\n  %q\nwant\n  %q", got, tc.want)
			}
		})
	}
}

// TestProviderErrorUnwrap pins the property every caller relies on: adding
// detail must not break the sentinel classification.
func TestProviderErrorUnwrap(t *testing.T) {
	err := error(&ProviderError{Provider: "parspack", StatusCode: 404, Err: ErrNotFound})

	if !errors.Is(err, ErrNotFound) {
		t.Errorf("errors.Is(err, ErrNotFound) = false, want true")
	}
	if errors.Is(err, ErrInvalidInput) {
		t.Errorf("errors.Is(err, ErrInvalidInput) = true, want false")
	}

	var target *ProviderError
	if !errors.As(err, &target) || target.StatusCode != 404 {
		t.Errorf("errors.As did not recover the status code, got %+v", target)
	}
}

// TestProviderErrorUnclassifiedStatusStillReadable covers a status with no
// sentinel (403 and friends): the classification is absent, but the detail
// that makes the failure actionable must not be.
func TestProviderErrorUnclassifiedStatusStillReadable(t *testing.T) {
	err := &ProviderError{Provider: "parspack", Operation: "GET x", StatusCode: 403, Message: "Forbidden"}

	got := err.Error()
	if !strings.Contains(got, "403") || !strings.Contains(got, "Forbidden") {
		t.Errorf("Error() = %q, want the status and message", got)
	}
	if strings.Contains(got, "()") {
		t.Errorf("Error() = %q, want no empty sentinel suffix", got)
	}
}

func TestTruncateBody(t *testing.T) {
	if got := TruncateBody([]byte("  spaced  ")); got != "spaced" {
		t.Errorf("TruncateBody() = %q, want %q", got, "spaced")
	}

	long := strings.Repeat("x", maxBodyExcerpt+100)
	got := TruncateBody([]byte(long))
	if len(got) >= len(long) {
		t.Errorf("TruncateBody() kept %d bytes, want it capped near %d", len(got), maxBodyExcerpt)
	}
	if !strings.HasSuffix(got, "(truncated)") {
		t.Errorf("TruncateBody() = %q, want it to say it was truncated", got)
	}
}
