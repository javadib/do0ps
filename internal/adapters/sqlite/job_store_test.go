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
