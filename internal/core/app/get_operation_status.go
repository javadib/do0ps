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
// interrupted ones against the provider when it can. It reconciles job types
// from both providers this project implements: arvanProvider is used only
// for domain.JobTypeIssueArvanCloudManagedCertificate (issue #73) — every
// other job type above is Parspack-only.
type GetOperationStatus struct {
	jobs          ports.JobRepository
	provider      ports.ParspackProvider
	arvanProvider ports.ArvanCloudProvider
	clock         ports.Clock
}

// NewGetOperationStatus builds the use case from its ports.
func NewGetOperationStatus(jobs ports.JobRepository, provider ports.ParspackProvider, arvanProvider ports.ArvanCloudProvider, clock ports.Clock) *GetOperationStatus {
	return &GetOperationStatus{jobs: jobs, provider: provider, arvanProvider: arvanProvider, clock: clock}
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
	// ArvanCloud's managed-certificate issuance (issue #73) does not fit the
	// generic finder-based cases below: an order existing is not by itself
	// success — it can still be legitimately in flight — so it gets its own
	// method instead of a finder/resourceName/failReason triple.
	if job.Type == domain.JobTypeIssueArvanCloudManagedCertificate {
		return uc.reconcileArvanCloudSslOrder(ctx, job, creds)
	}

	var (
		resourceName string
		finder       func(context.Context, domain.ProviderCredentials, string) (json.RawMessage, error)
		failReason   string
	)

	switch job.Type {
	case domain.JobTypeProvisionServer:
		var payload provisionPayload
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			return fmt.Errorf("decoding payload of operation %s: %w", job.ID, err)
		}
		resourceName = payload.Spec.Name
		failReason = "interrupted by process restart before the server was created; the request can be safely retried"
		finder = func(ctx context.Context, creds domain.ProviderCredentials, name string) (json.RawMessage, error) {
			srv, err := uc.provider.FindServerByName(ctx, creds, name)
			if err != nil {
				return nil, err
			}
			return json.Marshal(srv)
		}

	case domain.JobTypeProvisionLoadBalancer:
		var payload provisionLoadBalancerPayload
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			return fmt.Errorf("decoding payload of operation %s: %w", job.ID, err)
		}
		resourceName = payload.LoadBalancer.Name
		failReason = "interrupted by process restart before the load balancer was created; the request can be safely retried"
		finder = func(ctx context.Context, creds domain.ProviderCredentials, name string) (json.RawMessage, error) {
			lb, err := uc.provider.FindLoadBalancerByName(ctx, creds, name)
			if err != nil {
				return nil, err
			}
			return json.Marshal(lb)
		}

	case domain.JobTypeCreateSnapshot:
		var payload createSnapshotPayload
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			return fmt.Errorf("decoding payload of operation %s: %w", job.ID, err)
		}
		resourceName = payload.Name
		failReason = "interrupted by process restart before the snapshot was created; the request can be safely retried"
		finder = func(ctx context.Context, creds domain.ProviderCredentials, name string) (json.RawMessage, error) {
			// Snapshots are not addressable by name alone — the provider lists
			// them per account — so the server they belong to disambiguates.
			snaps, err := uc.provider.ListVMSnapshots(ctx, creds)
			if err != nil {
				return nil, err
			}
			found, err := findSnapshotByServerAndName(snaps, payload.ServerID, name)
			if err != nil {
				return nil, err
			}
			return json.Marshal(found)
		}

	default:
		// Restore converges on the same state when replayed, so an interrupted
		// restore needs no reconciliation; nothing else is interrupted here.
		return nil
	}

	now := uc.clock.Now()
	if err := job.MarkRunning(now); err != nil {
		return fmt.Errorf("reconciling operation %s: %w", job.ID, err)
	}

	result, err := finder(ctx, creds, resourceName)
	switch {
	case err == nil:
		if err := job.MarkDone(result, now); err != nil {
			return fmt.Errorf("reconciling operation %s: %w", job.ID, err)
		}
	case isNotFound(err):
		if err := job.MarkFailed(failReason, now); err != nil {
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

// reconcileArvanCloudSslOrder resolves an interrupted managed-certificate
// issuance job by querying ssl.cert.order.index (ListArvanCloudSslOrders,
// via findLatestArvanCloudSslOrder) for the domain's actual current order,
// instead of blindly calling IssueArvanCloudManagedCertificate a second time
// (AGENTS.md 4.4, issue #73's explicit acceptance criterion). Unlike the
// generic finder-based cases in reconcile, an order existing is not
// automatically success: a still-in-flight order (not
// domain.ArvanCloudCertificateOrderTerminal) leaves the job Running with no
// result yet, so a later call — with or without credentials — reports
// "running" and checks again, rather than declaring an outcome that has not
// actually happened.
func (uc *GetOperationStatus) reconcileArvanCloudSslOrder(ctx context.Context, job *domain.Job, creds domain.ProviderCredentials) error {
	var payload issueArvanCloudManagedCertificatePayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("decoding payload of operation %s: %w", job.ID, err)
	}

	now := uc.clock.Now()
	if err := job.MarkRunning(now); err != nil {
		return fmt.Errorf("reconciling operation %s: %w", job.ID, err)
	}
	// Running is no longer "interrupted" — MarkRunning does not clear Error
	// on its own (unlike MarkDone/MarkFailed below), and this call may well
	// leave the job genuinely Running rather than falling through to one of
	// those two.
	job.Error = ""

	order, err := findLatestArvanCloudSslOrder(ctx, uc.arvanProvider, creds, payload.Domain)
	switch {
	case err == nil && order.Status == domain.ArvanCloudCertificateOrderStatusValid:
		result, merr := json.Marshal(order)
		if merr != nil {
			return fmt.Errorf("encoding reconciled ssl order for operation %s: %w", job.ID, merr)
		}
		if err := job.MarkDone(result, now); err != nil {
			return fmt.Errorf("reconciling operation %s: %w", job.ID, err)
		}
	case err == nil && order.Status == domain.ArvanCloudCertificateOrderStatusKilled:
		reason := fmt.Sprintf(
			"arvancloud managed certificate order %s failed permanently (status killed); retry_arvancloud_ssl_order can attempt a manual retry",
			order.ID)
		if err := job.MarkFailed(reason, now); err != nil {
			return fmt.Errorf("reconciling operation %s: %w", job.ID, err)
		}
	case err == nil:
		// Still in flight (unprocessed/pending/processing/ready/invalid/
		// terminated/canceled): leave the job Running with no result — see
		// this method's own doc comment.
	case isNotFound(err):
		if err := job.MarkFailed(
			"interrupted by process restart before the certificate order was placed; the request can be safely retried",
			now,
		); err != nil {
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

// findSnapshotByServerAndName returns the snapshot matching both the server it
// was taken from and its name, or domain.ErrNotFound.
func findSnapshotByServerAndName(snapshots []domain.VMSnapshot, serverID, name string) (*domain.VMSnapshot, error) {
	for i := range snapshots {
		if snapshots[i].ServerID == serverID && snapshots[i].Name == name {
			return &snapshots[i], nil
		}
	}
	return nil, fmt.Errorf("snapshot %q of server %s: %w", name, serverID, domain.ErrNotFound)
}
