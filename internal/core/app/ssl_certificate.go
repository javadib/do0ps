package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// GetSSLCertificateInput identifies the order to download the certificate
// for.
type GetSSLCertificateInput struct {
	Credentials domain.ProviderCredentials
	OrderID     string
}

// GetSSLCertificate is a fast operation: it downloads the issued
// certificate, or reports that it is not ready yet (Ready false is not an
// error — the caller is expected to poll).
type GetSSLCertificate struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewGetSSLCertificate builds the use case from its ports.
func NewGetSSLCertificate(queue ports.Queue, provider ports.ParspackProvider) *GetSSLCertificate {
	return &GetSSLCertificate{queue: queue, provider: provider}
}

// Execute returns the certificate's current state.
func (uc *GetSSLCertificate) Execute(ctx context.Context, in GetSSLCertificateInput) (*domain.SSLCertificate, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.OrderID == "" {
		return nil, fmt.Errorf("order_id is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		cert, err := uc.provider.GetSSLCertificate(ctx, in.Credentials, in.OrderID)
		if err != nil {
			return nil, fmt.Errorf("getting certificate for SSL order %s: %w", in.OrderID, err)
		}
		return json.Marshal(cert)
	})
	if err != nil {
		return nil, err
	}

	var cert domain.SSLCertificate
	if err := json.Unmarshal(raw, &cert); err != nil {
		return nil, fmt.Errorf("decoding SSL certificate: %w", err)
	}
	return &cert, nil
}

// ReissueSSLCertificateInput identifies the order and carries the new CSR.
type ReissueSSLCertificateInput struct {
	Credentials domain.ProviderCredentials
	OrderID     string
	CSR         string
}

// ReissueSSLCertificate is a fast operation: it requests a new certificate
// for an already-issued order using a new CSR.
type ReissueSSLCertificate struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewReissueSSLCertificate builds the use case from its ports.
func NewReissueSSLCertificate(queue ports.Queue, provider ports.ParspackProvider) *ReissueSSLCertificate {
	return &ReissueSSLCertificate{queue: queue, provider: provider}
}

// Execute submits the reissue request.
func (uc *ReissueSSLCertificate) Execute(ctx context.Context, in ReissueSSLCertificateInput) (*domain.SSLCertificate, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.OrderID == "" {
		return nil, fmt.Errorf("order_id is required: %w", domain.ErrInvalidInput)
	}
	if in.CSR == "" {
		return nil, fmt.Errorf("csr is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		cert, err := uc.provider.ReissueSSLCertificate(ctx, in.Credentials, in.OrderID, in.CSR)
		if err != nil {
			return nil, fmt.Errorf("reissuing certificate for SSL order %s: %w", in.OrderID, err)
		}
		return json.Marshal(cert)
	})
	if err != nil {
		return nil, err
	}

	var cert domain.SSLCertificate
	if err := json.Unmarshal(raw, &cert); err != nil {
		return nil, fmt.Errorf("decoding reissued SSL certificate: %w", err)
	}
	return &cert, nil
}
