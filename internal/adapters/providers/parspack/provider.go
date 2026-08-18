package parspack

import (
	"context"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
)

// The methods below implement ports.ParspackProvider. Each one is responsible
// for exactly two things: calling the right endpoint, and translating the
// provider's payload into the domain types. Provider-specific JSON shapes stay
// in this package — nothing above the adapter boundary should ever see them.
//
// VM lifecycle methods (CreateServer, GetServer, ListServers, DeleteServer,
// FindServerByName) live in vms.go, wired to the real cloud-server API
// (issue #9). CDN zone management and DNS records live in cdn.go, wired to
// the CDN API (issue #19). SSH keys below remain a stub — issue #10 wires
// them up against their own confirmed cloud-server endpoints.

// CreateSSHKey registers a public key with the provider.
func (c *Client) CreateSSHKey(ctx context.Context, creds domain.ProviderCredentials, key domain.SSHKey) (*domain.SSHKey, error) {
	return nil, fmt.Errorf("create SSH key %q: %w", key.Name, errNotImplemented)
}

// ListSSHKeys returns every key registered with the credentials.
func (c *Client) ListSSHKeys(ctx context.Context, creds domain.ProviderCredentials) ([]domain.SSHKey, error) {
	return nil, fmt.Errorf("list SSH keys: %w", errNotImplemented)
}

// DeleteSSHKey removes a registered key by provider ID.
func (c *Client) DeleteSSHKey(ctx context.Context, creds domain.ProviderCredentials, id string) error {
	return fmt.Errorf("delete SSH key %s: %w", id, errNotImplemented)
}
