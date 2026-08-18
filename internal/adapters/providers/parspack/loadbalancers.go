package parspack

import (
	"context"
	"fmt"
	"time"

	"github.com/javadib/do0ps/internal/core/domain"
)

// loadBalancersBasePath is confirmed against github.com/abrhacom/go-api-abrha's
// load_balancers.go (AGENTS.md 4.5: cross-reference that client when the docs
// site cannot be scraped). Relative to Client.baseURL, i.e.
// https://my.parspack.com/cserver/api/public/v1/load_balancers.
//
// These are the cloud-server/VM-network-level load balancers of the Abrha-based
// cloud-server API. The CDN API has its own, unrelated edge-level Load Balance
// concept (issue #24) on a different base URL — nothing here touches it.
const loadBalancersBasePath = "api/public/v1/load_balancers"

// The wire types below mirror github.com/abrhacom/go-api-abrha's
// load_balancers.go exactly (field names and JSON tags), restricted to the
// fields domain.LoadBalancer carries: certificates, sticky sessions, proxy
// protocol, size, type and the rest of the upstream surface are deliberately
// not modeled. Nothing above the adapter boundary ever sees these — the CRUD
// methods below translate them into internal/core/domain types.

type loadBalancerWire struct {
	ID                  string               `json:"id,omitempty"`
	Name                string               `json:"name,omitempty"`
	IP                  string               `json:"ip,omitempty"`
	Algorithm           string               `json:"algorithm,omitempty"`
	Status              string               `json:"status,omitempty"`
	ForwardingRules     []forwardingRuleWire `json:"forwarding_rules,omitempty"`
	HealthCheck         *healthCheckWire     `json:"health_check,omitempty"`
	Region              *regionWire          `json:"region,omitempty"`
	VmIDs               []string             `json:"vm_ids,omitempty"`
	RedirectHTTPToHTTPS bool                 `json:"redirect_http_to_https,omitempty"`
	VPCUUID             string               `json:"vpc_uuid,omitempty"`
	Created             string               `json:"created_at,omitempty"`
}

type forwardingRuleWire struct {
	EntryProtocol  string `json:"entry_protocol,omitempty"`
	EntryPort      int    `json:"entry_port,omitempty"`
	TargetProtocol string `json:"target_protocol,omitempty"`
	TargetPort     int    `json:"target_port,omitempty"`
}

type healthCheckWire struct {
	Protocol               string `json:"protocol,omitempty"`
	Port                   int    `json:"port,omitempty"`
	Path                   string `json:"path,omitempty"`
	CheckIntervalSeconds   int    `json:"check_interval_seconds,omitempty"`
	ResponseTimeoutSeconds int    `json:"response_timeout_seconds,omitempty"`
	UnhealthyThreshold     int    `json:"unhealthy_threshold,omitempty"`
	HealthyThreshold       int    `json:"healthy_threshold,omitempty"`
}

// loadBalancerRoot mirrors the single-resource envelope {"load_balancer":
// {...}}.
type loadBalancerRoot struct {
	LoadBalancer *loadBalancerWire `json:"load_balancer"`
}

// loadBalancersRoot mirrors the list envelope {"load_balancers": [...]}.
type loadBalancersRoot struct {
	LoadBalancers []loadBalancerWire `json:"load_balancers"`
}

// loadBalancerRequest is what Create/Update send, mirroring go-api-abrha's
// LoadBalancerRequest restricted to the fields domain.LoadBalancer supports.
type loadBalancerRequest struct {
	Name                string               `json:"name"`
	Algorithm           string               `json:"algorithm,omitempty"`
	Region              string               `json:"region,omitempty"`
	ForwardingRules     []forwardingRuleWire `json:"forwarding_rules,omitempty"`
	HealthCheck         *healthCheckWire     `json:"health_check,omitempty"`
	VmIDs               []string             `json:"vm_ids,omitempty"`
	RedirectHTTPToHTTPS bool                 `json:"redirect_http_to_https,omitempty"`
	VPCUUID             string               `json:"vpc_uuid,omitempty"`
}

// CreateLoadBalancer provisions a new load balancer. The provider returns the
// balancer in "new" status; it is ready once the status turns "active".
func (c *Client) CreateLoadBalancer(ctx context.Context, creds domain.ProviderCredentials, lb domain.LoadBalancer) (*domain.LoadBalancer, error) {
	var root loadBalancerRoot
	if err := c.doJSON(ctx, creds, "POST", loadBalancersBasePath, toLoadBalancerRequest(lb), &root); err != nil {
		return nil, fmt.Errorf("creating load balancer %q: %w", lb.Name, err)
	}
	if root.LoadBalancer == nil {
		return nil, fmt.Errorf("creating load balancer %q: %w", lb.Name, errEmptyResponse)
	}
	return toDomainLoadBalancer(root.LoadBalancer), nil
}

// GetLoadBalancer returns a single load balancer by provider ID.
func (c *Client) GetLoadBalancer(ctx context.Context, creds domain.ProviderCredentials, id string) (*domain.LoadBalancer, error) {
	var root loadBalancerRoot
	if err := c.doJSON(ctx, creds, "GET", loadBalancersBasePath+"/"+id, nil, &root); err != nil {
		return nil, fmt.Errorf("get load balancer %s: %w", id, err)
	}
	if root.LoadBalancer == nil {
		return nil, fmt.Errorf("get load balancer %s: %w", id, errEmptyResponse)
	}
	return toDomainLoadBalancer(root.LoadBalancer), nil
}

// ListLoadBalancers returns every load balancer visible to the credentials.
func (c *Client) ListLoadBalancers(ctx context.Context, creds domain.ProviderCredentials) ([]domain.LoadBalancer, error) {
	var root loadBalancersRoot
	if err := c.doJSON(ctx, creds, "GET", loadBalancersBasePath, nil, &root); err != nil {
		return nil, fmt.Errorf("list load balancers: %w", err)
	}

	balancers := make([]domain.LoadBalancer, len(root.LoadBalancers))
	for i := range root.LoadBalancers {
		balancers[i] = *toDomainLoadBalancer(&root.LoadBalancers[i])
	}
	return balancers, nil
}

// UpdateLoadBalancer replaces a load balancer's configuration by provider ID.
func (c *Client) UpdateLoadBalancer(ctx context.Context, creds domain.ProviderCredentials, id string, lb domain.LoadBalancer) (*domain.LoadBalancer, error) {
	var root loadBalancerRoot
	if err := c.doJSON(ctx, creds, "PUT", loadBalancersBasePath+"/"+id, toLoadBalancerRequest(lb), &root); err != nil {
		return nil, fmt.Errorf("update load balancer %s: %w", id, err)
	}
	if root.LoadBalancer == nil {
		return nil, fmt.Errorf("update load balancer %s: %w", id, errEmptyResponse)
	}
	return toDomainLoadBalancer(root.LoadBalancer), nil
}

// DeleteLoadBalancer removes a load balancer by provider ID.
func (c *Client) DeleteLoadBalancer(ctx context.Context, creds domain.ProviderCredentials, id string) error {
	if err := c.doJSON(ctx, creds, "DELETE", loadBalancersBasePath+"/"+id, nil, nil); err != nil {
		return fmt.Errorf("delete load balancer %s: %w", id, err)
	}
	return nil
}

// FindLoadBalancerByName supports crash reconciliation. It must return
// domain.ErrNotFound — not a nil load balancer — when no balancer matches
// (AGENTS.md 4.4).
func (c *Client) FindLoadBalancerByName(ctx context.Context, creds domain.ProviderCredentials, name string) (*domain.LoadBalancer, error) {
	balancers, err := c.ListLoadBalancers(ctx, creds)
	if err != nil {
		return nil, err
	}
	for i := range balancers {
		if balancers[i].Name == name {
			return &balancers[i], nil
		}
	}
	return nil, fmt.Errorf("load balancer %q: %w", name, domain.ErrNotFound)
}

// toLoadBalancerRequest translates a domain.LoadBalancer into the wire
// request. The read-only fields (ID, IP, Status, CreatedAt) are not part of
// the request.
func toLoadBalancerRequest(lb domain.LoadBalancer) loadBalancerRequest {
	req := loadBalancerRequest{
		Name:                lb.Name,
		Algorithm:           lb.Algorithm,
		Region:              lb.Region,
		VmIDs:               lb.ServerIDs,
		RedirectHTTPToHTTPS: lb.RedirectHTTPToTLS,
		VPCUUID:             lb.VPCUUID,
	}
	for _, r := range lb.ForwardingRules {
		req.ForwardingRules = append(req.ForwardingRules, forwardingRuleWire{
			EntryProtocol:  r.EntryProtocol,
			EntryPort:      r.EntryPort,
			TargetProtocol: r.TargetProtocol,
			TargetPort:     r.TargetPort,
		})
	}
	if lb.HealthCheck != nil {
		req.HealthCheck = &healthCheckWire{
			Protocol:               lb.HealthCheck.Protocol,
			Port:                   lb.HealthCheck.Port,
			Path:                   lb.HealthCheck.Path,
			CheckIntervalSeconds:   lb.HealthCheck.CheckIntervalSeconds,
			ResponseTimeoutSeconds: lb.HealthCheck.ResponseTimeoutSeconds,
			UnhealthyThreshold:     lb.HealthCheck.UnhealthyThreshold,
			HealthyThreshold:       lb.HealthCheck.HealthyThreshold,
		}
	}
	return req
}

// toDomainLoadBalancer translates a wire load balancer into the shared domain
// shape.
func toDomainLoadBalancer(l *loadBalancerWire) *domain.LoadBalancer {
	lb := &domain.LoadBalancer{
		ID:                l.ID,
		Name:              l.Name,
		Algorithm:         l.Algorithm,
		IP:                l.IP,
		Status:            l.Status,
		ServerIDs:         l.VmIDs,
		RedirectHTTPToTLS: l.RedirectHTTPToHTTPS,
		VPCUUID:           l.VPCUUID,
	}
	if l.Region != nil {
		lb.Region = l.Region.Slug
	}
	for _, r := range l.ForwardingRules {
		lb.ForwardingRules = append(lb.ForwardingRules, domain.ForwardingRule{
			EntryProtocol:  r.EntryProtocol,
			EntryPort:      r.EntryPort,
			TargetProtocol: r.TargetProtocol,
			TargetPort:     r.TargetPort,
		})
	}
	if l.HealthCheck != nil {
		lb.HealthCheck = &domain.LoadBalancerHealthCheck{
			Protocol:               l.HealthCheck.Protocol,
			Port:                   l.HealthCheck.Port,
			Path:                   l.HealthCheck.Path,
			CheckIntervalSeconds:   l.HealthCheck.CheckIntervalSeconds,
			ResponseTimeoutSeconds: l.HealthCheck.ResponseTimeoutSeconds,
			UnhealthyThreshold:     l.HealthCheck.UnhealthyThreshold,
			HealthyThreshold:       l.HealthCheck.HealthyThreshold,
		}
	}
	if l.Created != "" {
		if t, err := time.Parse(time.RFC3339, l.Created); err == nil {
			lb.CreatedAt = t
		}
	}
	return lb
}
