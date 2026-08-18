// Package domain holds the pure business types and rules of do0ps.
//
// It must not import any framework, transport or storage package: no Fiber,
// no database/sql, no provider SDKs. Everything the domain needs from the
// outside world is expressed as an interface in internal/port.
package domain

import (
	"encoding/json"
	"time"
)

// JobStatus is the lifecycle state of a queued unit of work.
type JobStatus int

const (
	JobStatusUnknown JobStatus = iota // zero value: never a valid persisted status
	JobStatusPending
	JobStatusRunning
	JobStatusDone
	JobStatusFailed
)

// String returns the persisted representation of the status.
func (s JobStatus) String() string {
	switch s {
	case JobStatusPending:
		return "pending"
	case JobStatusRunning:
		return "running"
	case JobStatusDone:
		return "done"
	case JobStatusFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// ParseJobStatus converts a persisted status string back into a JobStatus.
func ParseJobStatus(s string) (JobStatus, error) {
	switch s {
	case "pending":
		return JobStatusPending, nil
	case "running":
		return JobStatusRunning, nil
	case "done":
		return JobStatusDone, nil
	case "failed":
		return JobStatusFailed, nil
	default:
		return JobStatusUnknown, ErrInvalidJobStatus
	}
}

// JobType identifies which handler executes a job.
type JobType string

// Job types, one per long-running use case.
const (
	JobTypeProvisionServer       JobType = "provision_server"
	JobTypeProvisionLoadBalancer JobType = "provision_load_balancer"
)

// InterruptedReason marks a job whose in-process execution was lost to a
// restart. Such a job cannot simply be replayed: the provider call may have
// succeeded moments before the crash, and the credentials needed to check are
// held only by the caller's session.
const InterruptedReason = "interrupted by process restart"

// Job is a unit of provider work tracked in the job store.
//
// Fast operations use a Job only in memory; long operations persist it so the
// caller can poll progress through an operation ID after the tool call
// returned.
type Job struct {
	ID          string
	TenantID    string // reserved for future multi-tenant mode; empty today
	Type        JobType
	Payload     json.RawMessage
	Status      JobStatus
	Attempts    int
	NextRetryAt time.Time
	Result      json.RawMessage
	Error       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// MarkRunning moves a pending job into execution and counts the attempt.
func (j *Job) MarkRunning(now time.Time) error {
	if j.Status != JobStatusPending {
		return ErrInvalidJobTransition
	}
	j.Status = JobStatusRunning
	j.Attempts++
	j.UpdatedAt = now
	return nil
}

// MarkDone records a terminal success with the provider result.
func (j *Job) MarkDone(result json.RawMessage, now time.Time) error {
	if j.Status != JobStatusRunning {
		return ErrInvalidJobTransition
	}
	j.Status = JobStatusDone
	j.Result = result
	j.Error = ""
	j.UpdatedAt = now
	return nil
}

// MarkFailed records a terminal failure. Retryable failures should instead go
// back to pending through Reschedule.
func (j *Job) MarkFailed(reason string, now time.Time) error {
	if j.Status != JobStatusRunning {
		return ErrInvalidJobTransition
	}
	j.Status = JobStatusFailed
	j.Error = reason
	j.UpdatedAt = now
	return nil
}

// Reschedule returns a running job to the pending queue for a later retry.
func (j *Job) Reschedule(at time.Time, reason string, now time.Time) error {
	if j.Status != JobStatusRunning {
		return ErrInvalidJobTransition
	}
	j.Status = JobStatusPending
	j.NextRetryAt = at
	j.Error = reason
	j.UpdatedAt = now
	return nil
}

// IsTerminal reports whether the job will not change state again.
func (j *Job) IsTerminal() bool {
	return j.Status == JobStatusDone || j.Status == JobStatusFailed
}

// MarkInterrupted flags a non-terminal job found at startup. It stays pending
// so it is not silently reported as successful, and carries a reason telling
// later readers that the provider must be consulted before any retry.
func (j *Job) MarkInterrupted(now time.Time) error {
	if j.IsTerminal() {
		return ErrInvalidJobTransition
	}
	j.Status = JobStatusPending
	j.Error = InterruptedReason
	j.UpdatedAt = now
	return nil
}

// WasInterrupted reports whether the job is awaiting reconciliation after a
// restart.
func (j *Job) WasInterrupted() bool {
	return !j.IsTerminal() && j.Error == InterruptedReason
}

// NeedsReconciliation reports whether recovery must ask the provider about the
// job's real outcome before retrying it. A job found running at startup may
// have completed on the provider side just before the process died; replaying
// a create-style call blindly would duplicate the resource.
func (j *Job) NeedsReconciliation() bool {
	return j.Status == JobStatusRunning
}
