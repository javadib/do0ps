package sqlite_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/javadib/do0ps/internal/adapters/sqlite"
	"github.com/javadib/do0ps/internal/core/domain"
)

func newStore(t *testing.T) *sqlite.JobStore {
	t.Helper()

	db, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	return sqlite.NewJobStore(db)
}

func TestJobStoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	store := newStore(t)
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)

	job := &domain.Job{
		ID:          "op-1",
		Type:        domain.JobTypeProvisionServer,
		Payload:     json.RawMessage(`{"spec":{"Name":"web-01"}}`),
		Status:      domain.JobStatusPending,
		NextRetryAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := store.Create(ctx, job); err != nil {
		t.Fatalf("Create: %v", err)
	}

	loaded, err := store.Get(ctx, "op-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if loaded.Status != domain.JobStatusPending || loaded.Type != domain.JobTypeProvisionServer {
		t.Fatalf("loaded job = %+v, want a pending provision_server job", loaded)
	}
	if !loaded.CreatedAt.Equal(now) {
		t.Errorf("created_at = %s, want %s", loaded.CreatedAt, now)
	}

	if err := loaded.MarkRunning(now.Add(time.Second)); err != nil {
		t.Fatalf("MarkRunning: %v", err)
	}
	if err := store.Update(ctx, loaded); err != nil {
		t.Fatalf("Update: %v", err)
	}

	unfinished, err := store.ListUnfinished(ctx)
	if err != nil {
		t.Fatalf("ListUnfinished: %v", err)
	}
	if len(unfinished) != 1 || unfinished[0].Status != domain.JobStatusRunning {
		t.Fatalf("unfinished = %+v, want one running job", unfinished)
	}

	if err := unfinished[0].MarkDone(json.RawMessage(`{"id":"srv-1"}`), now.Add(2*time.Second)); err != nil {
		t.Fatalf("MarkDone: %v", err)
	}
	if err := store.Update(ctx, unfinished[0]); err != nil {
		t.Fatalf("Update: %v", err)
	}

	remaining, err := store.ListUnfinished(ctx)
	if err != nil {
		t.Fatalf("ListUnfinished: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("unfinished after completion = %d, want 0", len(remaining))
	}
}

func TestJobStoreGetUnknown(t *testing.T) {
	_, err := newStore(t).Get(context.Background(), "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

// TestJobStoreSurvivesRestart simulates a process restart: jobs are written
// through one *sql.DB handle, that handle is closed (as happens on process
// exit), and a fresh Open against the same file must still see them. This is
// the persistence guarantee the startup-recovery use case (app.Recovery)
// depends on — recovery is worthless if pending/running jobs don't actually
// survive the restart they're meant to be recovered from.
func TestJobStoreSurvivesRestart(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "restart.db")
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)

	func() {
		db, err := sqlite.Open(ctx, dbPath)
		if err != nil {
			t.Fatalf("Open (first process): %v", err)
		}
		defer db.Close()

		store := sqlite.NewJobStore(db)
		emptyPayload := json.RawMessage(`{}`)
		pending := &domain.Job{
			ID:          "op-pending",
			Type:        domain.JobTypeProvisionServer,
			Payload:     emptyPayload,
			Status:      domain.JobStatusPending,
			NextRetryAt: now,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := store.Create(ctx, pending); err != nil {
			t.Fatalf("Create pending: %v", err)
		}

		running := &domain.Job{
			ID:        "op-running",
			Type:      domain.JobTypeProvisionServer,
			Payload:   emptyPayload,
			Status:    domain.JobStatusPending,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := store.Create(ctx, running); err != nil {
			t.Fatalf("Create running: %v", err)
		}
		if err := running.MarkRunning(now); err != nil {
			t.Fatalf("MarkRunning: %v", err)
		}
		if err := store.Update(ctx, running); err != nil {
			t.Fatalf("Update running: %v", err)
		}

		done := &domain.Job{
			ID:        "op-done",
			Type:      domain.JobTypeProvisionServer,
			Payload:   emptyPayload,
			Status:    domain.JobStatusPending,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := store.Create(ctx, done); err != nil {
			t.Fatalf("Create done: %v", err)
		}
		if err := done.MarkRunning(now); err != nil {
			t.Fatalf("MarkRunning done: %v", err)
		}
		if err := done.MarkDone(json.RawMessage(`{}`), now); err != nil {
			t.Fatalf("MarkDone: %v", err)
		}
		if err := store.Update(ctx, done); err != nil {
			t.Fatalf("Update done: %v", err)
		}
		// db.Close() below stands in for the process dying.
	}()

	// A brand new *sql.DB against the same path stands in for the next
	// process starting up.
	db, err := sqlite.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open (second process): %v", err)
	}
	defer db.Close()
	store := sqlite.NewJobStore(db)

	unfinished, err := store.ListUnfinished(ctx)
	if err != nil {
		t.Fatalf("ListUnfinished after restart: %v", err)
	}

	byID := make(map[string]*domain.Job, len(unfinished))
	for _, job := range unfinished {
		byID[job.ID] = job
	}
	if len(byID) != 2 {
		t.Fatalf("unfinished after restart = %d, want 2 (pending + running, not done): %+v", len(byID), unfinished)
	}
	if got := byID["op-pending"]; got == nil || got.Status != domain.JobStatusPending {
		t.Errorf("op-pending after restart = %+v, want a pending job", got)
	}
	if got := byID["op-running"]; got == nil || got.Status != domain.JobStatusRunning {
		t.Errorf("op-running after restart = %+v, want a running job", got)
	}
	if _, ok := byID["op-done"]; ok {
		t.Errorf("op-done reappeared in ListUnfinished after restart, want it excluded (terminal)")
	}
}
