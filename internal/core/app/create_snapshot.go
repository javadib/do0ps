package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// CreateSnapshotInput is the normalized form of a create_snapshot tool call.
type CreateSnapshotInput struct {
	Credentials domain.ProviderCredentials
	ServerID    string
	Name        string
}

// CreateSnapshotOutput is returned immediately, before the snapshot exists.
type CreateSnapshotOutput struct {
	OperationID string
	Status      domain.OperationStatus
}

// createSnapshotPayload is what gets persisted for a snapshot job. Credentials
// are deliberately absent: they are never written to storage (AGENTS.md 4.2)
// and live only in memory for the lifetime of the operation.
type createSnapshotPayload struct {
	ServerID string `json:"server_id"`
	Name     string `json:"name"`
}

// CreateSnapshot is a long operation: it starts an async snapshot action and
// returns an operation ID right away; the caller polls get_operation_status.
type CreateSnapshot struct {
	op *longOp
}

// NewCreateSnapshot builds the use case from its ports.
func NewCreateSnapshot(
	jobs ports.JobRepository,
	queue ports.Queue,
	provider ports.ParspackProvider,
	clock ports.Clock,
	ids ports.IDGenerator,
	opts ...longOpOption,
) (*CreateSnapshot, error) {
	op, err := newLongOp(jobs, queue, provider, clock, ids, opts...)
	if err != nil {
		return nil, fmt.Errorf("configuring create snapshot use case: %w", err)
	}
	return &CreateSnapshot{op: op}, nil
}

// Execute validates the request, persists a pending job and hands it to the
// queue. It never blocks on the provider.
func (uc *CreateSnapshot) Execute(ctx context.Context, in CreateSnapshotInput) (CreateSnapshotOutput, error) {
	if err := in.Credentials.Validate(); err != nil {
		return CreateSnapshotOutput{}, err
	}
	if in.ServerID == "" {
		return CreateSnapshotOutput{}, fmt.Errorf("server_id is required: %w", domain.ErrInvalidInput)
	}
	if in.Name == "" {
		return CreateSnapshotOutput{}, fmt.Errorf("snapshot name is required: %w", domain.ErrInvalidInput)
	}

	payload, err := json.Marshal(createSnapshotPayload{ServerID: in.ServerID, Name: in.Name})
	if err != nil {
		return CreateSnapshotOutput{}, fmt.Errorf("encoding snapshot payload: %w", err)
	}

	operationID, err := uc.op.start(ctx, domain.JobTypeCreateSnapshot, payload, in.Credentials)
	if err != nil {
		return CreateSnapshotOutput{}, err
	}
	return CreateSnapshotOutput{OperationID: operationID, Status: domain.OperationStatusPending}, nil
}

// Handle executes a snapshot job on a worker. It satisfies ports.JobHandler
// and is registered with the queue adapter at wiring time.
func (uc *CreateSnapshot) Handle(ctx context.Context, job *domain.Job) (json.RawMessage, error) {
	creds, ok := uc.op.credentials(job.ID)
	if !ok {
		return nil, fmt.Errorf("credentials for operation %s are no longer held in memory: %w", job.ID, domain.ErrInvalidCredentials)
	}
	// Credentials are deliberately NOT released here: a failed attempt is
	// retried by the queue, and that retry needs them just as much. They are
	// released from Settled, once the job can no longer be re-attempted.

	var payload createSnapshotPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return nil, fmt.Errorf("decoding snapshot payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, uc.op.pollTimeout)
	defer cancel()

	// A retry must not duplicate the snapshot: adopt one a previous attempt
	// already created before starting a new snapshot action.
	if job.Attempts > 1 || job.WasInterrupted() {
		existing, err := uc.op.findSnapshot(ctx, creds, payload.ServerID, payload.Name)
		if err == nil {
			return json.Marshal(existing)
		}
		if !isNotFound(err) {
			return nil, err
		}
	}

	action, err := uc.op.provider.CreateVMSnapshot(ctx, creds, payload.ServerID, payload.Name)
	if err != nil {
		return nil, fmt.Errorf("creating snapshot %q of server %s: %w", payload.Name, payload.ServerID, err)
	}
	if _, err := uc.op.waitAction(ctx, creds, action); err != nil {
		return nil, err
	}

	snap, err := uc.op.waitSnapshotVisible(ctx, creds, payload.ServerID, payload.Name)
	if err != nil {
		return nil, err
	}
	return json.Marshal(snap)
}

// Settled releases the operation's in-memory credentials. It satisfies
// ports.JobSettled and is registered with the queue adapter at wiring time,
// which calls it once the job reaches a terminal state.
func (uc *CreateSnapshot) Settled(jobID string) { uc.op.forgetCredentials(jobID) }
