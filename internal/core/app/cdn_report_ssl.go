package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// The use cases below cover the read-only CDN report/analytics endpoints and
// the CDN-zone-level SSL settings (issue #24): access/security/error/WAF
// logs, top visitors, monthly traffic usage, minimum TLS version, attached
// certificates (read-only), and HSTS. All of them are FAST operations: each
// dispatches onto the worker pool and the caller waits for the result within
// the same tool call (AGENTS.md 4.3) — there is nothing here that runs for
// minutes.
//
// ports.ParspackProvider does not yet declare these methods (they are being
// integrated centrally, see the issue), so each use case below is typed
// against a small local interface declaring only the one or two provider
// methods it needs. *parspack.Client already implements every one of these
// methods, so it satisfies each local interface automatically via Go's
// structural typing — no explicit "implements" declaration is required.

// validateZoneUUID (shared by every CDN use case that scopes to one zone) is
// defined once in cdn_cache.go.

// validateCDNLogQuery checks the shared log filter against the enums the CDN
// API confirms, so a bad step value fails fast here instead of reaching the
// provider and coming back as a 422.
func validateCDNLogQuery(q domain.CDNLogQuery) error {
	if !domain.ValidCDNLogStep(q.Step) {
		return fmt.Errorf("step %d is not one of the values Parspack accepts (10, 25, 50, 100): %w", q.Step, domain.ErrInvalidInput)
	}
	return nil
}

// GetCDNAccessLogInput identifies the zone and page/filter of access log to
// fetch.
type GetCDNAccessLogInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Query       domain.CDNLogQuery
}

// GetCDNAccessLog is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type GetCDNAccessLog struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewGetCDNAccessLog builds the use case from its ports.
func NewGetCDNAccessLog(queue ports.Queue, provider ports.ParspackProvider) *GetCDNAccessLog {
	return &GetCDNAccessLog{queue: queue, provider: provider}
}

// Execute returns one page of a zone's access log.
func (uc *GetCDNAccessLog) Execute(ctx context.Context, in GetCDNAccessLogInput) (*domain.CDNAccessLogPage, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if err := validateZoneUUID(in.ZoneUUID); err != nil {
		return nil, err
	}
	if err := validateCDNLogQuery(in.Query); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		page, err := uc.provider.GetCDNAccessLog(ctx, in.Credentials, in.ZoneUUID, in.Query)
		if err != nil {
			return nil, fmt.Errorf("getting access log for zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(page)
	})
	if err != nil {
		return nil, err
	}

	var page domain.CDNAccessLogPage
	if err := json.Unmarshal(raw, &page); err != nil {
		return nil, fmt.Errorf("decoding access log page: %w", err)
	}
	return &page, nil
}

// GetCDNSecurityLogInput identifies the zone and page/filter of security log
// to fetch.
type GetCDNSecurityLogInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Query       domain.CDNLogQuery
}

// GetCDNSecurityLog is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type GetCDNSecurityLog struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewGetCDNSecurityLog builds the use case from its ports.
func NewGetCDNSecurityLog(queue ports.Queue, provider ports.ParspackProvider) *GetCDNSecurityLog {
	return &GetCDNSecurityLog{queue: queue, provider: provider}
}

// Execute returns one page of a zone's security log.
func (uc *GetCDNSecurityLog) Execute(ctx context.Context, in GetCDNSecurityLogInput) (*domain.CDNSecurityLogPage, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if err := validateZoneUUID(in.ZoneUUID); err != nil {
		return nil, err
	}
	if err := validateCDNLogQuery(in.Query); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		page, err := uc.provider.GetCDNSecurityLog(ctx, in.Credentials, in.ZoneUUID, in.Query)
		if err != nil {
			return nil, fmt.Errorf("getting security log for zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(page)
	})
	if err != nil {
		return nil, err
	}

	var page domain.CDNSecurityLogPage
	if err := json.Unmarshal(raw, &page); err != nil {
		return nil, fmt.Errorf("decoding security log page: %w", err)
	}
	return &page, nil
}

// GetCDNErrorLogInput identifies the zone and page/filter of error log to
// fetch.
type GetCDNErrorLogInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Query       domain.CDNLogQuery
}

// GetCDNErrorLog is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type GetCDNErrorLog struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewGetCDNErrorLog builds the use case from its ports.
func NewGetCDNErrorLog(queue ports.Queue, provider ports.ParspackProvider) *GetCDNErrorLog {
	return &GetCDNErrorLog{queue: queue, provider: provider}
}

// Execute returns one page of a zone's error log.
func (uc *GetCDNErrorLog) Execute(ctx context.Context, in GetCDNErrorLogInput) (*domain.CDNErrorLogPage, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if err := validateZoneUUID(in.ZoneUUID); err != nil {
		return nil, err
	}
	if err := validateCDNLogQuery(in.Query); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		page, err := uc.provider.GetCDNErrorLog(ctx, in.Credentials, in.ZoneUUID, in.Query)
		if err != nil {
			return nil, fmt.Errorf("getting error log for zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(page)
	})
	if err != nil {
		return nil, err
	}

	var page domain.CDNErrorLogPage
	if err := json.Unmarshal(raw, &page); err != nil {
		return nil, fmt.Errorf("decoding error log page: %w", err)
	}
	return &page, nil
}

// GetCDNWAFLogInput identifies the zone and page/filter of WAF log to fetch.
type GetCDNWAFLogInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Query       domain.CDNLogQuery
}

// GetCDNWAFLog is a fast operation: it runs on a worker but the caller waits
// for the result inside the same tool call.
type GetCDNWAFLog struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewGetCDNWAFLog builds the use case from its ports.
func NewGetCDNWAFLog(queue ports.Queue, provider ports.ParspackProvider) *GetCDNWAFLog {
	return &GetCDNWAFLog{queue: queue, provider: provider}
}

// Execute returns one page of a zone's WAF log.
func (uc *GetCDNWAFLog) Execute(ctx context.Context, in GetCDNWAFLogInput) (*domain.CDNWAFLogPage, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if err := validateZoneUUID(in.ZoneUUID); err != nil {
		return nil, err
	}
	if err := validateCDNLogQuery(in.Query); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		page, err := uc.provider.GetCDNWAFLog(ctx, in.Credentials, in.ZoneUUID, in.Query)
		if err != nil {
			return nil, fmt.Errorf("getting WAF log for zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(page)
	})
	if err != nil {
		return nil, err
	}

	var page domain.CDNWAFLogPage
	if err := json.Unmarshal(raw, &page); err != nil {
		return nil, fmt.Errorf("decoding WAF log page: %w", err)
	}
	return &page, nil
}

// GetCDNTopVisitorsInput identifies the zone and required date range.
type GetCDNTopVisitorsInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Start       string // "YYYY-MM-DD", required
	End         string // "YYYY-MM-DD", required
}

// GetCDNTopVisitors is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type GetCDNTopVisitors struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewGetCDNTopVisitors builds the use case from its ports.
func NewGetCDNTopVisitors(queue ports.Queue, provider ports.ParspackProvider) *GetCDNTopVisitors {
	return &GetCDNTopVisitors{queue: queue, provider: provider}
}

// Execute returns the top visitor IPs for a zone within the given date
// range. start and end are required by the provider.
func (uc *GetCDNTopVisitors) Execute(ctx context.Context, in GetCDNTopVisitorsInput) ([]domain.CDNTopVisitor, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if err := validateZoneUUID(in.ZoneUUID); err != nil {
		return nil, err
	}
	if in.Start == "" {
		return nil, fmt.Errorf("start is required: %w", domain.ErrInvalidInput)
	}
	if in.End == "" {
		return nil, fmt.Errorf("end is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		visitors, err := uc.provider.GetCDNTopVisitors(ctx, in.Credentials, in.ZoneUUID, in.Start, in.End)
		if err != nil {
			return nil, fmt.Errorf("getting top visitors for zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(visitors)
	})
	if err != nil {
		return nil, err
	}

	var visitors []domain.CDNTopVisitor
	if err := json.Unmarshal(raw, &visitors); err != nil {
		return nil, fmt.Errorf("decoding top visitors: %w", err)
	}
	return visitors, nil
}

// GetCDNMonthlyTrafficUsage needs.

// GetCDNMonthlyTrafficUsageInput identifies the zone to look up.
type GetCDNMonthlyTrafficUsageInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
}

// GetCDNMonthlyTrafficUsage is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type GetCDNMonthlyTrafficUsage struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewGetCDNMonthlyTrafficUsage builds the use case from its ports.
func NewGetCDNMonthlyTrafficUsage(queue ports.Queue, provider ports.ParspackProvider) *GetCDNMonthlyTrafficUsage {
	return &GetCDNMonthlyTrafficUsage{queue: queue, provider: provider}
}

// Execute returns a zone's current-month traffic usage and plan limit.
func (uc *GetCDNMonthlyTrafficUsage) Execute(ctx context.Context, in GetCDNMonthlyTrafficUsageInput) (*domain.CDNTrafficUsage, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if err := validateZoneUUID(in.ZoneUUID); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		usage, err := uc.provider.GetCDNMonthlyTrafficUsage(ctx, in.Credentials, in.ZoneUUID)
		if err != nil {
			return nil, fmt.Errorf("getting monthly traffic usage for zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(usage)
	})
	if err != nil {
		return nil, err
	}

	var usage domain.CDNTrafficUsage
	if err := json.Unmarshal(raw, &usage); err != nil {
		return nil, fmt.Errorf("decoding monthly traffic usage: %w", err)
	}
	return &usage, nil
}

// GetCDNMinTLSVersionInput identifies the zone to look up.
type GetCDNMinTLSVersionInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
}

// GetCDNMinTLSVersion is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type GetCDNMinTLSVersion struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewGetCDNMinTLSVersion builds the use case from its ports.
func NewGetCDNMinTLSVersion(queue ports.Queue, provider ports.ParspackProvider) *GetCDNMinTLSVersion {
	return &GetCDNMinTLSVersion{queue: queue, provider: provider}
}

// Execute returns the minimum TLS version a zone currently accepts.
func (uc *GetCDNMinTLSVersion) Execute(ctx context.Context, in GetCDNMinTLSVersionInput) (domain.CDNMinTLSVersion, error) {
	if err := in.Credentials.Validate(); err != nil {
		return "", err
	}
	if err := validateZoneUUID(in.ZoneUUID); err != nil {
		return "", err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		version, err := uc.provider.GetCDNMinTLSVersion(ctx, in.Credentials, in.ZoneUUID)
		if err != nil {
			return nil, fmt.Errorf("getting min TLS version for zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(version)
	})
	if err != nil {
		return "", err
	}

	var version domain.CDNMinTLSVersion
	if err := json.Unmarshal(raw, &version); err != nil {
		return "", fmt.Errorf("decoding min TLS version: %w", err)
	}
	return version, nil
}

// UpdateCDNMinTLSVersion needs.

// UpdateCDNMinTLSVersionInput is the normalized form of an
// update_cdn_min_tls_version tool call.
type UpdateCDNMinTLSVersionInput struct {
	Credentials   domain.ProviderCredentials
	ZoneUUID      string
	MinTLSVersion domain.CDNMinTLSVersion
}

// UpdateCDNMinTLSVersion is a fast operation: it runs on a worker but the
// caller waits for the result inside the same tool call.
type UpdateCDNMinTLSVersion struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewUpdateCDNMinTLSVersion builds the use case from its ports.
func NewUpdateCDNMinTLSVersion(queue ports.Queue, provider ports.ParspackProvider) *UpdateCDNMinTLSVersion {
	return &UpdateCDNMinTLSVersion{queue: queue, provider: provider}
}

// Execute validates and applies a zone's minimum TLS version, returning the
// applied value synchronously.
func (uc *UpdateCDNMinTLSVersion) Execute(ctx context.Context, in UpdateCDNMinTLSVersionInput) (domain.CDNMinTLSVersion, error) {
	if err := in.Credentials.Validate(); err != nil {
		return "", err
	}
	if err := validateZoneUUID(in.ZoneUUID); err != nil {
		return "", err
	}
	if !domain.ValidCDNMinTLSVersion(in.MinTLSVersion) {
		return "", fmt.Errorf("min_tls_version %q is not one of the versions Parspack accepts: %w", in.MinTLSVersion, domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.UpdateCDNMinTLSVersion(ctx, in.Credentials, in.ZoneUUID, in.MinTLSVersion); err != nil {
			return nil, fmt.Errorf("updating min TLS version for zone %s: %w", in.ZoneUUID, err)
		}
		return json.RawMessage(`{}`), nil
	})
	if err != nil {
		return "", err
	}
	return in.MinTLSVersion, nil
}

// ListCDNCertificatesInput identifies the zone whose attached certificates
// to list, plus optional pagination and a domain substring filter.
type ListCDNCertificatesInput struct {
	Credentials  domain.ProviderCredentials
	ZoneUUID     string
	PerPage      int
	Page         int
	DomainFilter string
}

// ListCDNCertificates is a fast, read-only operation: it lists certificates
// already attached to a zone. Ordering a new certificate is a separate
// workflow (app.CreateSSLOrder and friends, issue #18) not exposed here.
type ListCDNCertificates struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewListCDNCertificates builds the use case from its ports.
func NewListCDNCertificates(queue ports.Queue, provider ports.ParspackProvider) *ListCDNCertificates {
	return &ListCDNCertificates{queue: queue, provider: provider}
}

// Execute returns the certificates currently attached to a zone.
func (uc *ListCDNCertificates) Execute(ctx context.Context, in ListCDNCertificatesInput) ([]domain.CDNCertificate, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if err := validateZoneUUID(in.ZoneUUID); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		certs, err := uc.provider.ListCDNCertificates(ctx, in.Credentials, in.ZoneUUID, in.PerPage, in.Page, in.DomainFilter)
		if err != nil {
			return nil, fmt.Errorf("listing SSL certificates for zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(certs)
	})
	if err != nil {
		return nil, err
	}

	var certs []domain.CDNCertificate
	if err := json.Unmarshal(raw, &certs); err != nil {
		return nil, fmt.Errorf("decoding SSL certificate list: %w", err)
	}
	return certs, nil
}

// GetCDNHSTSInput identifies the zone to look up.
type GetCDNHSTSInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
}

// GetCDNHSTS is a fast operation: it runs on a worker but the caller waits
// for the result inside the same tool call.
type GetCDNHSTS struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewGetCDNHSTS builds the use case from its ports.
func NewGetCDNHSTS(queue ports.Queue, provider ports.ParspackProvider) *GetCDNHSTS {
	return &GetCDNHSTS{queue: queue, provider: provider}
}

// Execute returns a zone's current HSTS configuration.
func (uc *GetCDNHSTS) Execute(ctx context.Context, in GetCDNHSTSInput) (*domain.CDNHSTSSettings, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if err := validateZoneUUID(in.ZoneUUID); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		settings, err := uc.provider.GetCDNHSTS(ctx, in.Credentials, in.ZoneUUID)
		if err != nil {
			return nil, fmt.Errorf("getting HSTS settings for zone %s: %w", in.ZoneUUID, err)
		}
		return json.Marshal(settings)
	})
	if err != nil {
		return nil, err
	}

	var settings domain.CDNHSTSSettings
	if err := json.Unmarshal(raw, &settings); err != nil {
		return nil, fmt.Errorf("decoding HSTS settings: %w", err)
	}
	return &settings, nil
}

// UpdateCDNHSTSInput is the normalized form of an update_cdn_hsts tool call.
type UpdateCDNHSTSInput struct {
	Credentials domain.ProviderCredentials
	ZoneUUID    string
	Settings    domain.CDNHSTSSettings
}

// UpdateCDNHSTS is a fast operation: it runs on a worker but the caller
// waits for the result inside the same tool call.
type UpdateCDNHSTS struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewUpdateCDNHSTS builds the use case from its ports.
func NewUpdateCDNHSTS(queue ports.Queue, provider ports.ParspackProvider) *UpdateCDNHSTS {
	return &UpdateCDNHSTS{queue: queue, provider: provider}
}

// Execute validates and applies a zone's HSTS configuration, returning the
// applied settings synchronously.
func (uc *UpdateCDNHSTS) Execute(ctx context.Context, in UpdateCDNHSTSInput) (*domain.CDNHSTSSettings, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if err := validateZoneUUID(in.ZoneUUID); err != nil {
		return nil, err
	}
	if in.Settings.MaxAgeSeconds < 0 || in.Settings.MaxAgeSeconds > 31536000 {
		return nil, fmt.Errorf("max_age_seconds %d is out of range 0-31536000: %w", in.Settings.MaxAgeSeconds, domain.ErrInvalidInput)
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.UpdateCDNHSTS(ctx, in.Credentials, in.ZoneUUID, in.Settings); err != nil {
			return nil, fmt.Errorf("updating HSTS settings for zone %s: %w", in.ZoneUUID, err)
		}
		return json.RawMessage(`{}`), nil
	})
	if err != nil {
		return nil, err
	}
	return &in.Settings, nil
}
