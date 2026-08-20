package arvancloud

import "testing"

// TestNewProvider checks the two things the adapter guarantees before any
// capability method exists: it is built around the client it was handed, and
// it refuses to be built around none.
func TestNewProvider(t *testing.T) {
	client, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	provider, err := NewProvider(client)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	if provider.Client() != client {
		t.Error("Provider.Client() is not the client it was built with")
	}

	if _, err := NewProvider(nil); err == nil {
		t.Error("NewProvider(nil) error = nil, want a refusal")
	}
}

// TestDefaultBaseURL pins the API root against the spec's servers entry: a
// wrong base URL fails as a 404 on every call, which reads like a missing
// resource rather than a misconfigured client.
func TestDefaultBaseURL(t *testing.T) {
	client, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if want := "https://napi.arvancloud.ir/cdn/4.0"; client.baseURL != want {
		t.Errorf("default base URL = %q, want %q", client.baseURL, want)
	}
}
