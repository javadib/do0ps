package parspack

import (
	"context"
	"fmt"
	"time"

	"github.com/javadib/do0ps/internal/core/domain"
)

// vpcsBasePath is confirmed against github.com/abrhacom/go-api-abrha's
// vpcs.go (AGENTS.md 4.5: cross-reference that client when the docs site
// cannot be scraped). Relative to Client.baseURL, i.e.
// https://my.parspack.com/cserver/api/public/v1/vpcs.
const vpcsBasePath = "api/public/v1/vpcs"

// The wire types below mirror github.com/abrhacom/go-api-abrha's vpcs.go
// exactly (field names and JSON tags) so this adapter decodes real Parspack
// responses correctly. Nothing above the adapter boundary ever sees these —
// CreateVPC/GetVPC/ListVPCs/DeleteVPC translate them into
// internal/core/domain types.

type vpcWire struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	IPRange     string `json:"ip_range,omitempty"`
	RegionSlug  string `json:"region,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	Default     bool   `json:"default,omitempty"`
}

// vpcCreateRequest mirrors go-api-abrha's VPCCreateRequest.
type vpcCreateRequest struct {
	Name        string `json:"name,omitempty"`
	RegionSlug  string `json:"region,omitempty"`
	Description string `json:"description,omitempty"`
	IPRange     string `json:"ip_range,omitempty"`
}

type vpcRoot struct {
	VPC *vpcWire `json:"vpc"`
}

type vpcsRoot struct {
	VPCs []vpcWire `json:"vpcs"`
}

// CreateVPC provisions an isolated private network. Returns the
// provider-assigned copy, which carries the ID and default flag.
func (c *Client) CreateVPC(ctx context.Context, creds domain.ProviderCredentials, vpc domain.VPC) (*domain.VPC, error) {
	var root vpcRoot
	if err := c.doJSON(ctx, creds, "POST", vpcsBasePath, vpcCreateRequest{
		Name:        vpc.Name,
		RegionSlug:  vpc.Region,
		Description: vpc.Description,
		IPRange:     vpc.IPRange,
	}, &root); err != nil {
		return nil, fmt.Errorf("creating VPC %q: %w", vpc.Name, err)
	}
	if root.VPC == nil {
		return nil, fmt.Errorf("creating VPC %q: %w", vpc.Name, errEmptyResponse)
	}
	return toDomainVPC(root.VPC), nil
}

// GetVPC returns a single VPC by provider ID.
func (c *Client) GetVPC(ctx context.Context, creds domain.ProviderCredentials, id string) (*domain.VPC, error) {
	var root vpcRoot
	if err := c.doJSON(ctx, creds, "GET", vpcsBasePath+"/"+id, nil, &root); err != nil {
		return nil, fmt.Errorf("get VPC %s: %w", id, err)
	}
	if root.VPC == nil {
		return nil, fmt.Errorf("get VPC %s: %w", id, errEmptyResponse)
	}
	return toDomainVPC(root.VPC), nil
}

// ListVPCs returns every VPC visible to the credentials.
func (c *Client) ListVPCs(ctx context.Context, creds domain.ProviderCredentials) ([]domain.VPC, error) {
	var root vpcsRoot
	if err := c.doJSON(ctx, creds, "GET", vpcsBasePath, nil, &root); err != nil {
		return nil, fmt.Errorf("list VPCs: %w", err)
	}

	vpcs := make([]domain.VPC, len(root.VPCs))
	for i := range root.VPCs {
		vpcs[i] = *toDomainVPC(&root.VPCs[i])
	}
	return vpcs, nil
}

// DeleteVPC removes a VPC by provider ID. There is no way to recover a VPC
// once destroyed.
func (c *Client) DeleteVPC(ctx context.Context, creds domain.ProviderCredentials, id string) error {
	if err := c.doJSON(ctx, creds, "DELETE", vpcsBasePath+"/"+id, nil, nil); err != nil {
		return fmt.Errorf("delete VPC %s: %w", id, err)
	}
	return nil
}

// toDomainVPC translates a wire VPC into the shared domain.VPC shape.
func toDomainVPC(v *vpcWire) *domain.VPC {
	vpc := &domain.VPC{
		ID:          v.ID,
		Name:        v.Name,
		Region:      v.RegionSlug,
		Description: v.Description,
		IPRange:     v.IPRange,
		Default:     v.Default,
	}
	if v.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, v.CreatedAt); err == nil {
			vpc.CreatedAt = t
		}
	}
	return vpc
}
