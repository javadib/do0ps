package arvancloud

import (
	"context"
	"fmt"
	"net/http"

	"github.com/javadib/do0ps/internal/core/domain"
)

// Troubleshoot capability (issue #79/AC19): a domain-scoped built-in
// diagnostic tool that checks common configuration issues and reports pass/fail
// for each check. Base paths are confirmed against
// docs/api-specs/arvancloud-cdn-4.0.yml's "Troubleshoot" tag, relative to
// Client.baseURL — i.e. https://napi.arvancloud.ir/cdn/4.0/domains/{domain}/troubleshoots.

func troubleshootDomainPath(domainName string) string {
	return "domains/" + domainName + "/troubleshoots"
}

func troubleshootLatestPath(domainName string) string {
	return "domains/" + domainName + "/troubleshoots/latest"
}

// --- Wire types matching the CDN API's Troubleshoot response shapes --------

type troubleshootDetailWire struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Details string `json:"details"`
}

type troubleshootWire struct {
	ID        string                    `json:"id"`
	Details   []troubleshootDetailWire  `json:"details"`
	CreatedAt string                    `json:"created_at"`
}

// --- Domain conversion helpers ---------------------------------------------

func troubleshootFromWire(w troubleshootWire) domain.ArvanCloudTroubleshoot {
	details := make([]domain.ArvanCloudTroubleshootDetail, len(w.Details))
	for i, d := range w.Details {
		details[i] = domain.ArvanCloudTroubleshootDetail{
			ID:      domain.ArvanCloudTroubleshootDetailID(d.ID),
			Status:  domain.ArvanCloudTroubleshootDetailStatus(d.Status),
			Details: d.Details,
		}
	}
	return domain.ArvanCloudTroubleshoot{
		ID:        w.ID,
		Details:   details,
		CreatedAt: w.CreatedAt,
	}
}

// --- Port implementation ---------------------------------------------------

// ListArvanCloudTroubleshoots returns past troubleshoot runs for a domain.
// Fast operation: GET /domains/{domain}/troubleshoots.
func (p *Provider) ListArvanCloudTroubleshoots(ctx context.Context, creds domain.ProviderCredentials, domainName string) ([]domain.ArvanCloudTroubleshoot, error) {
	var out []troubleshootWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, troubleshootDomainPath(domainName), nil, &out); err != nil {
		return nil, fmt.Errorf("listing arvancloud troubleshoots for %q: %w", domainName, err)
	}
	result := make([]domain.ArvanCloudTroubleshoot, len(out))
	for i, w := range out {
		result[i] = troubleshootFromWire(w)
	}
	return result, nil
}

// RunArvanCloudTroubleshoot starts a new troubleshoot run and returns the
// completed result synchronously. Fast operation:
// POST /domains/{domain}/troubleshoots → 201 with full Troubleshoot object.
func (p *Provider) RunArvanCloudTroubleshoot(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudTroubleshoot, error) {
	var w troubleshootWire
	if err := p.client.doJSON(ctx, creds, http.MethodPost, troubleshootDomainPath(domainName), nil, &w); err != nil {
		return nil, fmt.Errorf("running arvancloud troubleshoot for %q: %w", domainName, err)
	}
	result := troubleshootFromWire(w)
	return &result, nil
}

// GetLatestArvanCloudTroubleshoot returns the most recent troubleshoot run
// for a domain. Fast operation: GET /domains/{domain}/troubleshoots/latest.
func (p *Provider) GetLatestArvanCloudTroubleshoot(ctx context.Context, creds domain.ProviderCredentials, domainName string) (*domain.ArvanCloudTroubleshoot, error) {
	var w troubleshootWire
	if err := p.client.doJSON(ctx, creds, http.MethodGet, troubleshootLatestPath(domainName), nil, &w); err != nil {
		return nil, fmt.Errorf("getting latest arvancloud troubleshoot for %q: %w", domainName, err)
	}
	result := troubleshootFromWire(w)
	return &result, nil
}
