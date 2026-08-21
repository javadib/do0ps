package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// This file holds the FAST account-level Certum certificate use cases for
// ArvanCloud (issue #74/AC14): certificate products, and every order
// operation except issuing one. All fast operations (ports.ArvanCloudProvider,
// AGENTS.md 4.3): each dispatches onto the queue and blocks for the result
// within the same tool call. The one LONG operation this issue adds,
// IssueArvanCloudAccountCertificate, lives in its own file
// (issue_arvancloud_account_certificate.go) because it needs the
// Job/JobRepository machinery these use cases do not.
//
// Deliberately NOT the domain-scoped SSL/TLS settings and managed/uploaded
// certificate workflow (issue #73, arvancloud_ssl.go) — see
// domain/arvancloud_account_ssl.go's package comment.
//
// What IS validated client-side, per issue #74's acceptance criteria: the
// long operation's PrivateKeySize against domain.ValidArvanCloudCertificatePrivateKeySize
// (see issue_arvancloud_account_certificate.go) — nothing in this file takes
// a field with a closed enum to check.

// ListArvanCloudCertificateProductsInput carries the credentials needed to
// list purchasable Certum certificate products. There is nothing else to
// specify: listing is unscoped.
type ListArvanCloudCertificateProductsInput struct {
	Credentials domain.ProviderCredentials
}

// ListArvanCloudCertificateProducts is a fast operation.
type ListArvanCloudCertificateProducts struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudCertificateProducts builds the use case from its ports.
func NewListArvanCloudCertificateProducts(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudCertificateProducts {
	return &ListArvanCloudCertificateProducts{queue: queue, provider: provider}
}

// Execute returns every purchasable Certum certificate product.
func (uc *ListArvanCloudCertificateProducts) Execute(ctx context.Context, in ListArvanCloudCertificateProductsInput) ([]domain.ArvanCloudCertificateProduct, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		products, err := uc.provider.ListArvanCloudCertificateProducts(ctx, in.Credentials)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud certificate products: %w", err)
		}
		return json.Marshal(products)
	})
	if err != nil {
		return nil, err
	}

	var products []domain.ArvanCloudCertificateProduct
	if err := json.Unmarshal(raw, &products); err != nil {
		return nil, fmt.Errorf("decoding arvancloud certificate product list: %w", err)
	}
	return products, nil
}

// ListArvanCloudAccountCertificateOrdersInput carries the credentials needed
// to list the account's Certum order history. There is nothing else to
// specify: listing is unscoped (ports.ArvanCloudProvider's own doc comment —
// there is no per-domain filter on this endpoint).
type ListArvanCloudAccountCertificateOrdersInput struct {
	Credentials domain.ProviderCredentials
}

// ListArvanCloudAccountCertificateOrders is a fast operation.
type ListArvanCloudAccountCertificateOrders struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudAccountCertificateOrders builds the use case from its
// ports.
func NewListArvanCloudAccountCertificateOrders(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudAccountCertificateOrders {
	return &ListArvanCloudAccountCertificateOrders{queue: queue, provider: provider}
}

// Execute returns the account's full Certum order history.
func (uc *ListArvanCloudAccountCertificateOrders) Execute(ctx context.Context, in ListArvanCloudAccountCertificateOrdersInput) ([]domain.ArvanCloudAccountCertificateOrder, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		orders, err := uc.provider.ListArvanCloudAccountCertificateOrders(ctx, in.Credentials)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud account certificate orders: %w", err)
		}
		return json.Marshal(orders)
	})
	if err != nil {
		return nil, err
	}

	var orders []domain.ArvanCloudAccountCertificateOrder
	if err := json.Unmarshal(raw, &orders); err != nil {
		return nil, fmt.Errorf("decoding arvancloud account certificate order list: %w", err)
	}
	return orders, nil
}

// GetArvanCloudAccountCertificateOrderInput identifies the order to look up.
type GetArvanCloudAccountCertificateOrderInput struct {
	Credentials domain.ProviderCredentials
	OrderID     string
	// KeepPrivateKey mirrors the endpoint's own keep_private_key query
	// parameter — see ports.ArvanCloudProvider.GetArvanCloudAccountCertificateOrder's
	// own doc comment. Nil leaves the provider's own default in effect.
	KeepPrivateKey *bool
}

// GetArvanCloudAccountCertificateOrder is a fast operation.
type GetArvanCloudAccountCertificateOrder struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudAccountCertificateOrder builds the use case from its
// ports.
func NewGetArvanCloudAccountCertificateOrder(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudAccountCertificateOrder {
	return &GetArvanCloudAccountCertificateOrder{queue: queue, provider: provider}
}

// Execute returns one order's current state.
func (uc *GetArvanCloudAccountCertificateOrder) Execute(ctx context.Context, in GetArvanCloudAccountCertificateOrderInput) (*domain.ArvanCloudAccountCertificateOrder, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.OrderID == "" {
		return nil, fmt.Errorf("order_id is required: %w", domain.ErrInvalidInput)
	}

	return dispatchArvanCloudAccountCertificateOrder(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudAccountCertificateOrder, error) {
		found, err := uc.provider.GetArvanCloudAccountCertificateOrder(ctx, in.Credentials, in.OrderID, in.KeepPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud account certificate order %q: %w", in.OrderID, err)
		}
		return found, nil
	})
}

// arvanCloudAccountCertificateOrderIDInput is embedded by every use case
// below that is scoped to exactly one order by id and needs nothing else.
type arvanCloudAccountCertificateOrderIDInput struct {
	Credentials domain.ProviderCredentials
	OrderID     string
}

func (in arvanCloudAccountCertificateOrderIDInput) validate() error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.OrderID == "" {
		return fmt.Errorf("order_id is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

// RevokeArvanCloudAccountCertificateInput identifies the order to revoke.
type RevokeArvanCloudAccountCertificateInput = arvanCloudAccountCertificateOrderIDInput

// RevokeArvanCloudAccountCertificate is a fast operation.
type RevokeArvanCloudAccountCertificate struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewRevokeArvanCloudAccountCertificate builds the use case from its ports.
func NewRevokeArvanCloudAccountCertificate(queue ports.Queue, provider ports.ArvanCloudProvider) *RevokeArvanCloudAccountCertificate {
	return &RevokeArvanCloudAccountCertificate{queue: queue, provider: provider}
}

// Execute revokes the order's certificate and returns it as updated.
func (uc *RevokeArvanCloudAccountCertificate) Execute(ctx context.Context, in RevokeArvanCloudAccountCertificateInput) (*domain.ArvanCloudAccountCertificateOrder, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	return dispatchArvanCloudAccountCertificateOrder(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudAccountCertificateOrder, error) {
		updated, err := uc.provider.RevokeArvanCloudAccountCertificate(ctx, in.Credentials, in.OrderID)
		if err != nil {
			return nil, fmt.Errorf("revoking arvancloud account certificate order %q: %w", in.OrderID, err)
		}
		return updated, nil
	})
}

// ReissueArvanCloudAccountCertificateInput identifies the order to reissue.
type ReissueArvanCloudAccountCertificateInput = arvanCloudAccountCertificateOrderIDInput

// ReissueArvanCloudAccountCertificate is a fast operation.
type ReissueArvanCloudAccountCertificate struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewReissueArvanCloudAccountCertificate builds the use case from its ports.
func NewReissueArvanCloudAccountCertificate(queue ports.Queue, provider ports.ArvanCloudProvider) *ReissueArvanCloudAccountCertificate {
	return &ReissueArvanCloudAccountCertificate{queue: queue, provider: provider}
}

// Execute reissues the order's certificate and returns it as updated.
func (uc *ReissueArvanCloudAccountCertificate) Execute(ctx context.Context, in ReissueArvanCloudAccountCertificateInput) (*domain.ArvanCloudAccountCertificateOrder, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	return dispatchArvanCloudAccountCertificateOrder(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudAccountCertificateOrder, error) {
		updated, err := uc.provider.ReissueArvanCloudAccountCertificate(ctx, in.Credentials, in.OrderID)
		if err != nil {
			return nil, fmt.Errorf("reissuing arvancloud account certificate order %q: %w", in.OrderID, err)
		}
		return updated, nil
	})
}

// InstallArvanCloudAccountCertificateInput identifies the order to install.
type InstallArvanCloudAccountCertificateInput = arvanCloudAccountCertificateOrderIDInput

// InstallArvanCloudAccountCertificate is a fast operation — see
// domain.ArvanCloudCertificateInstallResult's own doc comment for why this
// endpoint, unlike issuance, completes synchronously.
type InstallArvanCloudAccountCertificate struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewInstallArvanCloudAccountCertificate builds the use case from its ports.
func NewInstallArvanCloudAccountCertificate(queue ports.Queue, provider ports.ArvanCloudProvider) *InstallArvanCloudAccountCertificate {
	return &InstallArvanCloudAccountCertificate{queue: queue, provider: provider}
}

// Execute installs the order's certificate onto edge servers.
func (uc *InstallArvanCloudAccountCertificate) Execute(ctx context.Context, in InstallArvanCloudAccountCertificateInput) (*domain.ArvanCloudCertificateInstallResult, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		result, err := uc.provider.InstallArvanCloudAccountCertificate(ctx, in.Credentials, in.OrderID)
		if err != nil {
			return nil, fmt.Errorf("installing arvancloud account certificate order %q: %w", in.OrderID, err)
		}
		return json.Marshal(result)
	})
	if err != nil {
		return nil, err
	}

	var result domain.ArvanCloudCertificateInstallResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud certificate install result: %w", err)
	}
	return &result, nil
}

// --- Dispatch helpers -------------------------------------------------

// dispatchArvanCloudAccountCertificateOrder runs fn on the queue and decodes
// its result back into a *domain.ArvanCloudAccountCertificateOrder.
func dispatchArvanCloudAccountCertificateOrder(
	ctx context.Context, queue ports.Queue, fn func(ctx context.Context) (*domain.ArvanCloudAccountCertificateOrder, error),
) (*domain.ArvanCloudAccountCertificateOrder, error) {
	raw, err := queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		result, err := fn(ctx)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
	})
	if err != nil {
		return nil, err
	}

	var result domain.ArvanCloudAccountCertificateOrder
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud account certificate order: %w", err)
	}
	return &result, nil
}
