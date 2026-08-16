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
	Create(ctx context.Context, job *domain.Job) error
	Get(ctx context.Context, id string) (*domain.Job, error)
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
// to this server.
type ParspackProvider interface {
	CreateServer(ctx context.Context, creds domain.ProviderCredentials, spec domain.ServerSpec) (*domain.Server, error)
	GetServer(ctx context.Context, creds domain.ProviderCredentials, id string) (*domain.Server, error)
	ListServers(ctx context.Context, creds domain.ProviderCredentials) ([]domain.Server, error)

	// FindServerByName supports crash recovery: before retrying a create-style
	// job found in running state, core asks whether the resource already exists.
	// Returns domain.ErrNotFound when it does not.
	FindServerByName(ctx context.Context, creds domain.ProviderCredentials, name string) (*domain.Server, error)

	ListDNSZones(ctx context.Context, creds domain.ProviderCredentials) ([]domain.DNSZone, error)
	ListDNSRecords(ctx context.Context, creds domain.ProviderCredentials, zoneID string) ([]domain.DNSRecord, error)
	CreateDNSRecord(ctx context.Context, creds domain.ProviderCredentials, rec domain.DNSRecord) (*domain.DNSRecord, error)
	DeleteDNSRecord(ctx context.Context, creds domain.ProviderCredentials, zoneID, recordID string) error
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
