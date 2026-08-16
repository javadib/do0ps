// Package system provides the small secondary adapters for facilities the
// core deliberately does not reach for directly: the wall clock and identifier
// generation. Injecting them keeps use cases deterministic under test.
package system

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/javadib/do0ps/internal/core/ports"
)

// Clock reports the real current time.
type Clock struct{}

var _ ports.Clock = Clock{}

// Now returns the current UTC time.
func (Clock) Now() time.Time { return time.Now().UTC() }

// IDGenerator produces random operation identifiers.
type IDGenerator struct{}

var _ ports.IDGenerator = IDGenerator{}

// NewID returns a 128-bit random, URL-safe identifier. Operation IDs are
// handed to callers, so they come from crypto/rand rather than a predictable
// sequence.
func (IDGenerator) NewID() string {
	var buf [16]byte
	// rand.Read from crypto/rand never returns an error on supported platforms;
	// it panics internally if the OS entropy source fails, which is a condition
	// this process cannot meaningfully continue past anyway.
	_, _ = rand.Read(buf[:])
	return hex.EncodeToString(buf[:])
}
