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
// (issue #9). Firewall methods (CreateFirewall, GetFirewall, ListFirewalls,
// UpdateFirewall, DeleteFirewall) live in firewalls.go, wired to the same
// cloud-server API (issue #11). Everything below remains a stub — issues #10
// (SSH keys) and #19 (CDN zones/DNS) wire these up against their own confirmed
// endpoints.

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

// ListDNSZones returns the domains hosted on the account.
func (c *Client) ListDNSZones(ctx context.Context, creds domain.ProviderCredentials) ([]domain.DNSZone, error) {
	return nil, fmt.Errorf("list DNS zones: %w", errNotImplemented)
}

// ListDNSRecords returns the records of one zone.
func (c *Client) ListDNSRecords(ctx context.Context, creds domain.ProviderCredentials, zoneID string) ([]domain.DNSRecord, error) {
	return nil, fmt.Errorf("list DNS records of zone %s: %w", zoneID, errNotImplemented)
}

// CreateDNSRecord adds a record to a zone.
func (c *Client) CreateDNSRecord(ctx context.Context, creds domain.ProviderCredentials, rec domain.DNSRecord) (*domain.DNSRecord, error) {
	return nil, fmt.Errorf("create %s record %q: %w", rec.Type, rec.Name, errNotImplemented)
}

// DeleteDNSRecord removes a record from a zone.
func (c *Client) DeleteDNSRecord(ctx context.Context, creds domain.ProviderCredentials, zoneID, recordID string) error {
	return fmt.Errorf("delete DNS record %s: %w", recordID, errNotImplemented)
}
