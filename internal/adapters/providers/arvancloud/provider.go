package arvancloud

import (
	"errors"

	"github.com/javadib/do0ps/internal/core/ports"
)

// Provider implements ports.ArvanCloudProvider on top of the HTTP client in
// client.go. Each capability lands in its own file next to this one, and every
// method is responsible for exactly two things: calling the right endpoint,
// and translating the provider's payload into the domain types. ArvanCloud's
// JSON shapes stay inside this package — nothing above the adapter boundary
// ever sees them.
//
// Sub-resource methods take the domain NAME, not a UUID: the API keys every
// /domains/{domain}/... path on it (see ports.ArvanCloudProvider).
//
// The port has no methods yet, so neither does this type. The struct and the
// interface assertion exist from the start so the capability issues that
// follow add to something already wired rather than introducing it.
type Provider struct {
	client *Client
}

var _ ports.ArvanCloudProvider = (*Provider)(nil)

// NewProvider builds a Provider around an already-configured client. Options
// belong to New, so the composition root configures the transport in one
// place and hands the result here.
func NewProvider(client *Client) (*Provider, error) {
	if client == nil {
		return nil, errors.New("arvancloud client must not be nil")
	}
	return &Provider{client: client}, nil
}

// Client returns the HTTP client this Provider calls through. The capability
// methods added by later issues use the field directly; this accessor is for
// the composition root and for tests, which need to confirm the Provider was
// built around the client they configured.
func (p *Provider) Client() *Client { return p.client }
