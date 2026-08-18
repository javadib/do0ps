package parspack

import (
	"context"
	"fmt"
	"time"

	"github.com/javadib/do0ps/internal/core/domain"
)

// firewallsBasePath is confirmed against github.com/abrhacom/go-api-abrha's
// firewalls.go (AGENTS.md 4.5: cross-reference that client when the docs site
// cannot be scraped). Relative to Client.baseURL, i.e.
// https://my.parspack.com/cserver/api/public/v1/firewalls.
//
// These are the cloud-server/VM-network-level firewalls of the Abrha-based
// cloud-server API. The CDN API has its own, unrelated edge-level firewall
// concept (issue #24) on a different base URL — nothing here touches it.
const firewallsBasePath = "api/public/v1/firewalls"

// The wire types below mirror github.com/abrhacom/go-api-abrha's firewalls.go
// exactly (field names and JSON tags), restricted to the fields domain.Firewall
// carries: rule sources/destinations are addresses only, since the domain type
// does not model rule-level tags, VM IDs, or Kubernetes/LoadBalancer targets.
// Nothing above the adapter boundary ever sees these — the CRUD methods below
// translate them into internal/core/domain types.

type firewallWire struct {
	ID            string             `json:"id,omitempty"`
	Name          string             `json:"name,omitempty"`
	Status        string             `json:"status,omitempty"`
	InboundRules  []inboundRuleWire  `json:"inbound_rules,omitempty"`
	OutboundRules []outboundRuleWire `json:"outbound_rules,omitempty"`
	VmIDs         []string           `json:"vm_ids,omitempty"`
	Tags          []string           `json:"tags,omitempty"`
	Created       string             `json:"created_at,omitempty"`
}

type inboundRuleWire struct {
	Protocol  string       `json:"protocol,omitempty"`
	PortRange string       `json:"ports,omitempty"`
	Sources   *sourcesWire `json:"sources,omitempty"`
}

type outboundRuleWire struct {
	Protocol     string            `json:"protocol,omitempty"`
	PortRange    string            `json:"ports,omitempty"`
	Destinations *destinationsWire `json:"destinations,omitempty"`
}

type sourcesWire struct {
	Addresses []string `json:"addresses,omitempty"`
}

type destinationsWire struct {
	Addresses []string `json:"addresses,omitempty"`
}

// firewallRoot mirrors the single-resource envelope {"firewall": {...}}.
type firewallRoot struct {
	Firewall *firewallWire `json:"firewall"`
}

// firewallsRoot mirrors the list envelope {"firewalls": [...]}.
type firewallsRoot struct {
	Firewalls []firewallWire `json:"firewalls"`
}

// firewallRequest is what Create/Update send, mirroring go-api-abrha's
// FirewallRequest restricted to the fields domain.Firewall supports.
type firewallRequest struct {
	Name          string             `json:"name"`
	InboundRules  []inboundRuleWire  `json:"inbound_rules,omitempty"`
	OutboundRules []outboundRuleWire `json:"outbound_rules,omitempty"`
	VmIDs         []string           `json:"vm_ids,omitempty"`
}

// CreateFirewall registers a new rules-based firewall.
func (c *Client) CreateFirewall(ctx context.Context, creds domain.ProviderCredentials, fw domain.Firewall) (*domain.Firewall, error) {
	reqBody := toFirewallRequest(fw)
	var root firewallRoot
	if err := c.doJSON(ctx, creds, "POST", firewallsBasePath, reqBody, &root); err != nil {
		return nil, fmt.Errorf("creating firewall %q: %w", fw.Name, err)
	}
	if root.Firewall == nil {
		return nil, fmt.Errorf("creating firewall %q: %w", fw.Name, errEmptyResponse)
	}
	return toDomainFirewall(root.Firewall), nil
}

// GetFirewall returns a single firewall by provider ID.
func (c *Client) GetFirewall(ctx context.Context, creds domain.ProviderCredentials, id string) (*domain.Firewall, error) {
	var root firewallRoot
	if err := c.doJSON(ctx, creds, "GET", firewallsBasePath+"/"+id, nil, &root); err != nil {
		return nil, fmt.Errorf("get firewall %s: %w", id, err)
	}
	if root.Firewall == nil {
		return nil, fmt.Errorf("get firewall %s: %w", id, errEmptyResponse)
	}
	return toDomainFirewall(root.Firewall), nil
}

// ListFirewalls returns every firewall visible to the credentials.
func (c *Client) ListFirewalls(ctx context.Context, creds domain.ProviderCredentials) ([]domain.Firewall, error) {
	var root firewallsRoot
	if err := c.doJSON(ctx, creds, "GET", firewallsBasePath, nil, &root); err != nil {
		return nil, fmt.Errorf("list firewalls: %w", err)
	}

	firewalls := make([]domain.Firewall, len(root.Firewalls))
	for i := range root.Firewalls {
		firewalls[i] = *toDomainFirewall(&root.Firewalls[i])
	}
	return firewalls, nil
}

// UpdateFirewall replaces a firewall's configuration by provider ID.
func (c *Client) UpdateFirewall(ctx context.Context, creds domain.ProviderCredentials, id string, fw domain.Firewall) (*domain.Firewall, error) {
	var root firewallRoot
	if err := c.doJSON(ctx, creds, "PUT", firewallsBasePath+"/"+id, toFirewallRequest(fw), &root); err != nil {
		return nil, fmt.Errorf("update firewall %s: %w", id, err)
	}
	if root.Firewall == nil {
		return nil, fmt.Errorf("update firewall %s: %w", id, errEmptyResponse)
	}
	return toDomainFirewall(root.Firewall), nil
}

// DeleteFirewall removes a firewall by provider ID.
func (c *Client) DeleteFirewall(ctx context.Context, creds domain.ProviderCredentials, id string) error {
	if err := c.doJSON(ctx, creds, "DELETE", firewallsBasePath+"/"+id, nil, nil); err != nil {
		return fmt.Errorf("delete firewall %s: %w", id, err)
	}
	return nil
}

// toFirewallRequest translates a domain.Firewall into the wire request. The
// read-only fields (ID, Status, Tags, CreatedAt) are not part of the request.
func toFirewallRequest(fw domain.Firewall) firewallRequest {
	req := firewallRequest{Name: fw.Name, VmIDs: fw.ServerIDs}
	for _, r := range fw.InboundRules {
		req.InboundRules = append(req.InboundRules, inboundRuleWire{
			Protocol:  r.Protocol,
			PortRange: r.PortRange,
			Sources:   &sourcesWire{Addresses: r.Addresses},
		})
	}
	for _, r := range fw.OutboundRules {
		req.OutboundRules = append(req.OutboundRules, outboundRuleWire{
			Protocol:     r.Protocol,
			PortRange:    r.PortRange,
			Destinations: &destinationsWire{Addresses: r.Addresses},
		})
	}
	return req
}

// toDomainFirewall translates a wire firewall into the shared domain shape.
func toDomainFirewall(f *firewallWire) *domain.Firewall {
	fw := &domain.Firewall{
		ID:        f.ID,
		Name:      f.Name,
		Status:    f.Status,
		ServerIDs: f.VmIDs,
	}
	for _, r := range f.InboundRules {
		rule := domain.FirewallRule{Protocol: r.Protocol, PortRange: r.PortRange}
		if r.Sources != nil {
			rule.Addresses = r.Sources.Addresses
		}
		fw.InboundRules = append(fw.InboundRules, rule)
	}
	for _, r := range f.OutboundRules {
		rule := domain.FirewallRule{Protocol: r.Protocol, PortRange: r.PortRange}
		if r.Destinations != nil {
			rule.Addresses = r.Destinations.Addresses
		}
		fw.OutboundRules = append(fw.OutboundRules, rule)
	}
	if f.Created != "" {
		if t, err := time.Parse(time.RFC3339, f.Created); err == nil {
			fw.CreatedAt = t
		}
	}
	return fw
}
