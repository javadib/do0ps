package parspack

import (
	"context"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
)

// The methods below implement ports.ParspackProvider. Each one is responsible
// for exactly two things: calling the right endpoint through c.do, and
// translating the provider's payload into the domain types. Provider-specific
// JSON shapes stay in this package — nothing above the adapter boundary should
// ever see them.

// CreateServer provisions a compute instance.
func (c *Client) CreateServer(ctx context.Context, creds domain.ProviderCredentials, spec domain.ServerSpec) (*domain.Server, error) {
	return nil, fmt.Errorf("create server %q: %w", spec.Name, errNotImplemented)
}

// GetServer returns a single instance by provider ID.
func (c *Client) GetServer(ctx context.Context, creds domain.ProviderCredentials, id string) (*domain.Server, error) {
	return nil, fmt.Errorf("get server %s: %w", id, errNotImplemented)
}

// ListServers returns every instance visible to the credentials.
func (c *Client) ListServers(ctx context.Context, creds domain.ProviderCredentials) ([]domain.Server, error) {
	return nil, fmt.Errorf("list servers: %w", errNotImplemented)
}

// FindServerByName supports crash reconciliation. It must return
// domain.ErrNotFound — not a nil server — when no instance matches.
func (c *Client) FindServerByName(ctx context.Context, creds domain.ProviderCredentials, name string) (*domain.Server, error) {
	servers, err := c.ListServers(ctx, creds)
	if err != nil {
		return nil, err
	}
	for i := range servers {
		if servers[i].Name == name {
			return &servers[i], nil
		}
	}
	return nil, fmt.Errorf("server %q: %w", name, domain.ErrNotFound)
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
