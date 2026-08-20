package domain

import (
	"encoding/json"
	"time"
)

// OperationClass distinguishes the two kinds of provider operation described
// in AGENTS.md 4.3. Each application use case declares which class it is:
//   - Fast operations run on a worker while the MCP caller's tool call blocks
//     on the result (ports.Queue.Dispatch).
//   - Long operations are persisted as a Job and return an operation ID
//     immediately; the caller polls GetOperationStatus for progress
//     (ports.Queue.Submit).
type OperationClass int

const (
	OperationClassUnknown OperationClass = iota
	OperationClassFast
	OperationClassLong
)

// String returns the value used in logs and docs.
func (c OperationClass) String() string {
	switch c {
	case OperationClassFast:
		return "fast"
	case OperationClassLong:
		return "long"
	default:
		return "unknown"
	}
}

// ParseOperationClass converts a class name back into an OperationClass.
func ParseOperationClass(s string) (OperationClass, error) {
	switch s {
	case "fast":
		return OperationClassFast, nil
	case "long":
		return OperationClassLong, nil
	default:
		return OperationClassUnknown, ErrInvalidInput
	}
}

// OperationStatus is the caller-visible state of a long-running operation.
// It is a deliberately smaller vocabulary than JobStatus: callers care about
// progress, not about retry bookkeeping.
type OperationStatus int

const (
	OperationStatusUnknown OperationStatus = iota
	OperationStatusPending
	OperationStatusRunning
	OperationStatusSucceeded
	OperationStatusFailed
)

// String returns the value reported to MCP callers.
func (s OperationStatus) String() string {
	switch s {
	case OperationStatusPending:
		return "pending"
	case OperationStatusRunning:
		return "running"
	case OperationStatusSucceeded:
		return "succeeded"
	case OperationStatusFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// Operation is the caller-facing view of a long-running job.
type Operation struct {
	ID        string
	Status    OperationStatus
	Result    json.RawMessage
	Error     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// OperationFromJob projects a stored job into the view returned by
// get_operation_status.
func OperationFromJob(j *Job) Operation {
	var status OperationStatus
	switch j.Status {
	case JobStatusPending:
		status = OperationStatusPending
	case JobStatusRunning:
		status = OperationStatusRunning
	case JobStatusDone:
		status = OperationStatusSucceeded
	case JobStatusFailed:
		status = OperationStatusFailed
	default:
		status = OperationStatusUnknown
	}

	return Operation{
		ID:        j.ID,
		Status:    status,
		Result:    j.Result,
		Error:     j.Error,
		CreatedAt: j.CreatedAt,
		UpdatedAt: j.UpdatedAt,
	}
}
