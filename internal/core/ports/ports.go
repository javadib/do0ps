// Package ports declares the interfaces the application core depends on.
//
// Every interface here is owned by core and implemented by an adapter under
// internal/adapters. Core never imports an adapter package; the composition
// root in cmd/server wires concrete implementations into the use cases.
package ports

import (
	"context"
	"encoding/json"
	"time"

	"github.com/javadib/do0ps/internal/core/domain"
)

// Task is a unit of fast provider work executed on a worker goroutine.
type Task func(ctx context.Context) (json.RawMessage, error)

// JobHandler executes a persisted long-running job. Handlers are implemented
// by use cases in internal/core/app and registered with the queue adapter at
// wiring time, keyed by domain.JobType.
type JobHandler func(ctx context.Context, job *domain.Job) (json.RawMessage, error)

// Queue schedules provider work. Implemented by internal/adapters/queue with
// Go channels and a bounded worker pool.
type Queue interface {
	// Dispatch runs a fast operation on a worker and blocks until it returns.
	// The caller's context bounds the wait, so a saturated pool surfaces as a
	// deadline error rather than an indefinite hang.
	Dispatch(ctx context.Context, task Task) (json.RawMessage, error)

	// Submit schedules an already-persisted job for background execution and
	// returns as soon as it is accepted. Used for long operations, where the
	// MCP caller polls progress through an operation ID instead of waiting.
	Submit(ctx context.Context, job *domain.Job) error
}

// JobRepository persists job state. Implemented by internal/adapters/sqlite.
type JobRepository interface {
	// Create persists a new job. The caller assigns Job.ID beforehand (see
	// ports.IDGenerator); Create returns an error if that ID is already in use.
	Create(ctx context.Context, job *domain.Job) error

	// Get returns the job with the given ID, or domain.ErrNotFound if none
	// exists.
	Get(ctx context.Context, id string) (*domain.Job, error)

	// Update overwrites a persisted job's mutable fields (status, attempts,
	// next_retry_at, result, error, updated_at) by ID. Returns domain.ErrNotFound
	// if the job was never created.
	Update(ctx context.Context, job *domain.Job) error

	// ListUnfinished returns every job still in pending or running state. Used
	// once at startup for recovery; running jobs must be reconciled against the
	// provider before they are retried (see AGENTS.md 4.4).
	ListUnfinished(ctx context.Context) ([]*domain.Job, error)

	// ListDue returns at most limit pending jobs whose next_retry_at has passed.
	ListDue(ctx context.Context, now time.Time, limit int) ([]*domain.Job, error)
}

// ParspackProvider is the port for the Parspack API, listing exactly the
// operations core needs. Each provider gets its own dedicated port for now —
// a shared provider interface is deliberately deferred until two or three
// providers make the real overlap visible (see AGENTS.md 4.1).
//
// Credentials are passed per call: they belong to the chatbot session, never
// to this server. Create-style methods are not expected to be idempotent on
// their own — callers that must not duplicate a resource on retry (recovery
// after a crash, see AGENTS.md 4.4) are expected to check first, e.g. via
// FindServerByName, rather than relying on the provider to deduplicate.
type ParspackProvider interface {
	// VM lifecycle. CreateServer is a long operation (AGENTS.md 4.3); the
	// others are fast.
	CreateServer(ctx context.Context, creds domain.ProviderCredentials, spec domain.ServerSpec) (*domain.Server, error)
	GetServer(ctx context.Context, creds domain.ProviderCredentials, id string) (*domain.Server, error)
	ListServers(ctx context.Context, creds domain.ProviderCredentials) ([]domain.Server, error)

	// DeleteServer removes a server by provider ID. Deleting an ID that no
	// longer exists returns domain.ErrNotFound, so callers can treat delete as
	// idempotent by tolerating that specific error.
	DeleteServer(ctx context.Context, creds domain.ProviderCredentials, id string) error

	// FindServerByName supports crash recovery: before retrying a create-style
	// job found in running state, core asks whether the resource already exists.
	// Returns domain.ErrNotFound when it does not.
	FindServerByName(ctx context.Context, creds domain.ProviderCredentials, name string) (*domain.Server, error)

	// SSH key management. All fast operations. Keys are referenced by
	// ServerSpec.SSHKeys at server-creation time.
	CreateSSHKey(ctx context.Context, creds domain.ProviderCredentials, key domain.SSHKey) (*domain.SSHKey, error)
	ListSSHKeys(ctx context.Context, creds domain.ProviderCredentials) ([]domain.SSHKey, error)
	// DeleteSSHKey removes a key by provider ID. As with DeleteServer, an
	// already-absent ID reports domain.ErrNotFound rather than succeeding
	// silently, so callers decide for themselves whether that counts as done.
	DeleteSSHKey(ctx context.Context, creds domain.ProviderCredentials, id string) error

	ListDNSZones(ctx context.Context, creds domain.ProviderCredentials) ([]domain.DNSZone, error)
	ListDNSRecords(ctx context.Context, creds domain.ProviderCredentials, zoneID string) ([]domain.DNSRecord, error)
	CreateDNSRecord(ctx context.Context, creds domain.ProviderCredentials, rec domain.DNSRecord) (*domain.DNSRecord, error)
	DeleteDNSRecord(ctx context.Context, creds domain.ProviderCredentials, zoneID, recordID string) error

	// SSL certificate ordering workflow (AGENTS.md 4.5's SSL surface). All
	// fast operations: each is a single HTTP round trip, even though driving
	// a certificate to issuance takes a caller several separate calls
	// (create order, process, verify challenge, poll certificate).
	ListSSLProducts(ctx context.Context, creds domain.ProviderCredentials) ([]domain.SSLProduct, error)
	CreateSSLOrder(ctx context.Context, creds domain.ProviderCredentials, spec domain.SSLOrderSpec) (*domain.SSLOrder, error)
	// ProcessSSLOrder submits the CSR and contact details for a paid order
	// and returns the domain-ownership challenges to complete next.
	ProcessSSLOrder(ctx context.Context, creds domain.ProviderCredentials, orderID, csr string, contact domain.SSLContact) (*domain.SSLChallengeSet, error)
	// GetSSLChallenge re-shows the challenges of an already-processed order.
	GetSSLChallenge(ctx context.Context, creds domain.ProviderCredentials, orderID string) (*domain.SSLChallengeSet, error)
	// ReloadSSLChallenge switches the verification method, invalidating any
	// previously shown challenge tokens. emailPrefix is only meaningful when
	// method is "ADMIN".
	ReloadSSLChallenge(ctx context.Context, creds domain.ProviderCredentials, orderID, method, emailPrefix string) (*domain.SSLChallengeSet, error)
	// VerifySSLChallenge checks the completed challenge for method and, on
	// success, returns the certificate if it is ready immediately.
	VerifySSLChallenge(ctx context.Context, creds domain.ProviderCredentials, orderID, method string) (*domain.SSLVerifyResult, error)
	GetSSLCertificate(ctx context.Context, creds domain.ProviderCredentials, orderID string) (*domain.SSLCertificate, error)
	ReissueSSLCertificate(ctx context.Context, creds domain.ProviderCredentials, orderID, csr string) (*domain.SSLCertificate, error)
}

// Clock reports the current time. Injected so use cases stay deterministic
// under test.
type Clock interface {
	Now() time.Time
}

// IDGenerator produces operation and job identifiers.
type IDGenerator interface {
	NewID() string
}
