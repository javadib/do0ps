package parspack

import (
	"context"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
)

// reservedIPBasePath is confirmed against github.com/abrhacom/go-api-abrha's
// reserved_ips.go and reserved_ips_actions.go (AGENTS.md 4.5: cross-reference
// that client when the docs site cannot be scraped). Relative to
// Client.baseURL, i.e. https://my.parspack.com/cserver/api/public/v1/reserved_ips.
const reservedIPBasePath = "api/public/v1/reserved_ips"

// The wire types below mirror github.com/abrhacom/go-api-abrha's
// reserved_ips.go exactly (field names and JSON tags) so this adapter decodes
// real Parspack responses correctly. Nothing above the adapter boundary ever
// sees these — the port methods translate them into internal/core/domain
// types. Region and Vm reuse the shapes from vms.go.

type reservedIPWire struct {
	Region    *regionWire `json:"region,omitempty"`
	Vm        *vmWire     `json:"vm,omitempty"`
	IP        string      `json:"ip,omitempty"`
	ProjectID string      `json:"project_id,omitempty"`
	Locked    bool        `json:"locked,omitempty"`
}

type reservedIPRoot struct {
	ReservedIP *reservedIPWire `json:"reserved_ip"`
}

// reservedIPCreateRequest mirrors go-api-abrha's ReservedIPCreateRequest:
// a reserved IP is created against a region, or already attached to a vm.
type reservedIPCreateRequest struct {
	Region string `json:"region,omitempty"`
	VmID   string `json:"vm_id,omitempty"`
}

// reservedIPActionRequest mirrors the action bodies of reserved_ips_actions.go:
// "assign" carries the target vm_id, "unassign" names only the action type.
type reservedIPActionRequest struct {
	Type string `json:"type"`
	VmID string `json:"vm_id,omitempty"`
}

// ReserveIP reserves a static public IPv4 address in the given region,
// unassigned to any server. The address is billed from reservation, whether
// or not it is attached to a server.
func (c *Client) ReserveIP(ctx context.Context, creds domain.ProviderCredentials, region string) (*domain.ReservedIP, error) {
	req := reservedIPCreateRequest{Region: region}

	var root reservedIPRoot
	if err := c.doJSON(ctx, creds, "POST", reservedIPBasePath, req, &root); err != nil {
		return nil, fmt.Errorf("reserving IP in region %s: %w", region, err)
	}
	if root.ReservedIP == nil {
		return nil, fmt.Errorf("reserving IP in region %s: %w", region, errEmptyResponse)
	}
	return toDomainReservedIP(root.ReservedIP), nil
}

// ReleaseIP removes a reserved IP, releasing the address back to the pool.
func (c *Client) ReleaseIP(ctx context.Context, creds domain.ProviderCredentials, ip string) error {
	if err := c.doJSON(ctx, creds, "DELETE", reservedIPBasePath+"/"+ip, nil, nil); err != nil {
		return fmt.Errorf("releasing reserved IP %s: %w", ip, err)
	}
	return nil
}

// AssignIPToServer attaches an existing reserved IP to a server. The address
// itself is untouched — only the attachment changes. The returned ReservedIP
// reflects the provider's state after the assignment.
func (c *Client) AssignIPToServer(ctx context.Context, creds domain.ProviderCredentials, ip, serverID string) (*domain.ReservedIP, error) {
	req := reservedIPActionRequest{Type: "assign", VmID: serverID}
	if err := c.doJSON(ctx, creds, "POST", reservedIPBasePath+"/"+ip+"/actions", req, nil); err != nil {
		return nil, fmt.Errorf("assigning reserved IP %s to server %s: %w", ip, serverID, err)
	}
	return c.getReservedIP(ctx, creds, ip)
}

// UnassignIP detaches a reserved IP from whatever server it is attached to,
// leaving the address itself reserved and billed.
func (c *Client) UnassignIP(ctx context.Context, creds domain.ProviderCredentials, ip string) (*domain.ReservedIP, error) {
	req := reservedIPActionRequest{Type: "unassign"}
	if err := c.doJSON(ctx, creds, "POST", reservedIPBasePath+"/"+ip+"/actions", req, nil); err != nil {
		return nil, fmt.Errorf("unassigning reserved IP %s: %w", ip, err)
	}
	return c.getReservedIP(ctx, creds, ip)
}

// getReservedIP reads one reserved IP back after an action, so assign/unassign
// can report the address's current state instead of just the action event.
func (c *Client) getReservedIP(ctx context.Context, creds domain.ProviderCredentials, ip string) (*domain.ReservedIP, error) {
	var root reservedIPRoot
	if err := c.doJSON(ctx, creds, "GET", reservedIPBasePath+"/"+ip, nil, &root); err != nil {
		return nil, fmt.Errorf("getting reserved IP %s: %w", ip, err)
	}
	if root.ReservedIP == nil {
		return nil, fmt.Errorf("getting reserved IP %s: %w", ip, errEmptyResponse)
	}
	return toDomainReservedIP(root.ReservedIP), nil
}

// toDomainReservedIP translates a wire reserved IP into the shared domain
// shape. The URN mirrors go-api-abrha's ToURN("ReservedIP", ip), which keeps
// the "do:" prefix of the API family it was forked from.
func toDomainReservedIP(w *reservedIPWire) *domain.ReservedIP {
	ip := &domain.ReservedIP{
		IPAddress: w.IP,
		URN:       fmt.Sprintf("do:reservedip:%s", w.IP),
	}
	if w.Region != nil {
		ip.Region = w.Region.Slug
	}
	if w.Vm != nil {
		ip.ServerID = w.Vm.ID
	}
	return ip
}
