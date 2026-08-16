package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// GetOperationStatusInput identifies the operation to inspect.
//
// Credentials are optional. They are only needed when the operation was
// interrupted by a restart: this process no longer holds the caller's
// credentials, so the provider can only be consulted with credentials the
// caller re-supplies on this call.
type GetOperationStatusInput struct {
	OperationID string
	Credentials domain.ProviderCredentials
}

// GetOperationStatus reports progress of a long operation, reconciling
// interrupted ones against the provider when it can.
type GetOperationStatus struct {
	jobs     ports.JobRepository
	provider ports.ParspackProvider
	clock    ports.Clock
}

// NewGetOperationStatus builds the use case from its ports.
func NewGetOperationStatus(jobs ports.JobRepository, provider ports.ParspackProvider, clock ports.Clock) *GetOperationStatus {
	return &GetOperationStatus{jobs: jobs, provider: provider, clock: clock}
}

// Execute returns the caller-facing view of the operation.
func (uc *GetOperationStatus) Execute(ctx context.Context, in GetOperationStatusInput) (domain.Operation, error) {
	if in.OperationID == "" {
		return domain.Operation{}, fmt.Errorf("operation_id is required: %w", domain.ErrInvalidInput)
	}

	job, err := uc.jobs.Get(ctx, in.OperationID)
	if err != nil {
		return domain.Operation{}, fmt.Errorf("loading operation %s: %w", in.OperationID, err)
	}

	if job.WasInterrupted() && in.Credentials.Validate() == nil {
		if err := uc.reconcile(ctx, job, in.Credentials); err != nil {
			return domain.Operation{}, err
		}
	}

	return domain.OperationFromJob(job), nil
}

// reconcile resolves an interrupted job by asking the provider whether the
// resource exists, rather than replaying a create call that may already have
// succeeded (AGENTS.md 4.4).
func (uc *GetOperationStatus) reconcile(ctx context.Context, job *domain.Job, creds domain.ProviderCredentials) error {
	if job.Type != domain.JobTypeProvisionServer {
		return nil
	}

	var payload provisionPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("decoding payload of operation %s: %w", job.ID, err)
	}

	now := uc.clock.Now()
	if err := job.MarkRunning(now); err != nil {
		return fmt.Errorf("reconciling operation %s: %w", job.ID, err)
	}

	srv, err := uc.provider.FindServerByName(ctx, creds, payload.Spec.Name)
	switch {
	case err == nil:
		result, mErr := json.Marshal(srv)
		if mErr != nil {
			return fmt.Errorf("encoding reconciled server: %w", mErr)
		}
		if err := job.MarkDone(result, now); err != nil {
			return fmt.Errorf("reconciling operation %s: %w", job.ID, err)
		}
	case isNotFound(err):
		reason := "interrupted by process restart before the server was created; the request can be safely retried"
		if err := job.MarkFailed(reason, now); err != nil {
			return fmt.Errorf("reconciling operation %s: %w", job.ID, err)
		}
	default:
		// Leave the job interrupted so a later call can try again.
		return fmt.Errorf("reconciling operation %s against provider: %w", job.ID, err)
	}

	if err := uc.jobs.Update(ctx, job); err != nil {
		return fmt.Errorf("persisting reconciled operation %s: %w", job.ID, err)
	}
	return nil
}
