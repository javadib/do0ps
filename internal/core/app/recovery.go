package app

import (
	"context"
	"fmt"

	"github.com/javadib/do0ps/internal/core/ports"
)

// Recovery runs once at startup over jobs left behind by a previous process.
type Recovery struct {
	jobs  ports.JobRepository
	clock ports.Clock
}

// NewRecovery builds the use case from its ports.
func NewRecovery(jobs ports.JobRepository, clock ports.Clock) *Recovery {
	return &Recovery{jobs: jobs, clock: clock}
}

// Run flags every unfinished job as interrupted instead of replaying it.
//
// Replaying is unsafe: a job found running may have completed at the provider
// just before the crash, and the credentials required to check are held by the
// caller's session, not by this server. The job therefore stays non-terminal
// and is resolved by the next get_operation_status call that carries
// credentials. Run returns how many jobs were flagged.
func (uc *Recovery) Run(ctx context.Context) (int, error) {
	unfinished, err := uc.jobs.ListUnfinished(ctx)
	if err != nil {
		return 0, fmt.Errorf("listing unfinished jobs: %w", err)
	}

	now := uc.clock.Now()
	flagged := 0
	for _, job := range unfinished {
		if err := job.MarkInterrupted(now); err != nil {
			return flagged, fmt.Errorf("flagging job %s as interrupted: %w", job.ID, err)
		}
		if err := uc.jobs.Update(ctx, job); err != nil {
			return flagged, fmt.Errorf("persisting interrupted job %s: %w", job.ID, err)
		}
		flagged++
	}
	return flagged, nil
}
