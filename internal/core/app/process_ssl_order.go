package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// ProcessSSLOrderInput is the normalized form of a process_ssl_order tool
// call: submitting the CSR and applicant contact details for a paid order.
type ProcessSSLOrderInput struct {
	Credentials domain.ProviderCredentials
	OrderID     string
	CSR         string
	Contact     domain.SSLContact
}

// ProcessSSLOrder is a fast operation. On success it returns the
// domain-ownership challenges the caller must complete next.
type ProcessSSLOrder struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewProcessSSLOrder builds the use case from its ports.
func NewProcessSSLOrder(queue ports.Queue, provider ports.ParspackProvider) *ProcessSSLOrder {
	return &ProcessSSLOrder{queue: queue, provider: provider}
}

// Execute validates the request and submits it to the provider.
func (uc *ProcessSSLOrder) Execute(ctx context.Context, in ProcessSSLOrderInput) (*domain.SSLChallengeSet, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.OrderID == "" {
		return nil, fmt.Errorf("order_id is required: %w", domain.ErrInvalidInput)
	}
	if in.CSR == "" {
		return nil, fmt.Errorf("csr is required: %w", domain.ErrInvalidInput)
	}
	for field, value := range map[string]string{
		"first_name": in.Contact.FirstName,
		"last_name":  in.Contact.LastName,
		"country":    in.Contact.Country,
		"city":       in.Contact.City,
		"address":    in.Contact.Address,
		"phone":      in.Contact.Phone,
		"email":      in.Contact.Email,
	} {
		if value == "" {
			return nil, fmt.Errorf("%s is required: %w", field, domain.ErrInvalidInput)
		}
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		set, err := uc.provider.ProcessSSLOrder(ctx, in.Credentials, in.OrderID, in.CSR, in.Contact)
		if err != nil {
			return nil, fmt.Errorf("processing SSL order %s: %w", in.OrderID, err)
		}
		return json.Marshal(set)
	})
	if err != nil {
		return nil, err
	}

	var set domain.SSLChallengeSet
	if err := json.Unmarshal(raw, &set); err != nil {
		return nil, fmt.Errorf("decoding SSL order challenges: %w", err)
	}
	return &set, nil
}
