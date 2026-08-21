package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// This file holds the Active Health Check use cases for ArvanCloud (issue
// #70): a domain-scoped monitor that periodically probes an origin (in
// practice, a Load Balancing pool — arvancloud_loadbalancer.go, issue #69)
// over TCP or HTTP(S) and reports whether it is reachable — see
// domain/arvancloud_healthcheck.go's package comment. Every one of them is a
// fast operation (ports.ArvanCloudProvider, AGENTS.md 4.3): each dispatches
// onto the queue and blocks for the result within the same tool call.
//
// What IS validated client-side, per issue #70's acceptance criteria:
//
//   - Interval against domain.ValidArvanCloudHealthCheckIntervalMS — the
//     fixed 30000/60000/120000-millisecond enum — rejecting a
//     plausible-looking-but-wrong value like 30 (seconds, not milliseconds)
//     or 30000000 (issue #70's own example).
//   - Type against domain.ValidArvanCloudHealthCheckType, OriginType against
//     domain.ValidArvanCloudHealthCheckOriginType.
//   - RequestConfig carries exactly the branch Type selects (TCP -> TCP
//     config, HTTP/HTTPS -> HTTP config), each with its own required fields
//     (validateArvanCloudHealthCheckRequestConfigInput).
//   - Each report query's Name/Upstream (required by both report endpoints)
//     and Period/Direction/Type against their respective
//     domain.ValidArvanCloud... enums.

// --- Health checks ----------------------------------------------------

// arvanCloudHealthCheckDomainInput is embedded by every use case below that
// is scoped to exactly one domain by name and needs nothing else.
type arvanCloudHealthCheckDomainInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
}

func (in arvanCloudHealthCheckDomainInput) validate() error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.Domain == "" {
		return fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

// ListArvanCloudHealthChecksInput identifies the domain whose health checks
// to list.
type ListArvanCloudHealthChecksInput = arvanCloudHealthCheckDomainInput

// ListArvanCloudHealthChecks is a fast operation.
type ListArvanCloudHealthChecks struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudHealthChecks builds the use case from its ports.
func NewListArvanCloudHealthChecks(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudHealthChecks {
	return &ListArvanCloudHealthChecks{queue: queue, provider: provider}
}

// Execute returns every health check configured for the domain.
func (uc *ListArvanCloudHealthChecks) Execute(ctx context.Context, in ListArvanCloudHealthChecksInput) ([]domain.ArvanCloudHealthCheck, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		checks, err := uc.provider.ListArvanCloudHealthChecks(ctx, in.Credentials, in.Domain)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud health checks of domain %q: %w", in.Domain, err)
		}
		return json.Marshal(checks)
	})
	if err != nil {
		return nil, err
	}

	var checks []domain.ArvanCloudHealthCheck
	if err := json.Unmarshal(raw, &checks); err != nil {
		return nil, fmt.Errorf("decoding arvancloud health check list: %w", err)
	}
	return checks, nil
}

// validateArvanCloudHealthCheckRequestConfigInput checks that RequestConfig
// carries exactly the branch checkType selects, with that branch's own
// required fields populated — see this file's package comment.
func validateArvanCloudHealthCheckRequestConfigInput(checkType domain.ArvanCloudHealthCheckType, cfg domain.ArvanCloudHealthCheckRequestConfig) error {
	if checkType == domain.ArvanCloudHealthCheckTCP {
		if cfg.TCP == nil {
			return fmt.Errorf("request_config.tcp is required when type is \"TCP\": %w", domain.ErrInvalidInput)
		}
		if cfg.HTTP != nil {
			return fmt.Errorf("request_config.http must not be set when type is \"TCP\": %w", domain.ErrInvalidInput)
		}
		if cfg.TCP.Port < 1 || cfg.TCP.Port > 65535 {
			return fmt.Errorf("request_config.tcp.port must be between 1 and 65535, got %d: %w", cfg.TCP.Port, domain.ErrInvalidInput)
		}
		if cfg.TCP.TimeoutMS < 1 || cfg.TCP.TimeoutMS > 10000 {
			return fmt.Errorf("request_config.tcp.timeout_ms must be between 1 and 10000, got %d: %w", cfg.TCP.TimeoutMS, domain.ErrInvalidInput)
		}
		return nil
	}

	// HTTP or HTTPS.
	if cfg.HTTP == nil {
		return fmt.Errorf("request_config.http is required when type is %q: %w", checkType, domain.ErrInvalidInput)
	}
	if cfg.TCP != nil {
		return fmt.Errorf("request_config.tcp must not be set when type is %q: %w", checkType, domain.ErrInvalidInput)
	}
	h := cfg.HTTP
	if !domain.ValidArvanCloudHealthCheckHTTPMethod(string(h.Method)) {
		return fmt.Errorf("request_config.http.method %q is not one of HEAD/GET/POST/PUT: %w", h.Method, domain.ErrInvalidInput)
	}
	if h.Port < 1 || h.Port > 65535 {
		return fmt.Errorf("request_config.http.port must be between 1 and 65535, got %d: %w", h.Port, domain.ErrInvalidInput)
	}
	if h.Path == "" {
		return fmt.Errorf("request_config.http.path is required: %w", domain.ErrInvalidInput)
	}
	if h.TimeoutMS < 1 || h.TimeoutMS > 30000 {
		return fmt.Errorf("request_config.http.timeout_ms must be between 1 and 30000, got %d: %w", h.TimeoutMS, domain.ErrInvalidInput)
	}
	return nil
}

// validateArvanCloudHealthCheckInput checks the fields every create/update
// health check call shares, per this file's package comment.
func validateArvanCloudHealthCheckInput(hc domain.ArvanCloudHealthCheck) error {
	if hc.Name == "" {
		return fmt.Errorf("name is required: %w", domain.ErrInvalidInput)
	}
	if hc.Origin == "" {
		return fmt.Errorf("origin is required: %w", domain.ErrInvalidInput)
	}
	if !domain.ValidArvanCloudHealthCheckOriginType(string(hc.OriginType)) {
		return fmt.Errorf("origin_type %q is not one of the values ArvanCloud currently accepts (\"pool\"): %w", hc.OriginType, domain.ErrInvalidInput)
	}
	if !domain.ValidArvanCloudHealthCheckIntervalMS(hc.IntervalMS) {
		return fmt.Errorf(
			"interval_ms %d is not one of 30000, 60000 or 120000 (milliseconds — 30/60/120 seconds, not seconds themselves): %w",
			hc.IntervalMS, domain.ErrInvalidInput)
	}
	if hc.Threshold < 1 {
		return fmt.Errorf("threshold must be at least 1, got %d: %w", hc.Threshold, domain.ErrInvalidInput)
	}
	if hc.Retries < 0 || hc.Retries > 10 {
		return fmt.Errorf("retries must be between 0 and 10, got %d: %w", hc.Retries, domain.ErrInvalidInput)
	}
	if !domain.ValidArvanCloudHealthCheckType(string(hc.Type)) {
		return fmt.Errorf("type %q is not one of TCP/HTTP/HTTPS: %w", hc.Type, domain.ErrInvalidInput)
	}
	for _, zone := range hc.Zones {
		if !domain.ValidArvanCloudHealthCheckZoneMonitoringLevel(zone.MonitoringLevel) {
			return fmt.Errorf(
				"zones[].monitoring_level %q is not one of critical/non-critical/quiet-critical/off: %w",
				zone.MonitoringLevel, domain.ErrInvalidInput)
		}
	}
	return validateArvanCloudHealthCheckRequestConfigInput(hc.Type, hc.RequestConfig)
}

// CreateArvanCloudHealthCheckInput is the normalized form of a
// create_arvancloud_health_check tool call.
type CreateArvanCloudHealthCheckInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	Check       domain.ArvanCloudHealthCheck
}

// CreateArvanCloudHealthCheck creates a new health check.
type CreateArvanCloudHealthCheck struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewCreateArvanCloudHealthCheck builds the use case from its ports.
func NewCreateArvanCloudHealthCheck(queue ports.Queue, provider ports.ArvanCloudProvider) *CreateArvanCloudHealthCheck {
	return &CreateArvanCloudHealthCheck{queue: queue, provider: provider}
}

// Execute validates the request and creates the check, returning it as
// stored.
func (uc *CreateArvanCloudHealthCheck) Execute(ctx context.Context, in CreateArvanCloudHealthCheckInput) (*domain.ArvanCloudHealthCheck, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Domain == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if err := validateArvanCloudHealthCheckInput(in.Check); err != nil {
		return nil, err
	}

	return dispatchArvanCloudHealthCheck(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudHealthCheck, error) {
		created, err := uc.provider.CreateArvanCloudHealthCheck(ctx, in.Credentials, in.Domain, in.Check)
		if err != nil {
			return nil, fmt.Errorf("creating arvancloud health check on domain %q: %w", in.Domain, err)
		}
		return created, nil
	})
}

// arvanCloudHealthCheckIDInput is embedded by every use case below that is
// scoped to exactly one health check by domain + id.
type arvanCloudHealthCheckIDInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	ID          string
}

func (in arvanCloudHealthCheckIDInput) validate() error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.Domain == "" {
		return fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if in.ID == "" {
		return fmt.Errorf("id is required: %w", domain.ErrInvalidInput)
	}
	return nil
}

// GetArvanCloudHealthCheckInput identifies the health check to look up.
type GetArvanCloudHealthCheckInput = arvanCloudHealthCheckIDInput

// GetArvanCloudHealthCheck is a fast operation.
type GetArvanCloudHealthCheck struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudHealthCheck builds the use case from its ports.
func NewGetArvanCloudHealthCheck(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudHealthCheck {
	return &GetArvanCloudHealthCheck{queue: queue, provider: provider}
}

// Execute returns the current state of one health check.
func (uc *GetArvanCloudHealthCheck) Execute(ctx context.Context, in GetArvanCloudHealthCheckInput) (*domain.ArvanCloudHealthCheck, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudHealthCheck(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudHealthCheck, error) {
		found, err := uc.provider.GetArvanCloudHealthCheck(ctx, in.Credentials, in.Domain, in.ID)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud health check %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return found, nil
	})
}

// UpdateArvanCloudHealthCheckInput identifies the health check to update and
// its new field values.
type UpdateArvanCloudHealthCheckInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	ID          string
	Check       domain.ArvanCloudHealthCheck
}

// UpdateArvanCloudHealthCheck changes a health check. This is a fast
// operation.
type UpdateArvanCloudHealthCheck struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUpdateArvanCloudHealthCheck builds the use case from its ports.
func NewUpdateArvanCloudHealthCheck(queue ports.Queue, provider ports.ArvanCloudProvider) *UpdateArvanCloudHealthCheck {
	return &UpdateArvanCloudHealthCheck{queue: queue, provider: provider}
}

// Execute updates the check and returns it as stored afterward.
func (uc *UpdateArvanCloudHealthCheck) Execute(ctx context.Context, in UpdateArvanCloudHealthCheckInput) (*domain.ArvanCloudHealthCheck, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Domain == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if in.ID == "" {
		return nil, fmt.Errorf("id is required: %w", domain.ErrInvalidInput)
	}
	if err := validateArvanCloudHealthCheckInput(in.Check); err != nil {
		return nil, err
	}

	return dispatchArvanCloudHealthCheck(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudHealthCheck, error) {
		updated, err := uc.provider.UpdateArvanCloudHealthCheck(ctx, in.Credentials, in.Domain, in.ID, in.Check)
		if err != nil {
			return nil, fmt.Errorf("updating arvancloud health check %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return updated, nil
	})
}

// DeleteArvanCloudHealthCheckInput identifies the health check to remove.
type DeleteArvanCloudHealthCheckInput = arvanCloudHealthCheckIDInput

// DeleteArvanCloudHealthCheck is a fast operation. Deleting a check the
// provider no longer has is treated as already done rather than an error,
// matching DeleteArvanCloudRateLimitRule's tolerant-delete contract.
type DeleteArvanCloudHealthCheck struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewDeleteArvanCloudHealthCheck builds the use case from its ports.
func NewDeleteArvanCloudHealthCheck(queue ports.Queue, provider ports.ArvanCloudProvider) *DeleteArvanCloudHealthCheck {
	return &DeleteArvanCloudHealthCheck{queue: queue, provider: provider}
}

// Execute deletes the check, tolerating one that is already gone.
func (uc *DeleteArvanCloudHealthCheck) Execute(ctx context.Context, in DeleteArvanCloudHealthCheckInput) error {
	if err := in.validate(); err != nil {
		return err
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteArvanCloudHealthCheck(ctx, in.Credentials, in.Domain, in.ID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting arvancloud health check %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// --- Zones --------------------------------------------------------------

// ListArvanCloudDomainHealthCheckZonesInput identifies the domain whose
// check-execution zones to list.
type ListArvanCloudDomainHealthCheckZonesInput = arvanCloudHealthCheckDomainInput

// ListArvanCloudDomainHealthCheckZones is a fast operation.
type ListArvanCloudDomainHealthCheckZones struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudDomainHealthCheckZones builds the use case from its
// ports.
func NewListArvanCloudDomainHealthCheckZones(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudDomainHealthCheckZones {
	return &ListArvanCloudDomainHealthCheckZones{queue: queue, provider: provider}
}

// Execute returns the check-execution zones available to the domain.
func (uc *ListArvanCloudDomainHealthCheckZones) Execute(ctx context.Context, in ListArvanCloudDomainHealthCheckZonesInput) ([]domain.ArvanCloudHealthCheckZoneName, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		zones, err := uc.provider.ListArvanCloudDomainHealthCheckZones(ctx, in.Credentials, in.Domain)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud health check zones of domain %q: %w", in.Domain, err)
		}
		return json.Marshal(zones)
	})
	if err != nil {
		return nil, err
	}

	var zones []domain.ArvanCloudHealthCheckZoneName
	if err := json.Unmarshal(raw, &zones); err != nil {
		return nil, fmt.Errorf("decoding arvancloud health check zone list: %w", err)
	}
	return zones, nil
}

// ListArvanCloudHealthCheckZonesInput carries only credentials: this
// endpoint is account-independent, mirroring ListArvanCloudLBRegionsInput's
// own shape.
type ListArvanCloudHealthCheckZonesInput struct {
	Credentials domain.ProviderCredentials
}

// ListArvanCloudHealthCheckZones is a fast operation.
type ListArvanCloudHealthCheckZones struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudHealthCheckZones builds the use case from its ports.
func NewListArvanCloudHealthCheckZones(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudHealthCheckZones {
	return &ListArvanCloudHealthCheckZones{queue: queue, provider: provider}
}

// Execute returns the global, account-independent set of check-execution
// zones.
func (uc *ListArvanCloudHealthCheckZones) Execute(ctx context.Context, in ListArvanCloudHealthCheckZonesInput) ([]domain.ArvanCloudHealthCheckZoneName, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		zones, err := uc.provider.ListArvanCloudHealthCheckZones(ctx, in.Credentials)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud global health check zones: %w", err)
		}
		return json.Marshal(zones)
	})
	if err != nil {
		return nil, err
	}

	var zones []domain.ArvanCloudHealthCheckZoneName
	if err := json.Unmarshal(raw, &zones); err != nil {
		return nil, fmt.Errorf("decoding arvancloud health check zone list: %w", err)
	}
	return zones, nil
}

// --- Reports --------------------------------------------------------------

// validateArvanCloudHealthCheckReportQuery checks the fields both report
// use cases share: Name/Upstream are required by both endpoints (the spec
// declares them required query parameters), Period/Direction against their
// enums. includeType additionally validates Type, meaningful only for
// GetArvanCloudHealthCheckDetails.
func validateArvanCloudHealthCheckReportQuery(q domain.ArvanCloudHealthCheckReportQuery, includeType bool) error {
	if q.Name == "" {
		return fmt.Errorf("name is required: %w", domain.ErrInvalidInput)
	}
	if q.Upstream == "" {
		return fmt.Errorf("upstream is required: %w", domain.ErrInvalidInput)
	}
	if !domain.ValidArvanCloudHealthCheckReportPeriod(q.Period) {
		return fmt.Errorf("period %q is not one of 5m/1h/3h/6h/12h/24h/7d/30d: %w", q.Period, domain.ErrInvalidInput)
	}
	if !domain.ValidArvanCloudReportDirection(q.Direction) {
		return fmt.Errorf("direction %q is not \"asc\" or \"desc\": %w", q.Direction, domain.ErrInvalidInput)
	}
	if includeType && !domain.ValidArvanCloudHealthCheckReportType(q.Type) {
		return fmt.Errorf("type %q is not one of all/success/error: %w", q.Type, domain.ErrInvalidInput)
	}
	return nil
}

// GetArvanCloudHealthCheckSummaryInput identifies the domain and the report
// filter for a summary report.
type GetArvanCloudHealthCheckSummaryInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	Query       domain.ArvanCloudHealthCheckReportQuery
}

// GetArvanCloudHealthCheckSummary is a fast operation.
type GetArvanCloudHealthCheckSummary struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudHealthCheckSummary builds the use case from its ports.
func NewGetArvanCloudHealthCheckSummary(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudHealthCheckSummary {
	return &GetArvanCloudHealthCheckSummary{queue: queue, provider: provider}
}

// Execute returns the health check's per-zone summary report.
func (uc *GetArvanCloudHealthCheckSummary) Execute(ctx context.Context, in GetArvanCloudHealthCheckSummaryInput) ([]domain.ArvanCloudHealthCheckReportSummary, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Domain == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if err := validateArvanCloudHealthCheckReportQuery(in.Query, false); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		report, err := uc.provider.GetArvanCloudHealthCheckSummary(ctx, in.Credentials, in.Domain, in.Query)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud health check summary report on domain %q: %w", in.Domain, err)
		}
		return json.Marshal(report)
	})
	if err != nil {
		return nil, err
	}

	var report []domain.ArvanCloudHealthCheckReportSummary
	if err := json.Unmarshal(raw, &report); err != nil {
		return nil, fmt.Errorf("decoding arvancloud health check summary report: %w", err)
	}
	return report, nil
}

// GetArvanCloudHealthCheckDetailsInput identifies the domain and the report
// filter for a details report.
type GetArvanCloudHealthCheckDetailsInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	Query       domain.ArvanCloudHealthCheckReportQuery
}

// GetArvanCloudHealthCheckDetailsResult pairs one page of probe results with
// its pagination info, the return shape GetArvanCloudHealthCheckDetails'
// Execute needs since the queue's Dispatch carries exactly one JSON value.
type GetArvanCloudHealthCheckDetailsResult struct {
	Details []domain.ArvanCloudHealthCheckReportDetail
	Page    domain.ArvanCloudHealthCheckReportPageMeta
}

// GetArvanCloudHealthCheckDetails is a fast operation.
type GetArvanCloudHealthCheckDetails struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudHealthCheckDetails builds the use case from its ports.
func NewGetArvanCloudHealthCheckDetails(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudHealthCheckDetails {
	return &GetArvanCloudHealthCheckDetails{queue: queue, provider: provider}
}

// Execute returns one page of the health check's individual probe results.
func (uc *GetArvanCloudHealthCheckDetails) Execute(ctx context.Context, in GetArvanCloudHealthCheckDetailsInput) (*GetArvanCloudHealthCheckDetailsResult, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Domain == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if err := validateArvanCloudHealthCheckReportQuery(in.Query, true); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		details, page, err := uc.provider.GetArvanCloudHealthCheckDetails(ctx, in.Credentials, in.Domain, in.Query)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud health check details report on domain %q: %w", in.Domain, err)
		}
		return json.Marshal(GetArvanCloudHealthCheckDetailsResult{Details: details, Page: page})
	})
	if err != nil {
		return nil, err
	}

	var result GetArvanCloudHealthCheckDetailsResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud health check details report: %w", err)
	}
	return &result, nil
}

// --- Dispatch helpers -------------------------------------------------

// dispatchArvanCloudHealthCheck runs fn on the queue and decodes its result
// back into a *domain.ArvanCloudHealthCheck, the shape every health check
// use case above but list/delete returns.
func dispatchArvanCloudHealthCheck(
	ctx context.Context, queue ports.Queue, fn func(ctx context.Context) (*domain.ArvanCloudHealthCheck, error),
) (*domain.ArvanCloudHealthCheck, error) {
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

	var result domain.ArvanCloudHealthCheck
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud health check: %w", err)
	}
	return &result, nil
}
