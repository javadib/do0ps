package domain

import (
	"encoding/json"
	"time"
)

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
