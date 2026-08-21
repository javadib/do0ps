package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// Snapshot and restore operations of the fake provider defined in app_test.go.
// Methods may live in any file of the same test package.

func (p *fakeProvider) CreateVMSnapshot(_ context.Context, _ domain.ProviderCredentials, serverID, name string) (*domain.VMAction, error) {
	p.snapshotCalls++
	if p.snapshotErr != nil {
		return nil, p.snapshotErr
	}
	action := &domain.VMAction{
		ID:       "act-1",
		ServerID: serverID,
		Type:     "snapshot",
		Status:   domain.VMActionStatusInProgress,
	}
	if p.actions == nil {
		p.actions = make(map[string]*domain.VMAction)
	}
	p.actions[action.ID] = action
	// The snapshot shows up in the list once the action is completed, which
	// GetVMAction reports on its first poll below.
	p.snapshots = append(p.snapshots, domain.VMSnapshot{ID: "5001", Name: name, ServerID: serverID})
	return action, nil
}

func (p *fakeProvider) GetVMAction(_ context.Context, _ domain.ProviderCredentials, serverID, actionID string) (*domain.VMAction, error) {
	action, ok := p.actions[actionID]
	if !ok {
		return nil, fmt.Errorf("action %q: %w", actionID, domain.ErrNotFound)
	}
	action.Status = domain.VMActionStatusCompleted
	return action, nil
}

func (p *fakeProvider) ListVMSnapshots(context.Context, domain.ProviderCredentials) ([]domain.VMSnapshot, error) {
	return p.snapshots, nil
}

func (p *fakeProvider) DeleteVMSnapshot(_ context.Context, _ domain.ProviderCredentials, id string) error {
	if p.deleteSnapErr != nil {
		return p.deleteSnapErr
	}
	p.deletedSnapshot = id
	return nil
}

func (p *fakeProvider) RestoreVM(_ context.Context, _ domain.ProviderCredentials, serverID, snapshotID string) (*domain.VMAction, error) {
	p.restoreCalls++
	if p.restoreErr != nil {
		return nil, p.restoreErr
	}
	action := &domain.VMAction{
		ID:       "act-2",
		ServerID: serverID,
		Type:     "restore",
		Status:   domain.VMActionStatusInProgress,
	}
	if p.actions == nil {
		p.actions = make(map[string]*domain.VMAction)
	}
	p.actions[action.ID] = action
	return action, nil
}

func TestCreateSnapshotReturnsPendingOperation(t *testing.T) {
	jobs := newMemJobs()
	queue := &inlineQueue{}
	clock := fixedClock{t: time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)}

	uc, err := app.NewCreateSnapshot(jobs, queue, &fakeProvider{}, clock, fixedIDs{id: "op-1"})
	if err != nil {
		t.Fatalf("NewCreateSnapshot: %v", err)
	}

	out, err := uc.Execute(context.Background(), app.CreateSnapshotInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		ServerID:    "vm-1",
		Name:        "pre-upgrade",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if out.OperationID != "op-1" {
		t.Errorf("operation ID = %q, want op-1", out.OperationID)
	}
	if out.Status != domain.OperationStatusPending {
		t.Errorf("status = %s, want pending", out.Status)
	}
	if len(queue.submitted) != 1 {
		t.Fatalf("submitted %d jobs, want 1", len(queue.submitted))
	}

	stored, err := jobs.Get(context.Background(), "op-1")
	if err != nil {
		t.Fatalf("job not persisted: %v", err)
	}
	if stored.Type != domain.JobTypeCreateSnapshot {
		t.Errorf("job type = %s, want create_snapshot", stored.Type)
	}
	// Provider credentials must never reach storage.
	if payload := string(stored.Payload); strings.Contains(payload, "api_key") || strings.Contains(payload, "APIKey") {
		t.Errorf("payload appears to contain credentials: %s", payload)
	}
}

func TestCreateSnapshotRequiresInput(t *testing.T) {
	uc, err := app.NewCreateSnapshot(newMemJobs(), &inlineQueue{}, &fakeProvider{}, fixedClock{}, fixedIDs{id: "op-1"})
	if err != nil {
		t.Fatalf("NewCreateSnapshot: %v", err)
	}

	_, err = uc.Execute(context.Background(), app.CreateSnapshotInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		ServerID:    "vm-1",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput for a missing name", err)
	}
}

func TestCreateSnapshotHandleRunsToCompletion(t *testing.T) {
	jobs := newMemJobs()
	provider := &fakeProvider{}
	clock := fixedClock{t: time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)}

	uc, err := app.NewCreateSnapshot(jobs, &inlineQueue{}, provider, clock, fixedIDs{id: "op-1"},
		app.WithActionPollInterval(time.Millisecond),
		app.WithActionPollTimeout(time.Minute),
	)
	if err != nil {
		t.Fatalf("NewCreateSnapshot: %v", err)
	}

	if _, err := uc.Execute(context.Background(), app.CreateSnapshotInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		ServerID:    "vm-1",
		Name:        "pre-upgrade",
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	job, err := jobs.Get(context.Background(), "op-1")
	if err != nil {
		t.Fatalf("job not persisted: %v", err)
	}
	result, err := uc.Handle(context.Background(), job)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	var snap domain.VMSnapshot
	if err := json.Unmarshal(result, &snap); err != nil {
		t.Fatalf("decoding result: %v", err)
	}
	if snap.ID != "5001" || snap.Name != "pre-upgrade" || snap.ServerID != "vm-1" {
		t.Errorf("snapshot = %+v, want id 5001, name pre-upgrade, server vm-1", snap)
	}
	if provider.snapshotCalls != 1 {
		t.Errorf("CreateVMSnapshot called %d times, want 1", provider.snapshotCalls)
	}
}

func TestCreateSnapshotHandleAdoptsExistingSnapshotOnRetry(t *testing.T) {
	jobs := newMemJobs()
	provider := &fakeProvider{
		snapshots: []domain.VMSnapshot{{ID: "5001", Name: "pre-upgrade", ServerID: "vm-1"}},
	}
	clock := fixedClock{t: time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)}

	uc, err := app.NewCreateSnapshot(jobs, &inlineQueue{}, provider, clock, fixedIDs{id: "op-1"},
		app.WithActionPollInterval(time.Millisecond),
		app.WithActionPollTimeout(time.Minute),
	)
	if err != nil {
		t.Fatalf("NewCreateSnapshot: %v", err)
	}

	if _, err := uc.Execute(context.Background(), app.CreateSnapshotInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		ServerID:    "vm-1",
		Name:        "pre-upgrade",
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	job, err := jobs.Get(context.Background(), "op-1")
	if err != nil {
		t.Fatalf("job not persisted: %v", err)
	}
	job.Attempts = 2 // simulate a retry whose first attempt may have created the snapshot

	result, err := uc.Handle(context.Background(), job)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	var snap domain.VMSnapshot
	if err := json.Unmarshal(result, &snap); err != nil {
		t.Fatalf("decoding result: %v", err)
	}
	if snap.ID != "5001" {
		t.Errorf("snapshot ID = %q, want 5001 (adopted, not re-created)", snap.ID)
	}
	if provider.snapshotCalls != 0 {
		t.Errorf("CreateVMSnapshot called %d times, want 0 on a retry that can adopt", provider.snapshotCalls)
	}
}

func TestListSnapshotsReturnsProviderResult(t *testing.T) {
	provider := &fakeProvider{snapshots: []domain.VMSnapshot{
		{ID: "5001", Name: "pre-upgrade"},
		{ID: "5002", Name: "baseline"},
	}}
	uc := app.NewListSnapshots(&inlineQueue{}, provider)

	snapshots, err := uc.Execute(context.Background(), app.ListSnapshotsInput{Credentials: domain.ProviderCredentials{APIKey: "k"}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("len(snapshots) = %d, want 2", len(snapshots))
	}
}

func TestListSnapshotsRequiresCredentials(t *testing.T) {
	uc := app.NewListSnapshots(&inlineQueue{}, &fakeProvider{})

	_, err := uc.Execute(context.Background(), app.ListSnapshotsInput{})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestDeleteSnapshotCallsProvider(t *testing.T) {
	provider := &fakeProvider{}
	uc := app.NewDeleteSnapshot(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteSnapshotInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		SnapshotID:  "5001",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.deletedSnapshot != "5001" {
		t.Errorf("deletedSnapshot = %q, want 5001", provider.deletedSnapshot)
	}
}

func TestDeleteSnapshotTreatsAlreadyGoneAsSuccess(t *testing.T) {
	provider := &fakeProvider{deleteSnapErr: fmt.Errorf("snapshot 5001: %w", domain.ErrNotFound)}
	uc := app.NewDeleteSnapshot(&inlineQueue{}, provider)

	err := uc.Execute(context.Background(), app.DeleteSnapshotInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		SnapshotID:  "5001",
	})
	if err != nil {
		t.Fatalf("Execute: %v, want nil for an already-deleted snapshot", err)
	}
}

func TestRestoreVMReturnsPendingOperation(t *testing.T) {
	jobs := newMemJobs()
	queue := &inlineQueue{}
	clock := fixedClock{t: time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)}

	uc, err := app.NewRestoreVM(jobs, queue, &fakeProvider{}, clock, fixedIDs{id: "op-2"})
	if err != nil {
		t.Fatalf("NewRestoreVM: %v", err)
	}

	out, err := uc.Execute(context.Background(), app.RestoreVMInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		ServerID:    "vm-1",
		SnapshotID:  "5001",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if out.OperationID != "op-2" {
		t.Errorf("operation ID = %q, want op-2", out.OperationID)
	}
	if out.Status != domain.OperationStatusPending {
		t.Errorf("status = %s, want pending", out.Status)
	}
	if len(queue.submitted) != 1 {
		t.Fatalf("submitted %d jobs, want 1", len(queue.submitted))
	}

	stored, err := jobs.Get(context.Background(), "op-2")
	if err != nil {
		t.Fatalf("job not persisted: %v", err)
	}
	if stored.Type != domain.JobTypeRestoreVM {
		t.Errorf("job type = %s, want restore_vm", stored.Type)
	}
}

func TestRestoreVMRequiresInput(t *testing.T) {
	uc, err := app.NewRestoreVM(newMemJobs(), &inlineQueue{}, &fakeProvider{}, fixedClock{}, fixedIDs{id: "op-2"})
	if err != nil {
		t.Fatalf("NewRestoreVM: %v", err)
	}

	_, err = uc.Execute(context.Background(), app.RestoreVMInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		ServerID:    "vm-1",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput for a missing snapshot_id", err)
	}
}

func TestRestoreVMHandleRunsToCompletion(t *testing.T) {
	jobs := newMemJobs()
	provider := &fakeProvider{
		servers: []domain.Server{{ID: "vm-1", Name: "web-01", Status: domain.ServerStatusRunning}},
	}
	clock := fixedClock{t: time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)}

	uc, err := app.NewRestoreVM(jobs, &inlineQueue{}, provider, clock, fixedIDs{id: "op-2"},
		app.WithActionPollInterval(time.Millisecond),
		app.WithActionPollTimeout(time.Minute),
	)
	if err != nil {
		t.Fatalf("NewRestoreVM: %v", err)
	}

	if _, err := uc.Execute(context.Background(), app.RestoreVMInput{
		Credentials: domain.ProviderCredentials{APIKey: "k"},
		ServerID:    "vm-1",
		SnapshotID:  "5001",
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	job, err := jobs.Get(context.Background(), "op-2")
	if err != nil {
		t.Fatalf("job not persisted: %v", err)
	}
	result, err := uc.Handle(context.Background(), job)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	var srv domain.Server
	if err := json.Unmarshal(result, &srv); err != nil {
		t.Fatalf("decoding result: %v", err)
	}
	if srv.ID != "vm-1" {
		t.Errorf("server ID = %q, want vm-1", srv.ID)
	}
	if provider.restoreCalls != 1 {
		t.Errorf("RestoreVM called %d times, want 1", provider.restoreCalls)
	}
}

// TestGetOperationStatusReconcilesInterruptedSnapshot proves an interrupted
// snapshot job is resolved by asking the provider whether the snapshot already
// exists, never by replaying the create call (AGENTS.md 4.4).
func TestGetOperationStatusReconcilesInterruptedSnapshot(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	jobs := newMemJobs()

	payload, err := json.Marshal(map[string]any{"server_id": "vm-1", "name": "before-upgrade"})
	if err != nil {
		t.Fatalf("marshaling payload: %v", err)
	}
	job := &domain.Job{
		ID:        "op-1",
		Type:      domain.JobTypeCreateSnapshot,
		Payload:   payload,
		Status:    domain.JobStatusPending,
		Error:     domain.InterruptedReason,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := jobs.Create(context.Background(), job); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// The provider already has the snapshot, so reconcile must adopt it
	// rather than take a second one.
	provider := &fakeProvider{snapshots: []domain.VMSnapshot{
		{ID: "snap-9", Name: "before-upgrade", ServerID: "vm-1"},
	}}
	uc := app.NewGetOperationStatus(jobs, provider, nil, fixedClock{t: now.Add(time.Minute)})

	op, err := uc.Execute(context.Background(), app.GetOperationStatusInput{
		OperationID: "op-1",
		Credentials: domain.ProviderCredentials{APIKey: "k"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if op.Status != domain.OperationStatusSucceeded {
		t.Fatalf("status = %s, want succeeded", op.Status)
	}

	var snap domain.VMSnapshot
	if err := json.Unmarshal(op.Result, &snap); err != nil {
		t.Fatalf("decoding result: %v", err)
	}
	if snap.ID != "snap-9" {
		t.Errorf("result ID = %q, want snap-9", snap.ID)
	}
	if provider.snapshotCalls != 0 {
		t.Errorf("CreateVMSnapshot called %d times during reconciliation, want lookup only", provider.snapshotCalls)
	}
}

// TestGetOperationStatusReconcilesMissingSnapshotAsFailed proves an interrupted
// job whose snapshot was never taken is marked failed so the request can be
// retried safely.
func TestGetOperationStatusReconcilesMissingSnapshotAsFailed(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	jobs := newMemJobs()

	payload, err := json.Marshal(map[string]any{"server_id": "vm-1", "name": "before-upgrade"})
	if err != nil {
		t.Fatalf("marshaling payload: %v", err)
	}
	job := &domain.Job{
		ID:        "op-1",
		Type:      domain.JobTypeCreateSnapshot,
		Payload:   payload,
		Status:    domain.JobStatusPending,
		Error:     domain.InterruptedReason,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := jobs.Create(context.Background(), job); err != nil {
		t.Fatalf("Create: %v", err)
	}

	uc := app.NewGetOperationStatus(jobs, &fakeProvider{}, nil, fixedClock{t: now.Add(time.Minute)})

	op, err := uc.Execute(context.Background(), app.GetOperationStatusInput{
		OperationID: "op-1",
		Credentials: domain.ProviderCredentials{APIKey: "k"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if op.Status != domain.OperationStatusFailed {
		t.Fatalf("status = %s, want failed", op.Status)
	}
}
