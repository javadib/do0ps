package app

import (
	"errors"

	"github.com/javadib/do0ps/internal/core/domain"
)

// isNotFound is the shared check use cases use to treat a delete of an
// already-absent resource as success (AGENTS.md 4.4).
func isNotFound(err error) bool { return errors.Is(err, domain.ErrNotFound) }
