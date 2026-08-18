package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// RestoreVMInput identifies the target server and the snapshot to restore it
// from.
type RestoreVMInput struct {
	Credentials domain.ProviderCredentials
	ServerID    string
	SnapshotID  string
}

// RestoreVMOutput is returned immediately, before the restore finishes.
type RestoreVMOutput struct {
	OperationID string
	Status      domain.OperationStatus
}

// restoreVMPayload is what gets persisted for a restore job. Credentials are
// deliberately absent: they are never written to storage (AGENTS.md 4.2) and
// live only in memory for the lifetime of the operation.
type restoreVMPayload struct {
	ServerID   string `json:"server_id"`
	SnapshotID string `json:"snapshot_id"`
}

// RestoreVM is a long operation: it wipes the target server's disk and
// replaces it with the snapshot's contents behind an async action, returning
// an operation ID right away. This is destructive and cannot be undone.
type RestoreVM struct {
	op *longOp
}

// NewRestoreVM builds the use case from its ports.
func NewRestoreVM(
	jobs ports.JobRepository,
	queue ports.Queue,
	provider ports.ParspackProvider,
	clock ports.Clock,
	ids ports.IDGenerator,
	opts ...longOpOption,
) (*RestoreVM, error) {
	op, err := newLongOp(jobs, queue, provider, clock, ids, opts...)
	if err != nil {
		return nil, fmt.Errorf("configuring restore VM use case: %w", err)
	}
	return &RestoreVM{op: op}, nil
}

// Execute validates the request, persists a pending job and hands it to the
// queue. It never blocks on the provider.
func (uc *RestoreVM) Execute(ctx context.Context, in RestoreVMInput) (RestoreVMOutput, error) {
	if err := in.Credentials.Validate(); err != nil {
		return RestoreVMOutput{}, err
	}
	if in.ServerID == "" {
		return RestoreVMOutput{}, fmt.Errorf("server_id is required: %w", domain.ErrInvalidInput)
	}
	if in.SnapshotID == "" {
		return RestoreVMOutput{}, fmt.Errorf("snapshot_id is required: %w", domain.ErrInvalidInput)
	}

	payload, err := json.Marshal(restoreVMPayload{ServerID: in.ServerID, SnapshotID: in.SnapshotID})
	if err != nil {
		return RestoreVMOutput{}, fmt.Errorf("encoding restore payload: %w", err)
	}

	operationID, err := uc.op.start(ctx, domain.JobTypeRestoreVM, payload, in.Credentials)
	if err != nil {
		return RestoreVMOutput{}, err
	}
	return RestoreVMOutput{OperationID: operationID, Status: domain.OperationStatusPending}, nil
}

// Handle executes a restore job on a worker. It satisfies ports.JobHandler and
// is registered with the queue adapter at wiring time.
func (uc *RestoreVM) Handle(ctx context.Context, job *domain.Job) (json.RawMessage, error) {
	creds, ok := uc.op.credentials(job.ID)
	if !ok {
		return nil, fmt.Errorf("credentials for operation %s are no longer held in memory: %w", job.ID, domain.ErrInvalidCredentials)
	}
	defer uc.op.forgetCredentials(job.ID)

	var payload restoreVMPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return nil, fmt.Errorf("decoding restore payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, uc.op.pollTimeout)
	defer cancel()

	// Restore converges: replaying it lands on the same final disk state, so a
	// retry needs no adopt-before-replay reconciliation against the provider.
	action, err := uc.op.provider.RestoreVM(ctx, creds, payload.ServerID, payload.SnapshotID)
	if err != nil {
		return nil, fmt.Errorf("restoring snapshot %s to server %s: %w", payload.SnapshotID, payload.ServerID, err)
	}
	if _, err := uc.op.waitAction(ctx, creds, action); err != nil {
		return nil, err
	}

	srv, err := uc.op.provider.GetServer(ctx, creds, payload.ServerID)
	if err != nil {
		return nil, fmt.Errorf("getting restored server %s: %w", payload.ServerID, err)
	}
	return json.Marshal(srv)
}
