package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

func TestRecoveryFlagsUnfinishedJobsAsInterrupted(t *testing.T) {
	jobs := newMemJobs()
	createdAt := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)

	pending := &domain.Job{
		ID:        "op-pending",
		Type:      domain.JobTypeProvisionServer,
		Status:    domain.JobStatusPending,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
	running := &domain.Job{
		ID:        "op-running",
		Type:      domain.JobTypeProvisionServer,
		Status:    domain.JobStatusRunning,
		Attempts:  1,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
	done := &domain.Job{
		ID:        "op-done",
		Type:      domain.JobTypeProvisionServer,
		Status:    domain.JobStatusDone,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
	for _, j := range []*domain.Job{pending, running, done} {
		if err := jobs.Create(context.Background(), j); err != nil {
			t.Fatalf("Create %s: %v", j.ID, err)
		}
	}

	recoveredAt := createdAt.Add(time.Hour)
	uc := app.NewRecovery(jobs, fixedClock{t: recoveredAt})

	flagged, err := uc.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if flagged != 2 {
		t.Fatalf("flagged = %d, want 2 (pending + running, not done)", flagged)
	}

	for _, id := range []string{"op-pending", "op-running"} {
		got, err := jobs.Get(context.Background(), id)
		if err != nil {
			t.Fatalf("Get %s: %v", id, err)
		}
		if got.Status != domain.JobStatusPending {
			t.Errorf("job %s status = %s, want pending", id, got.Status)
		}
		if !got.WasInterrupted() {
			t.Errorf("job %s WasInterrupted() = false, want true", id)
		}
		if !got.UpdatedAt.Equal(recoveredAt) {
			t.Errorf("job %s UpdatedAt = %s, want %s", id, got.UpdatedAt, recoveredAt)
		}
	}

	// A job that had already reached a terminal state before the restart must
	// not be touched: replaying success/failure would misreport the outcome.
	untouched, err := jobs.Get(context.Background(), "op-done")
	if err != nil {
		t.Fatalf("Get op-done: %v", err)
	}
	if untouched.Status != domain.JobStatusDone || untouched.WasInterrupted() {
		t.Errorf("done job = %+v, want untouched", untouched)
	}
}

func TestRecoveryNoUnfinishedJobs(t *testing.T) {
	uc := app.NewRecovery(newMemJobs(), fixedClock{t: time.Now()})

	flagged, err := uc.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if flagged != 0 {
		t.Fatalf("flagged = %d, want 0", flagged)
	}
}
