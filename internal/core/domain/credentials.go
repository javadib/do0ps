package domain

import "fmt"

// ProviderCredentials are the end user's provider API credentials.
//
// They are never persisted and never read from server-side configuration: the
// calling chatbot session holds them and passes them as parameters on every
// provider-touching tool call (see AGENTS.md 4.2). Keep them out of logs.
type ProviderCredentials struct {
	APIKey    string
	SecretKey string // empty for providers that use a single key
}

// Validate checks that the minimum credential material is present.
func (c ProviderCredentials) Validate() error {
	if c.APIKey == "" {
		return fmt.Errorf("api_key is required: %w", ErrInvalidInput)
	}
	return nil
}

// String redacts the credentials so they cannot leak through %v or %s.
func (c ProviderCredentials) String() string { return "ProviderCredentials{REDACTED}" }

// GoString redacts the credentials under %#v.
func (c ProviderCredentials) GoString() string { return c.String() }

// Client is an authenticated caller of the MCP server itself — a bearer token
// from the allow-list. ID is reserved for a future multi-tenant mode.
type Client struct {
	ID   string
	Name string
}
