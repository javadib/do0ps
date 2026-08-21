package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// This file holds the FAST SSL/TLS use cases for ArvanCloud (issue #73): a
// domain's SSL settings plus the certificates attached to it. All fast
// operations (ports.ArvanCloudProvider, AGENTS.md 4.3): each dispatches onto
// the queue and blocks for the result within the same tool call. The one LONG
// operation this issue adds, IssueArvanCloudManagedCertificate, lives in its
// own file (issue_arvancloud_managed_certificate.go) because it needs the
// Job/JobRepository machinery these use cases do not.
//
// What IS validated client-side, per issue #73's acceptance criteria:
//
//   - UpdateArvanCloudSslSettings's TLSVersion against
//     domain.ValidArvanCloudTlsVersion, HSTSMaxAge against
//     domain.ValidArvanCloudHstsMaxAge (only when set — the spec's own
//     "empty means default"/"only meaningful when HSTSStatus is true"
//     framing leaves it optional) and CertificateKeyType against
//     domain.ValidArvanCloudCertificateKeyType.
//   - UploadArvanCloudCertificate requires both certificate and private_key
//     bytes — an empty upload is rejected client-side rather than sent to
//     the provider only to fail there.

// arvanCloudDomainOnlyInput is embedded by every use case below that is
// scoped to exactly one domain by name and needs nothing else.
type arvanCloudDomainOnlyInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
}

func (in arvanCloudDomainOnlyInput) validate() error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.Domain == "" {
		return fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

// --- Per-domain SSL settings -------------------------------------------

// GetArvanCloudSslSettingsInput identifies the domain whose SSL settings to
// look up.
type GetArvanCloudSslSettingsInput = arvanCloudDomainOnlyInput

// GetArvanCloudSslSettings is a fast operation.
type GetArvanCloudSslSettings struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudSslSettings builds the use case from its ports.
func NewGetArvanCloudSslSettings(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudSslSettings {
	return &GetArvanCloudSslSettings{queue: queue, provider: provider}
}

// Execute returns the domain's current SSL/TLS settings.
func (uc *GetArvanCloudSslSettings) Execute(ctx context.Context, in GetArvanCloudSslSettingsInput) (*domain.ArvanCloudSslSettings, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudSslSettings(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudSslSettings, error) {
		found, err := uc.provider.GetArvanCloudSslSettings(ctx, in.Credentials, in.Domain)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud ssl settings for domain %q: %w", in.Domain, err)
		}
		return found, nil
	})
}

// validateArvanCloudSslSettingsInput checks the fields an update SSL
// settings call must satisfy. See this file's package comment.
func validateArvanCloudSslSettingsInput(settings domain.ArvanCloudSslSettings) error {
	if !domain.ValidArvanCloudTlsVersion(string(settings.TLSVersion)) {
		return fmt.Errorf(
			"tls_version %q is not one of \"\", \"TLSv1\", \"TLSv1.1\", \"TLSv1.2\" or \"TLSv1.3\": %w",
			settings.TLSVersion, domain.ErrInvalidInput)
	}
	if settings.HSTSMaxAge != "" && !domain.ValidArvanCloudHstsMaxAge(settings.HSTSMaxAge) {
		return fmt.Errorf(
			"hsts_max_age %q is not one of \"1mo\", \"2mo\", \"3mo\", \"4mo\", \"5mo\", \"6mo\", \"12mo\" or \"24mo\": %w",
			settings.HSTSMaxAge, domain.ErrInvalidInput)
	}
	if !domain.ValidArvanCloudCertificateKeyType(string(settings.CertificateKeyType)) {
		return fmt.Errorf(
			"certificate_key_type %q is not one of \"rsa\" or \"ec\": %w",
			settings.CertificateKeyType, domain.ErrInvalidInput)
	}
	return nil
}

// UpdateArvanCloudSslSettingsInput identifies the domain and its new SSL/TLS
// configuration.
type UpdateArvanCloudSslSettingsInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	Settings    domain.ArvanCloudSslSettings
}

// UpdateArvanCloudSslSettings changes a domain's SSL/TLS configuration. This
// is a fast operation.
type UpdateArvanCloudSslSettings struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUpdateArvanCloudSslSettings builds the use case from its ports.
func NewUpdateArvanCloudSslSettings(queue ports.Queue, provider ports.ArvanCloudProvider) *UpdateArvanCloudSslSettings {
	return &UpdateArvanCloudSslSettings{queue: queue, provider: provider}
}

// Execute validates the request — see validateArvanCloudSslSettingsInput —
// and updates the settings, returning them as stored afterward.
func (uc *UpdateArvanCloudSslSettings) Execute(ctx context.Context, in UpdateArvanCloudSslSettingsInput) (*domain.ArvanCloudSslSettings, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Domain == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if err := validateArvanCloudSslSettingsInput(in.Settings); err != nil {
		return nil, err
	}

	return dispatchArvanCloudSslSettings(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudSslSettings, error) {
		updated, err := uc.provider.UpdateArvanCloudSslSettings(ctx, in.Credentials, in.Domain, in.Settings)
		if err != nil {
			return nil, fmt.Errorf("updating arvancloud ssl settings for domain %q: %w", in.Domain, err)
		}
		return updated, nil
	})
}

// --- Certificates ---------------------------------------------------------

// ListArvanCloudCertificatesInput identifies the domain whose certificates to
// list.
type ListArvanCloudCertificatesInput = arvanCloudDomainOnlyInput

// ListArvanCloudCertificates is a fast operation.
type ListArvanCloudCertificates struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudCertificates builds the use case from its ports.
func NewListArvanCloudCertificates(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudCertificates {
	return &ListArvanCloudCertificates{queue: queue, provider: provider}
}

// Execute returns every certificate attached to the domain.
func (uc *ListArvanCloudCertificates) Execute(ctx context.Context, in ListArvanCloudCertificatesInput) ([]domain.ArvanCloudCertificate, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		certs, err := uc.provider.ListArvanCloudCertificates(ctx, in.Credentials, in.Domain)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud certificates of domain %q: %w", in.Domain, err)
		}
		return json.Marshal(certs)
	})
	if err != nil {
		return nil, err
	}

	var certs []domain.ArvanCloudCertificate
	if err := json.Unmarshal(raw, &certs); err != nil {
		return nil, fmt.Errorf("decoding arvancloud certificate list: %w", err)
	}
	return certs, nil
}

// UploadArvanCloudCertificateInput is the normalized form of an
// upload_arvancloud_certificate tool call. PrivateKeyPEM is caller-supplied
// sensitive material — see ports.ArvanCloudProvider.UploadArvanCloudCertificate's
// doc comment; this use case only ever forwards it to the provider and never
// logs it.
type UploadArvanCloudCertificateInput struct {
	Credentials    domain.ProviderCredentials
	Domain         string
	CertificatePEM []byte
	PrivateKeyPEM  []byte
}

// UploadArvanCloudCertificate stores a customer-owned certificate. This is a
// fast operation. The endpoint's response carries no data (see the port
// method's own doc comment), so Execute returns only an error; a caller
// wanting the new certificate's ID calls ListArvanCloudCertificates
// afterward.
type UploadArvanCloudCertificate struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUploadArvanCloudCertificate builds the use case from its ports.
func NewUploadArvanCloudCertificate(queue ports.Queue, provider ports.ArvanCloudProvider) *UploadArvanCloudCertificate {
	return &UploadArvanCloudCertificate{queue: queue, provider: provider}
}

// Execute validates the request and uploads the certificate.
func (uc *UploadArvanCloudCertificate) Execute(ctx context.Context, in UploadArvanCloudCertificateInput) error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.Domain == "" {
		return fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if len(in.CertificatePEM) == 0 {
		return fmt.Errorf("certificate is required: %w", domain.ErrInvalidInput)
	}
	if len(in.PrivateKeyPEM) == 0 {
		return fmt.Errorf("private_key is required: %w", domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.UploadArvanCloudCertificate(ctx, in.Credentials, in.Domain, in.CertificatePEM, in.PrivateKeyPEM); err != nil {
			return nil, fmt.Errorf("uploading arvancloud certificate for domain %q: %w", in.Domain, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// arvanCloudCertificateIDInput is embedded by every use case below that is
// scoped to exactly one certificate by domain + id.
type arvanCloudCertificateIDInput struct {
	Credentials   domain.ProviderCredentials
	Domain        string
	CertificateID string
}

func (in arvanCloudCertificateIDInput) validate() error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.Domain == "" {
		return fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if in.CertificateID == "" {
		return fmt.Errorf("certificate_id is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

// GetArvanCloudCertificateInput identifies the certificate to look up.
type GetArvanCloudCertificateInput = arvanCloudCertificateIDInput

// GetArvanCloudCertificate is a fast operation.
type GetArvanCloudCertificate struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudCertificate builds the use case from its ports.
func NewGetArvanCloudCertificate(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudCertificate {
	return &GetArvanCloudCertificate{queue: queue, provider: provider}
}

// Execute returns the current state of one certificate.
func (uc *GetArvanCloudCertificate) Execute(ctx context.Context, in GetArvanCloudCertificateInput) (*domain.ArvanCloudCertificate, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		found, err := uc.provider.GetArvanCloudCertificate(ctx, in.Credentials, in.Domain, in.CertificateID)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud certificate %q on domain %q: %w", in.CertificateID, in.Domain, err)
		}
		return json.Marshal(found)
	})
	if err != nil {
		return nil, err
	}

	var cert domain.ArvanCloudCertificate
	if err := json.Unmarshal(raw, &cert); err != nil {
		return nil, fmt.Errorf("decoding arvancloud certificate: %w", err)
	}
	return &cert, nil
}

// DeleteArvanCloudCertificateInput identifies the certificate to remove.
type DeleteArvanCloudCertificateInput = arvanCloudCertificateIDInput

// DeleteArvanCloudCertificate is a fast operation. Deleting a certificate
// the provider no longer has is treated as already done rather than an
// error, matching DeleteArvanCloudDdosRule's tolerant-delete contract.
type DeleteArvanCloudCertificate struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewDeleteArvanCloudCertificate builds the use case from its ports.
func NewDeleteArvanCloudCertificate(queue ports.Queue, provider ports.ArvanCloudProvider) *DeleteArvanCloudCertificate {
	return &DeleteArvanCloudCertificate{queue: queue, provider: provider}
}

// Execute deletes the certificate, tolerating one that is already gone.
func (uc *DeleteArvanCloudCertificate) Execute(ctx context.Context, in DeleteArvanCloudCertificateInput) error {
	if err := in.validate(); err != nil {
		return err
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteArvanCloudCertificate(ctx, in.Credentials, in.Domain, in.CertificateID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting arvancloud certificate %q on domain %q: %w", in.CertificateID, in.Domain, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// RevokeArvanCloudCertificateInput identifies the certificate to revoke.
type RevokeArvanCloudCertificateInput = arvanCloudCertificateIDInput

// RevokeArvanCloudCertificate is a fast operation.
type RevokeArvanCloudCertificate struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewRevokeArvanCloudCertificate builds the use case from its ports.
func NewRevokeArvanCloudCertificate(queue ports.Queue, provider ports.ArvanCloudProvider) *RevokeArvanCloudCertificate {
	return &RevokeArvanCloudCertificate{queue: queue, provider: provider}
}

// Execute revokes the certificate.
func (uc *RevokeArvanCloudCertificate) Execute(ctx context.Context, in RevokeArvanCloudCertificateInput) error {
	if err := in.validate(); err != nil {
		return err
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.RevokeArvanCloudCertificate(ctx, in.Credentials, in.Domain, in.CertificateID); err != nil {
			return nil, fmt.Errorf("revoking arvancloud certificate %q on domain %q: %w", in.CertificateID, in.Domain, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// --- Managed-certificate orders --------------------------------------------

// ListArvanCloudSslOrdersInput identifies the domain whose order history to
// list.
type ListArvanCloudSslOrdersInput = arvanCloudDomainOnlyInput

// ListArvanCloudSslOrders is a fast operation.
type ListArvanCloudSslOrders struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudSslOrders builds the use case from its ports.
func NewListArvanCloudSslOrders(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudSslOrders {
	return &ListArvanCloudSslOrders{queue: queue, provider: provider}
}

// Execute returns the domain's managed-certificate order history.
func (uc *ListArvanCloudSslOrders) Execute(ctx context.Context, in ListArvanCloudSslOrdersInput) ([]domain.ArvanCloudCertificateOrder, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		orders, err := uc.provider.ListArvanCloudSslOrders(ctx, in.Credentials, in.Domain)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud ssl orders of domain %q: %w", in.Domain, err)
		}
		return json.Marshal(orders)
	})
	if err != nil {
		return nil, err
	}

	var orders []domain.ArvanCloudCertificateOrder
	if err := json.Unmarshal(raw, &orders); err != nil {
		return nil, fmt.Errorf("decoding arvancloud ssl order list: %w", err)
	}
	return orders, nil
}

// RetryArvanCloudSslOrderInput identifies the domain whose killed order to
// retry.
type RetryArvanCloudSslOrderInput = arvanCloudDomainOnlyInput

// RetryArvanCloudSslOrder is a fast operation. The endpoint's response
// carries no data, so Execute returns only an error; a caller wanting the
// retried order's state afterward calls ListArvanCloudSslOrders.
type RetryArvanCloudSslOrder struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewRetryArvanCloudSslOrder builds the use case from its ports.
func NewRetryArvanCloudSslOrder(queue ports.Queue, provider ports.ArvanCloudProvider) *RetryArvanCloudSslOrder {
	return &RetryArvanCloudSslOrder{queue: queue, provider: provider}
}

// Execute retries a previously "killed" order for the domain.
func (uc *RetryArvanCloudSslOrder) Execute(ctx context.Context, in RetryArvanCloudSslOrderInput) error {
	if err := in.validate(); err != nil {
		return err
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.RetryArvanCloudSslOrder(ctx, in.Credentials, in.Domain); err != nil {
			return nil, fmt.Errorf("retrying arvancloud ssl order for domain %q: %w", in.Domain, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// --- Dispatch helpers -------------------------------------------------

// dispatchArvanCloudSslSettings runs fn on the queue and decodes its result
// back into a *domain.ArvanCloudSslSettings.
func dispatchArvanCloudSslSettings(
	ctx context.Context, queue ports.Queue, fn func(ctx context.Context) (*domain.ArvanCloudSslSettings, error),
) (*domain.ArvanCloudSslSettings, error) {
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

	var result domain.ArvanCloudSslSettings
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud ssl settings: %w", err)
	}
	return &result, nil
}
