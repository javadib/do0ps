package domain

import "errors"

// Sentinel errors the service layer maps onto transport-level responses.
// Adapters compare with errors.Is and never invent their own copies.
var (
	// ErrNotFound means the requested resource does not exist, either locally
	// (unknown operation ID) or at the provider.
	ErrNotFound = errors.New("not found")

	// ErrUnauthorized means the caller's MCP bearer token is missing or not on
	// the allow-list.
	ErrUnauthorized = errors.New("unauthorized")

	// ErrInvalidCredentials means the provider rejected the credentials that
	// were passed in with the tool call.
	ErrInvalidCredentials = errors.New("invalid provider credentials")

	// ErrInvalidInput means a tool argument failed validation at the boundary.
	ErrInvalidInput = errors.New("invalid input")

	// ErrInvalidJobStatus means a persisted status string is not a known state.
	ErrInvalidJobStatus = errors.New("invalid job status")

	// ErrInvalidJobTransition means a job state change was not legal from the
	// job's current state.
	ErrInvalidJobTransition = errors.New("invalid job state transition")

	// ErrProviderUnavailable means the provider API could not be reached or
	// returned a retryable server-side failure.
	ErrProviderUnavailable = errors.New("provider unavailable")
)
