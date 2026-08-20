package domain

import (
	"fmt"
	"sort"
	"strings"
)

// ProviderError is a failure the provider itself reported, carrying the
// provider's own answer rather than only a sentinel.
//
// The sentinels above tell an adapter how to *classify* a failure; they say
// nothing about what actually went wrong. "provider unavailable" is useless to
// a chatbot that could have told the user "403: your IP is not whitelisted",
// so provider adapters wrap their sentinel in this type and the MCP adapter
// reports the detail back to the caller.
//
// Callers keep using errors.Is against the sentinels — Unwrap preserves that.
// Use errors.As to reach the detail.
//
// Nothing here may hold credentials: every field is populated from the
// provider's response, never from the request that was sent.
type ProviderError struct {
	// Provider names the upstream API, e.g. "parspack".
	Provider string

	// Operation is the request that failed, e.g. "GET api/public/v1/vms".
	// Never a full URL: query strings can carry caller-supplied values.
	Operation string

	// StatusCode is the HTTP status the provider answered with, or 0 when the
	// request never reached it (DNS failure, connection refused, timeout).
	StatusCode int

	// Message is the provider's own error message, if its body carried one.
	Message string

	// Details holds per-field validation errors, as Parspack's 422 responses
	// return them ({"errors": {"field": ["reason"]}}).
	Details map[string][]string

	// Body is a truncated copy of the raw response body, kept only when
	// nothing more structured could be parsed out of it, so an unexpected
	// error shape is still visible instead of being reduced to a status code.
	Body string

	// Err is the sentinel this failure classifies as.
	Err error
}

// maxBodyExcerpt caps the raw body kept on a ProviderError. Enough to show a
// short error payload or the opening of an HTML error page, small enough not
// to push a wall of markup into the model's context.
const maxBodyExcerpt = 512

// Error renders the failure the way it should reach the caller: what was
// called, what the provider answered, and why it is being classified as it is.
func (e *ProviderError) Error() string {
	var b strings.Builder

	b.WriteString(e.Provider)
	if e.Operation != "" {
		b.WriteString(" ")
		b.WriteString(e.Operation)
	}
	b.WriteString(": ")

	if e.StatusCode > 0 {
		fmt.Fprintf(&b, "HTTP %d", e.StatusCode)
	} else {
		b.WriteString("no response")
	}

	if e.Message != "" {
		b.WriteString(": ")
		b.WriteString(e.Message)
	}
	if detail := e.detailString(); detail != "" {
		b.WriteString(": ")
		b.WriteString(detail)
	}
	if e.Message == "" && len(e.Details) == 0 && e.Body != "" {
		b.WriteString(": ")
		b.WriteString(e.Body)
	}

	if e.Err != nil {
		b.WriteString(" (")
		b.WriteString(e.Err.Error())
		b.WriteString(")")
	}
	return b.String()
}

// detailString renders the per-field validation errors in a stable order, so
// the same failure always produces the same message.
func (e *ProviderError) detailString() string {
	if len(e.Details) == 0 {
		return ""
	}

	fields := make([]string, 0, len(e.Details))
	for field := range e.Details {
		fields = append(fields, field)
	}
	sort.Strings(fields)

	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		parts = append(parts, field+": "+strings.Join(e.Details[field], "; "))
	}
	return strings.Join(parts, ", ")
}

// Unwrap exposes the sentinel so errors.Is keeps classifying this failure.
func (e *ProviderError) Unwrap() error { return e.Err }

// TruncateBody trims a raw response body to what is worth carrying in an
// error: whitespace-normalized and capped at maxBodyExcerpt.
func TruncateBody(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if len(trimmed) <= maxBodyExcerpt {
		return trimmed
	}
	return trimmed[:maxBodyExcerpt] + "... (truncated)"
}
