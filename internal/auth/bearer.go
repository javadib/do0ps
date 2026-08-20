// Package auth guards the MCP endpoint with a bearer-token allow-list.
//
// This is MCP server access control only. It has nothing to do with the
// provider credentials that travel inside tool call arguments — the two auth
// layers are unrelated and must not be conflated (AGENTS.md 4.2).
package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/javadib/do0ps/internal/core/domain"
)

// localsKey is the context key under which the authenticated client is stored.
type localsKey struct{}

// Store resolves a bearer token to the client it belongs to.
type Store interface {
	Lookup(token string) (domain.Client, bool)
}

// StaticStore is an in-memory allow-list. Tokens are held as digests so a heap
// dump or accidental log of the store does not leak usable credentials.
type StaticStore struct {
	entries map[[32]byte]domain.Client
}

var _ Store = (*StaticStore)(nil)

// NewStaticStore builds an allow-list from token to client.
func NewStaticStore(tokens map[string]domain.Client) (*StaticStore, error) {
	if len(tokens) == 0 {
		return nil, errors.New("at least one bearer token must be configured")
	}

	entries := make(map[[32]byte]domain.Client, len(tokens))
	for token, client := range tokens {
		if len(token) < 16 {
			return nil, fmt.Errorf("token for client %q is too short: use at least 16 characters", client.ID)
		}
		entries[sha256.Sum256([]byte(token))] = client
	}
	return &StaticStore{entries: entries}, nil
}

// Lookup resolves a token. The digest comparison is constant time, so a
// caller cannot learn a valid prefix from response timing.
func (s *StaticStore) Lookup(token string) (domain.Client, bool) {
	sum := sha256.Sum256([]byte(token))
	for candidate, client := range s.entries {
		if subtle.ConstantTimeCompare(sum[:], candidate[:]) == 1 {
			return client, true
		}
	}
	return domain.Client{}, false
}

// ParseTokens reads an allow-list from a "token:client_id[:name]" list,
// separated by commas — the shape used by the MCP_AUTH_TOKENS environment
// variable. client_id is unused today and reserved for a future multi-tenant
// mode.
func ParseTokens(spec string) (map[string]domain.Client, error) {
	tokens := make(map[string]domain.Client)

	for _, entry := range strings.Split(spec, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		parts := strings.Split(entry, ":")
		if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
			return nil, errors.New(`token entry must look like "token:client_id" or "token:client_id:name"`)
		}

		client := domain.Client{ID: parts[1], Name: parts[1]}
		if len(parts) > 2 && parts[2] != "" {
			client.Name = parts[2]
		}
		tokens[parts[0]] = client
	}

	if len(tokens) == 0 {
		return nil, errors.New("no tokens configured")
	}
	return tokens, nil
}

// Middleware rejects requests that do not carry an allow-listed bearer token.
func Middleware(store Store) fiber.Handler {
	return func(c fiber.Ctx) error {
		token, ok := bearerToken(c.Get(fiber.HeaderAuthorization))
		if !ok {
			return unauthorized(c)
		}

		client, ok := store.Lookup(token)
		if !ok {
			return unauthorized(c)
		}

		c.Locals(localsKey{}, client)
		return c.Next()
	}
}

// ClientFrom returns the authenticated client attached by Middleware.
func ClientFrom(c fiber.Ctx) (domain.Client, bool) {
	client, ok := c.Locals(localsKey{}).(domain.Client)
	return client, ok
}

func bearerToken(header string) (string, bool) {
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(header[len(prefix):])
	return token, token != ""
}

// unauthorized answers without hinting at why the token was rejected.
func unauthorized(c fiber.Ctx) error {
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": domain.ErrUnauthorized.Error()})
}
