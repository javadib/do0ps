package parspack

import (
	"context"
	"fmt"
	"time"

	"github.com/javadib/do0ps/internal/core/domain"
)

// vmBasePath is confirmed against github.com/abrhacom/go-api-abrha's vms.go
// (AGENTS.md 4.5: cross-reference that client when the docs site cannot be
// scraped). Relative to Client.baseURL, i.e.
// https://my.parspack.com/cserver/api/public/v1/vms.
const vmBasePath = "api/public/v1/vms"

// The wire types below mirror github.com/abrhacom/go-api-abrha's vms.go,
// regions.go, images.go and sizes.go exactly (field names and JSON tags) so
// this adapter decodes real Parspack responses correctly. Nothing above the
// adapter boundary ever sees these — CreateServer/GetServer/ListServers
// translate them into internal/core/domain types.

type vmWire struct {
	ID       string      `json:"id,omitempty"`
	Name     string      `json:"name,omitempty"`
	Memory   int         `json:"memory,omitempty"`
	Vcpus    int         `json:"vcpus,omitempty"`
	Disk     int         `json:"disk,omitempty"`
	Region   *regionWire `json:"region,omitempty"`
	Image    *imageWire  `json:"image,omitempty"`
	Size     *sizeWire   `json:"size,omitempty"`
	SizeSlug string      `json:"size_slug,omitempty"`
	Locked   bool        `json:"locked,omitempty"`
	Status   string      `json:"status,omitempty"`
	Networks *networks   `json:"networks,omitempty"`
	Created  string      `json:"created_at,omitempty"`
	Tags     []string    `json:"tags,omitempty"`
	VPCUUID  string      `json:"vpc_uuid,omitempty"`
}

type regionWire struct {
	Slug string `json:"slug,omitempty"`
	Name string `json:"name,omitempty"`
}

type imageWire struct {
	Slug string `json:"slug,omitempty"`
	Name string `json:"name,omitempty"`
}

type sizeWire struct {
	Slug         string  `json:"slug,omitempty"`
	PriceMonthly float64 `json:"price_monthly,omitempty"`
	PriceHourly  float64 `json:"price_hourly,omitempty"`
}

type networks struct {
	V4 []networkV4 `json:"v4,omitempty"`
	V6 []networkV6 `json:"v6,omitempty"`
}

type networkV4 struct {
	IPAddress string `json:"ip_address,omitempty"`
	Type      string `json:"type,omitempty"` // "public" or "private"
}

type networkV6 struct {
	IPAddress string `json:"ip_address,omitempty"`
	Type      string `json:"type,omitempty"`
}

type vmRoot struct {
	Vm *vmWire `json:"vm"`
}

type vmsRoot struct {
	Vms []vmWire `json:"vms"`
}

// vmCreateRequest mirrors go-api-abrha's VmCreateRequest, restricted to the
// fields AGENTS.md 4.5/issue #9 confirm: name, region, size, image, plus the
// optional fields the "vm" Terraform resource documents (backups, ipv6,
// vpc_uuid, ssh_keys, user_data). Image is sent as a plain slug/ID string —
// domain.ServerSpec never carries the numeric-image-ID variant the upstream
// client's custom marshaler also supports.
type vmCreateRequest struct {
	Name     string   `json:"name"`
	Region   string   `json:"region"`
	Size     string   `json:"size"`
	Image    string   `json:"image"`
	SSHKeys  []string `json:"ssh_keys,omitempty"`
	Backups  bool     `json:"backups"`
	IPv6     bool     `json:"ipv6"`
	VPCUUID  string   `json:"vpc_uuid,omitempty"`
	UserData string   `json:"user_data,omitempty"`
}

// CreateServer provisions a compute instance. Parspack's create endpoint only
// accepts a size slug (AGENTS.md 4.5's confirmed payload: name/region/
// size/image) — a spec built from cpu_cores/ram_mb/disk_gb alone, without a
// plan_id, has no size slug to send and is rejected here rather than guessed
// at.
func (c *Client) CreateServer(ctx context.Context, creds domain.ProviderCredentials, spec domain.ServerSpec) (*domain.Server, error) {
	if spec.PlanID == "" {
		return nil, fmt.Errorf(
			"creating server %q: Parspack requires a plan_id (size slug); specifying cpu_cores/ram_mb/disk_gb "+
				"without one is not supported by this adapter yet: %w", spec.Name, domain.ErrInvalidInput)
	}

	reqBody := vmCreateRequest{
		Name:     spec.Name,
		Region:   spec.Region,
		Size:     spec.PlanID,
		Image:    spec.Image,
		SSHKeys:  spec.SSHKeys,
		Backups:  spec.Backups,
		IPv6:     spec.EnableIPv6,
		VPCUUID:  spec.VPCUUID,
		UserData: spec.UserData,
	}

	var root vmRoot
	if err := c.doJSON(ctx, creds, "POST", vmBasePath, reqBody, &root); err != nil {
		return nil, fmt.Errorf("creating server %q: %w", spec.Name, err)
	}
	if root.Vm == nil {
		return nil, fmt.Errorf("creating server %q: %w", spec.Name, errEmptyResponse)
	}
	return toDomainServer(root.Vm), nil
}

// GetServer returns a single instance by provider ID.
func (c *Client) GetServer(ctx context.Context, creds domain.ProviderCredentials, id string) (*domain.Server, error) {
	var root vmRoot
	if err := c.doJSON(ctx, creds, "GET", vmBasePath+"/"+id, nil, &root); err != nil {
		return nil, fmt.Errorf("get server %s: %w", id, err)
	}
	if root.Vm == nil {
		return nil, fmt.Errorf("get server %s: %w", id, errEmptyResponse)
	}
	return toDomainServer(root.Vm), nil
}

// ListServers returns every instance visible to the credentials.
func (c *Client) ListServers(ctx context.Context, creds domain.ProviderCredentials) ([]domain.Server, error) {
	var root vmsRoot
	if err := c.doJSON(ctx, creds, "GET", vmBasePath, nil, &root); err != nil {
		return nil, fmt.Errorf("list servers: %w", err)
	}

	servers := make([]domain.Server, len(root.Vms))
	for i := range root.Vms {
		servers[i] = *toDomainServer(&root.Vms[i])
	}
	return servers, nil
}

// DeleteServer removes an instance by provider ID.
func (c *Client) DeleteServer(ctx context.Context, creds domain.ProviderCredentials, id string) error {
	if err := c.doJSON(ctx, creds, "DELETE", vmBasePath+"/"+id, nil, nil); err != nil {
		return fmt.Errorf("delete server %s: %w", id, err)
	}
	return nil
}

// FindServerByName supports crash reconciliation. It must return
// domain.ErrNotFound — not a nil server — when no instance matches
// (AGENTS.md 4.4).
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

// toDomainServer translates a wire VM into the shared domain.Server shape.
// Status values (new/active/off/archive) match the DigitalOcean-compatible
// vocabulary Abrha-based APIs use; anything else maps to Unknown rather than
// being guessed at.
func toDomainServer(v *vmWire) *domain.Server {
	s := &domain.Server{
		ID:       v.ID,
		Provider: "parspack",
		Name:     v.Name,
		Status:   parseVMStatus(v.Status),
		CPUCores: v.Vcpus,
		RAMMB:    v.Memory,
		DiskGB:   v.Disk,
		Locked:   v.Locked,
		VPCUUID:  v.VPCUUID,
	}

	if v.Region != nil {
		s.Region = v.Region.Slug
	}
	if v.Image != nil {
		s.Image = v.Image.Slug
	}
	switch {
	case v.Size != nil:
		s.PlanID = v.Size.Slug
		s.PriceHourly = v.Size.PriceHourly
		s.PriceMonthly = v.Size.PriceMonthly
	case v.SizeSlug != "":
		s.PlanID = v.SizeSlug
	}

	if v.Networks != nil {
		for _, n := range v.Networks.V4 {
			switch n.Type {
			case "public":
				s.IPv4 = n.IPAddress
			case "private":
				s.IPv4Private = n.IPAddress
				s.PrivateNetworking = true
			}
		}
		if len(v.Networks.V6) > 0 {
			s.IPv6 = v.Networks.V6[0].IPAddress
		}
	}

	if v.Created != "" {
		if t, err := time.Parse(time.RFC3339, v.Created); err == nil {
			s.CreatedAt = t
		}
	}

	return s
}

func parseVMStatus(s string) domain.ServerStatus {
	switch s {
	case "new":
		return domain.ServerStatusProvisioning
	case "active":
		return domain.ServerStatusRunning
	case "off":
		return domain.ServerStatusStopped
	case "archive":
		return domain.ServerStatusDeleting
	default:
		return domain.ServerStatusUnknown
	}
}
