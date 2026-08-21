package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// This file holds the Reports (per-domain) and Aggregated Reports
// (account-wide) use cases for ArvanCloud (issue #75): pure GET
// traffic/security/DNS analytics — see domain/arvancloud_reports.go's
// package comment for the shared response building blocks these use cases
// return. Every one of them is a fast operation (ports.ArvanCloudProvider,
// AGENTS.md 4.3): each dispatches onto the queue and blocks for the result
// within the same tool call. None of these endpoints create, update or
// delete anything, so — unlike most of this package's other files — there is
// no "not assumed idempotent" caveat anywhere in this file.

// validateArvanCloudReportPeriod checks period against the shared eight-value
// enum both domain.ArvanCloudReportQuery.Period and
// domain.ArvanCloudAggregatedReportQuery.Period use — reusing
// ValidArvanCloudHealthCheckReportPeriod rather than redeclaring the same
// enum a third time (see that function's own doc comment).
func validateArvanCloudReportPeriod(period string) error {
	if !domain.ValidArvanCloudHealthCheckReportPeriod(period) {
		return fmt.Errorf("period %q is not one of 5m/1h/3h/6h/12h/24h/7d/30d: %w", period, domain.ErrInvalidInput)
	}
	return nil
}

// --- Per-domain Reports --------------------------------------------------

// arvanCloudReportInput is embedded/aliased by every per-domain report use
// case below that needs only credentials, a domain and a query.
type arvanCloudReportInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	Query       domain.ArvanCloudReportQuery
}

func (in arvanCloudReportInput) validate() error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.Domain == "" {
		return fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	return validateArvanCloudReportPeriod(in.Query.Period)
}

// GetArvanCloudTrafficReportInput identifies the domain and query for a
// traffic report.
type GetArvanCloudTrafficReportInput = arvanCloudReportInput

// GetArvanCloudTrafficReport is a fast operation.
type GetArvanCloudTrafficReport struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudTrafficReport builds the use case from its ports.
func NewGetArvanCloudTrafficReport(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudTrafficReport {
	return &GetArvanCloudTrafficReport{queue: queue, provider: provider}
}

// Execute returns the domain's traffic report.
func (uc *GetArvanCloudTrafficReport) Execute(ctx context.Context, in GetArvanCloudTrafficReportInput) (*domain.ArvanCloudTrafficReport, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		report, err := uc.provider.GetArvanCloudTrafficReport(ctx, in.Credentials, in.Domain, in.Query)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud traffic report for domain %q: %w", in.Domain, err)
		}
		return json.Marshal(report)
	})
	if err != nil {
		return nil, err
	}
	var report domain.ArvanCloudTrafficReport
	if err := json.Unmarshal(raw, &report); err != nil {
		return nil, fmt.Errorf("decoding arvancloud traffic report: %w", err)
	}
	return &report, nil
}

// GetArvanCloudTrafficSavedReportInput identifies the domain and query for a
// traffic-saved report.
type GetArvanCloudTrafficSavedReportInput = arvanCloudReportInput

// GetArvanCloudTrafficSavedReport is a fast operation.
type GetArvanCloudTrafficSavedReport struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudTrafficSavedReport builds the use case from its ports.
func NewGetArvanCloudTrafficSavedReport(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudTrafficSavedReport {
	return &GetArvanCloudTrafficSavedReport{queue: queue, provider: provider}
}

// Execute returns the domain's bandwidth-saved pie chart report.
func (uc *GetArvanCloudTrafficSavedReport) Execute(ctx context.Context, in GetArvanCloudTrafficSavedReportInput) (*domain.ArvanCloudTrafficSavedReport, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		report, err := uc.provider.GetArvanCloudTrafficSavedReport(ctx, in.Credentials, in.Domain, in.Query)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud traffic saved report for domain %q: %w", in.Domain, err)
		}
		return json.Marshal(report)
	})
	if err != nil {
		return nil, err
	}
	var report domain.ArvanCloudTrafficSavedReport
	if err := json.Unmarshal(raw, &report); err != nil {
		return nil, fmt.Errorf("decoding arvancloud traffic saved report: %w", err)
	}
	return &report, nil
}

// GetArvanCloudTrafficMapInput identifies the domain and query for a
// traffic geo-map report.
type GetArvanCloudTrafficMapInput = arvanCloudReportInput

// GetArvanCloudTrafficMap is a fast operation.
type GetArvanCloudTrafficMap struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudTrafficMap builds the use case from its ports.
func NewGetArvanCloudTrafficMap(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudTrafficMap {
	return &GetArvanCloudTrafficMap{queue: queue, provider: provider}
}

// Execute returns the domain's traffic as a geo-map.
func (uc *GetArvanCloudTrafficMap) Execute(ctx context.Context, in GetArvanCloudTrafficMapInput) (*domain.ArvanCloudTrafficMapReport, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		report, err := uc.provider.GetArvanCloudTrafficMap(ctx, in.Credentials, in.Domain, in.Query)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud traffic map for domain %q: %w", in.Domain, err)
		}
		return json.Marshal(report)
	})
	if err != nil {
		return nil, err
	}
	var report domain.ArvanCloudTrafficMapReport
	if err := json.Unmarshal(raw, &report); err != nil {
		return nil, fmt.Errorf("decoding arvancloud traffic map: %w", err)
	}
	return &report, nil
}

// GetArvanCloudVisitorsReportInput identifies the domain and query for a
// visitors report.
type GetArvanCloudVisitorsReportInput = arvanCloudReportInput

// GetArvanCloudVisitorsReport is a fast operation.
type GetArvanCloudVisitorsReport struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudVisitorsReport builds the use case from its ports.
func NewGetArvanCloudVisitorsReport(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudVisitorsReport {
	return &GetArvanCloudVisitorsReport{queue: queue, provider: provider}
}

// Execute returns the domain's visitor report.
func (uc *GetArvanCloudVisitorsReport) Execute(ctx context.Context, in GetArvanCloudVisitorsReportInput) (*domain.ArvanCloudVisitorsReport, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		report, err := uc.provider.GetArvanCloudVisitorsReport(ctx, in.Credentials, in.Domain, in.Query)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud visitors report for domain %q: %w", in.Domain, err)
		}
		return json.Marshal(report)
	})
	if err != nil {
		return nil, err
	}
	var report domain.ArvanCloudVisitorsReport
	if err := json.Unmarshal(raw, &report); err != nil {
		return nil, fmt.Errorf("decoding arvancloud visitors report: %w", err)
	}
	return &report, nil
}

// ListArvanCloudHighRequestIPsInput identifies the domain and query for a
// high-request-ips report.
type ListArvanCloudHighRequestIPsInput = arvanCloudReportInput

// ListArvanCloudHighRequestIPsResult pairs one page of results with its
// pagination info, the return shape ListArvanCloudHighRequestIPs' Execute
// needs since the queue's Dispatch carries exactly one JSON value — the same
// pattern GetArvanCloudHealthCheckDetailsResult uses.
type ListArvanCloudHighRequestIPsResult struct {
	IPs  []domain.ArvanCloudHighRequestIP
	Page domain.ArvanCloudReportPageMeta
}

// ListArvanCloudHighRequestIPs is a fast operation.
type ListArvanCloudHighRequestIPs struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudHighRequestIPs builds the use case from its ports.
func NewListArvanCloudHighRequestIPs(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudHighRequestIPs {
	return &ListArvanCloudHighRequestIPs{queue: queue, provider: provider}
}

// Execute returns one page of the domain's top requesting IPs.
func (uc *ListArvanCloudHighRequestIPs) Execute(ctx context.Context, in ListArvanCloudHighRequestIPsInput) (*ListArvanCloudHighRequestIPsResult, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		ips, page, err := uc.provider.ListArvanCloudHighRequestIPs(ctx, in.Credentials, in.Domain, in.Query)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud high-request ips for domain %q: %w", in.Domain, err)
		}
		return json.Marshal(ListArvanCloudHighRequestIPsResult{IPs: ips, Page: page})
	})
	if err != nil {
		return nil, err
	}
	var result ListArvanCloudHighRequestIPsResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud high-request ip list: %w", err)
	}
	return &result, nil
}

// GetArvanCloudResponseTimeReportInput identifies the domain and query for a
// response-time report.
type GetArvanCloudResponseTimeReportInput = arvanCloudReportInput

// GetArvanCloudResponseTimeReport is a fast operation.
type GetArvanCloudResponseTimeReport struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudResponseTimeReport builds the use case from its ports.
func NewGetArvanCloudResponseTimeReport(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudResponseTimeReport {
	return &GetArvanCloudResponseTimeReport{queue: queue, provider: provider}
}

// Execute returns the domain's response-time report.
func (uc *GetArvanCloudResponseTimeReport) Execute(ctx context.Context, in GetArvanCloudResponseTimeReportInput) (*domain.ArvanCloudResponseTimeReport, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		report, err := uc.provider.GetArvanCloudResponseTimeReport(ctx, in.Credentials, in.Domain, in.Query)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud response time report for domain %q: %w", in.Domain, err)
		}
		return json.Marshal(report)
	})
	if err != nil {
		return nil, err
	}
	var report domain.ArvanCloudResponseTimeReport
	if err := json.Unmarshal(raw, &report); err != nil {
		return nil, fmt.Errorf("decoding arvancloud response time report: %w", err)
	}
	return &report, nil
}

// GetArvanCloudStatusCodeReportInput identifies the domain and query for a
// status-code report.
type GetArvanCloudStatusCodeReportInput = arvanCloudReportInput

// GetArvanCloudStatusCodeReport is a fast operation.
type GetArvanCloudStatusCodeReport struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudStatusCodeReport builds the use case from its ports.
func NewGetArvanCloudStatusCodeReport(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudStatusCodeReport {
	return &GetArvanCloudStatusCodeReport{queue: queue, provider: provider}
}

// Execute returns the domain's status-code report.
func (uc *GetArvanCloudStatusCodeReport) Execute(ctx context.Context, in GetArvanCloudStatusCodeReportInput) (*domain.ArvanCloudStatusCodeReport, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		report, err := uc.provider.GetArvanCloudStatusCodeReport(ctx, in.Credentials, in.Domain, in.Query)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud status code report for domain %q: %w", in.Domain, err)
		}
		return json.Marshal(report)
	})
	if err != nil {
		return nil, err
	}
	var report domain.ArvanCloudStatusCodeReport
	if err := json.Unmarshal(raw, &report); err != nil {
		return nil, fmt.Errorf("decoding arvancloud status code report: %w", err)
	}
	return &report, nil
}

// GetArvanCloudStatusCodeSummaryInput identifies the domain and query for a
// status-code summary.
type GetArvanCloudStatusCodeSummaryInput = arvanCloudReportInput

// GetArvanCloudStatusCodeSummary is a fast operation.
type GetArvanCloudStatusCodeSummary struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudStatusCodeSummary builds the use case from its ports.
func NewGetArvanCloudStatusCodeSummary(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudStatusCodeSummary {
	return &GetArvanCloudStatusCodeSummary{queue: queue, provider: provider}
}

// Execute returns an overview of the domain's status-code report.
func (uc *GetArvanCloudStatusCodeSummary) Execute(ctx context.Context, in GetArvanCloudStatusCodeSummaryInput) (*domain.ArvanCloudStatusCodeSummary, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		summary, err := uc.provider.GetArvanCloudStatusCodeSummary(ctx, in.Credentials, in.Domain, in.Query)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud status code summary for domain %q: %w", in.Domain, err)
		}
		return json.Marshal(summary)
	})
	if err != nil {
		return nil, err
	}
	var summary domain.ArvanCloudStatusCodeSummary
	if err := json.Unmarshal(raw, &summary); err != nil {
		return nil, fmt.Errorf("decoding arvancloud status code summary: %w", err)
	}
	return &summary, nil
}

// ListArvanCloudErrorLogsInput identifies the domain and query for a list of
// errors.
type ListArvanCloudErrorLogsInput = arvanCloudReportInput

// ListArvanCloudErrorLogs is a fast operation.
type ListArvanCloudErrorLogs struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudErrorLogs builds the use case from its ports.
func NewListArvanCloudErrorLogs(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudErrorLogs {
	return &ListArvanCloudErrorLogs{queue: queue, provider: provider}
}

// Execute returns the domain's list of errors.
func (uc *ListArvanCloudErrorLogs) Execute(ctx context.Context, in ListArvanCloudErrorLogsInput) ([]domain.ArvanCloudErrorLog, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		logs, err := uc.provider.ListArvanCloudErrorLogs(ctx, in.Credentials, in.Domain, in.Query)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud error logs for domain %q: %w", in.Domain, err)
		}
		return json.Marshal(logs)
	})
	if err != nil {
		return nil, err
	}
	var logs []domain.ArvanCloudErrorLog
	if err := json.Unmarshal(raw, &logs); err != nil {
		return nil, fmt.Errorf("decoding arvancloud error log list: %w", err)
	}
	return logs, nil
}

// GetArvanCloudErrorLogsChartInput identifies the domain and query for a
// chart view of errors.
type GetArvanCloudErrorLogsChartInput = arvanCloudReportInput

// GetArvanCloudErrorLogsChart is a fast operation.
type GetArvanCloudErrorLogsChart struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudErrorLogsChart builds the use case from its ports.
func NewGetArvanCloudErrorLogsChart(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudErrorLogsChart {
	return &GetArvanCloudErrorLogsChart{queue: queue, provider: provider}
}

// Execute returns a chart view of the domain's errors.
func (uc *GetArvanCloudErrorLogsChart) Execute(ctx context.Context, in GetArvanCloudErrorLogsChartInput) (*domain.ArvanCloudErrorLogsChart, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		chart, err := uc.provider.GetArvanCloudErrorLogsChart(ctx, in.Credentials, in.Domain, in.Query)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud error logs chart for domain %q: %w", in.Domain, err)
		}
		return json.Marshal(chart)
	})
	if err != nil {
		return nil, err
	}
	var chart domain.ArvanCloudErrorLogsChart
	if err := json.Unmarshal(raw, &chart); err != nil {
		return nil, fmt.Errorf("decoding arvancloud error logs chart: %w", err)
	}
	return &chart, nil
}

// GetArvanCloudErrorLogDetailInput identifies the domain and query
// (including the error message to search for) for one error's detail.
type GetArvanCloudErrorLogDetailInput = arvanCloudReportInput

// GetArvanCloudErrorLogDetail is a fast operation. Deprecated in the spec
// (reports.error-log-details) but still implemented — see
// ports.ArvanCloudProvider.GetArvanCloudErrorLogDetail's own doc comment.
type GetArvanCloudErrorLogDetail struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudErrorLogDetail builds the use case from its ports.
func NewGetArvanCloudErrorLogDetail(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudErrorLogDetail {
	return &GetArvanCloudErrorLogDetail{queue: queue, provider: provider}
}

// Execute returns the detail of one error.
func (uc *GetArvanCloudErrorLogDetail) Execute(ctx context.Context, in GetArvanCloudErrorLogDetailInput) (*domain.ArvanCloudErrorLogDetail, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		detail, err := uc.provider.GetArvanCloudErrorLogDetail(ctx, in.Credentials, in.Domain, in.Query)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud error log detail for domain %q: %w", in.Domain, err)
		}
		return json.Marshal(detail)
	})
	if err != nil {
		return nil, err
	}
	var detail domain.ArvanCloudErrorLogDetail
	if err := json.Unmarshal(raw, &detail); err != nil {
		return nil, fmt.Errorf("decoding arvancloud error log detail: %w", err)
	}
	return &detail, nil
}

// GetArvanCloudDnsRequestsReportInput identifies the domain and query for a
// DNS request report.
type GetArvanCloudDnsRequestsReportInput = arvanCloudReportInput

// GetArvanCloudDnsRequestsReport is a fast operation.
type GetArvanCloudDnsRequestsReport struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudDnsRequestsReport builds the use case from its ports.
func NewGetArvanCloudDnsRequestsReport(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudDnsRequestsReport {
	return &GetArvanCloudDnsRequestsReport{queue: queue, provider: provider}
}

// Execute returns the domain's DNS request report.
func (uc *GetArvanCloudDnsRequestsReport) Execute(ctx context.Context, in GetArvanCloudDnsRequestsReportInput) (*domain.ArvanCloudDnsRequestsReport, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		report, err := uc.provider.GetArvanCloudDnsRequestsReport(ctx, in.Credentials, in.Domain, in.Query)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud dns requests report for domain %q: %w", in.Domain, err)
		}
		return json.Marshal(report)
	})
	if err != nil {
		return nil, err
	}
	var report domain.ArvanCloudDnsRequestsReport
	if err := json.Unmarshal(raw, &report); err != nil {
		return nil, fmt.Errorf("decoding arvancloud dns requests report: %w", err)
	}
	return &report, nil
}

// GetArvanCloudDnsGeoReportInput identifies the domain and query for a DNS
// geo-map report.
type GetArvanCloudDnsGeoReportInput = arvanCloudReportInput

// GetArvanCloudDnsGeoReport is a fast operation.
type GetArvanCloudDnsGeoReport struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudDnsGeoReport builds the use case from its ports.
func NewGetArvanCloudDnsGeoReport(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudDnsGeoReport {
	return &GetArvanCloudDnsGeoReport{queue: queue, provider: provider}
}

// Execute returns DNS requests as a geo-map for the domain.
func (uc *GetArvanCloudDnsGeoReport) Execute(ctx context.Context, in GetArvanCloudDnsGeoReportInput) (*domain.ArvanCloudDnsGeoReport, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		report, err := uc.provider.GetArvanCloudDnsGeoReport(ctx, in.Credentials, in.Domain, in.Query)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud dns geo report for domain %q: %w", in.Domain, err)
		}
		return json.Marshal(report)
	})
	if err != nil {
		return nil, err
	}
	var report domain.ArvanCloudDnsGeoReport
	if err := json.Unmarshal(raw, &report); err != nil {
		return nil, fmt.Errorf("decoding arvancloud dns geo report: %w", err)
	}
	return &report, nil
}

// GetArvanCloudAttackReportInput identifies the domain and query for an
// attack report.
type GetArvanCloudAttackReportInput = arvanCloudReportInput

// GetArvanCloudAttackReport is a fast operation.
type GetArvanCloudAttackReport struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudAttackReport builds the use case from its ports.
func NewGetArvanCloudAttackReport(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudAttackReport {
	return &GetArvanCloudAttackReport{queue: queue, provider: provider}
}

// Execute returns the domain's attack report.
func (uc *GetArvanCloudAttackReport) Execute(ctx context.Context, in GetArvanCloudAttackReportInput) (*domain.ArvanCloudAttackReport, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		report, err := uc.provider.GetArvanCloudAttackReport(ctx, in.Credentials, in.Domain, in.Query)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud attack report for domain %q: %w", in.Domain, err)
		}
		return json.Marshal(report)
	})
	if err != nil {
		return nil, err
	}
	var report domain.ArvanCloudAttackReport
	if err := json.Unmarshal(raw, &report); err != nil {
		return nil, fmt.Errorf("decoding arvancloud attack report: %w", err)
	}
	return &report, nil
}

// ListArvanCloudAttacksInput identifies the domain and query for a list of
// attack details.
type ListArvanCloudAttacksInput = arvanCloudReportInput

// ListArvanCloudAttacksResult pairs one page of results with its pagination
// info, the same "queue carries exactly one JSON value" reason
// ListArvanCloudHighRequestIPsResult exists for.
type ListArvanCloudAttacksResult struct {
	Attacks []domain.ArvanCloudAttackReportItem
	Page    domain.ArvanCloudReportPageMeta
}

// ListArvanCloudAttacks is a fast operation.
type ListArvanCloudAttacks struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudAttacks builds the use case from its ports.
func NewListArvanCloudAttacks(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudAttacks {
	return &ListArvanCloudAttacks{queue: queue, provider: provider}
}

// Execute returns one page of the domain's attack details.
func (uc *ListArvanCloudAttacks) Execute(ctx context.Context, in ListArvanCloudAttacksInput) (*ListArvanCloudAttacksResult, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		attacks, page, err := uc.provider.ListArvanCloudAttacks(ctx, in.Credentials, in.Domain, in.Query)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud attacks for domain %q: %w", in.Domain, err)
		}
		return json.Marshal(ListArvanCloudAttacksResult{Attacks: attacks, Page: page})
	})
	if err != nil {
		return nil, err
	}
	var result ListArvanCloudAttacksResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud attack list: %w", err)
	}
	return &result, nil
}

// ListArvanCloudAttackersInput identifies the domain and query for attacker
// info.
type ListArvanCloudAttackersInput = arvanCloudReportInput

// ListArvanCloudAttackers is a fast operation.
type ListArvanCloudAttackers struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudAttackers builds the use case from its ports.
func NewListArvanCloudAttackers(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudAttackers {
	return &ListArvanCloudAttackers{queue: queue, provider: provider}
}

// Execute returns the domain's attacker info.
func (uc *ListArvanCloudAttackers) Execute(ctx context.Context, in ListArvanCloudAttackersInput) ([]domain.ArvanCloudAttacker, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		attackers, err := uc.provider.ListArvanCloudAttackers(ctx, in.Credentials, in.Domain, in.Query)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud attackers for domain %q: %w", in.Domain, err)
		}
		return json.Marshal(attackers)
	})
	if err != nil {
		return nil, err
	}
	var attackers []domain.ArvanCloudAttacker
	if err := json.Unmarshal(raw, &attackers); err != nil {
		return nil, fmt.Errorf("decoding arvancloud attacker list: %w", err)
	}
	return attackers, nil
}

// GetArvanCloudAttackMapInput identifies the domain and query for an attack
// geo-map report.
type GetArvanCloudAttackMapInput = arvanCloudReportInput

// GetArvanCloudAttackMap is a fast operation.
type GetArvanCloudAttackMap struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudAttackMap builds the use case from its ports.
func NewGetArvanCloudAttackMap(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudAttackMap {
	return &GetArvanCloudAttackMap{queue: queue, provider: provider}
}

// Execute returns a geo-map of attacks for the domain.
func (uc *GetArvanCloudAttackMap) Execute(ctx context.Context, in GetArvanCloudAttackMapInput) (*domain.ArvanCloudAttackMapReport, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		report, err := uc.provider.GetArvanCloudAttackMap(ctx, in.Credentials, in.Domain, in.Query)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud attack map for domain %q: %w", in.Domain, err)
		}
		return json.Marshal(report)
	})
	if err != nil {
		return nil, err
	}
	var report domain.ArvanCloudAttackMapReport
	if err := json.Unmarshal(raw, &report); err != nil {
		return nil, fmt.Errorf("decoding arvancloud attack map: %w", err)
	}
	return &report, nil
}

// ListArvanCloudAttackedURIsInput identifies the domain and query for the
// list of URLs under attack.
type ListArvanCloudAttackedURIsInput = arvanCloudReportInput

// ListArvanCloudAttackedURIs is a fast operation.
type ListArvanCloudAttackedURIs struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudAttackedURIs builds the use case from its ports.
func NewListArvanCloudAttackedURIs(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudAttackedURIs {
	return &ListArvanCloudAttackedURIs{queue: queue, provider: provider}
}

// Execute returns the list of URLs under attack for the domain.
func (uc *ListArvanCloudAttackedURIs) Execute(ctx context.Context, in ListArvanCloudAttackedURIsInput) ([]domain.ArvanCloudAttackedURI, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		uris, err := uc.provider.ListArvanCloudAttackedURIs(ctx, in.Credentials, in.Domain, in.Query)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud attacked uris for domain %q: %w", in.Domain, err)
		}
		return json.Marshal(uris)
	})
	if err != nil {
		return nil, err
	}
	var uris []domain.ArvanCloudAttackedURI
	if err := json.Unmarshal(raw, &uris); err != nil {
		return nil, fmt.Errorf("decoding arvancloud attacked uri list: %w", err)
	}
	return uris, nil
}

// GetArvanCloudTransportLayerProxyTrafficInput identifies the domain, the
// Transport Layer Proxy and the query for its traffic report.
// TransportLayerProxyID is a caller-supplied opaque ID — see
// ports.ArvanCloudProvider's own doc comment for why this port does not
// resolve or validate it against any list of known proxies.
type GetArvanCloudTransportLayerProxyTrafficInput struct {
	Credentials           domain.ProviderCredentials
	Domain                string
	TransportLayerProxyID string
	Query                 domain.ArvanCloudReportQuery
}

func (in GetArvanCloudTransportLayerProxyTrafficInput) validate() error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	if in.Domain == "" {
		return fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if in.TransportLayerProxyID == "" {
		return fmt.Errorf("transport_layer_proxy_id is required: %w", domain.ErrInvalidInput)
	}
	return validateArvanCloudReportPeriod(in.Query.Period)
}

// GetArvanCloudTransportLayerProxyTraffic is a fast operation.
type GetArvanCloudTransportLayerProxyTraffic struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudTransportLayerProxyTraffic builds the use case from its
// ports.
func NewGetArvanCloudTransportLayerProxyTraffic(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudTransportLayerProxyTraffic {
	return &GetArvanCloudTransportLayerProxyTraffic{queue: queue, provider: provider}
}

// Execute returns traffic for one Transport Layer Proxy.
func (uc *GetArvanCloudTransportLayerProxyTraffic) Execute(ctx context.Context, in GetArvanCloudTransportLayerProxyTrafficInput) (*domain.ArvanCloudTransportLayerProxyTraffic, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		traffic, err := uc.provider.GetArvanCloudTransportLayerProxyTraffic(ctx, in.Credentials, in.Domain, in.TransportLayerProxyID, in.Query)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud transport layer proxy %q traffic for domain %q: %w", in.TransportLayerProxyID, in.Domain, err)
		}
		return json.Marshal(traffic)
	})
	if err != nil {
		return nil, err
	}
	var traffic domain.ArvanCloudTransportLayerProxyTraffic
	if err := json.Unmarshal(raw, &traffic); err != nil {
		return nil, fmt.Errorf("decoding arvancloud transport layer proxy traffic: %w", err)
	}
	return &traffic, nil
}

// DownloadArvanCloudDomainsReportInput carries only credentials: this
// endpoint is NOT scoped to a single domain and takes no other parameters
// (domains.reports.download) — see
// ports.ArvanCloudProvider.DownloadArvanCloudDomainsReport's own doc
// comment.
type DownloadArvanCloudDomainsReportInput struct {
	Credentials domain.ProviderCredentials
}

// DownloadArvanCloudDomainsReport is a fast operation.
type DownloadArvanCloudDomainsReport struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewDownloadArvanCloudDomainsReport builds the use case from its ports.
func NewDownloadArvanCloudDomainsReport(queue ports.Queue, provider ports.ArvanCloudProvider) *DownloadArvanCloudDomainsReport {
	return &DownloadArvanCloudDomainsReport{queue: queue, provider: provider}
}

// Execute returns a CSV export of the domains report.
func (uc *DownloadArvanCloudDomainsReport) Execute(ctx context.Context, in DownloadArvanCloudDomainsReportInput) (string, error) {
	if err := in.Credentials.Validate(); err != nil {
		return "", err
	}
	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		csv, err := uc.provider.DownloadArvanCloudDomainsReport(ctx, in.Credentials)
		if err != nil {
			return nil, fmt.Errorf("downloading arvancloud domains report: %w", err)
		}
		return json.Marshal(csv)
	})
	if err != nil {
		return "", err
	}
	var csv string
	if err := json.Unmarshal(raw, &csv); err != nil {
		return "", fmt.Errorf("decoding arvancloud domains report: %w", err)
	}
	return csv, nil
}

// --- Aggregated Reports (account-wide) --------------------------------------

// arvanCloudAggregatedReportInput is aliased by every Aggregated Reports use
// case below.
type arvanCloudAggregatedReportInput struct {
	Credentials domain.ProviderCredentials
	Query       domain.ArvanCloudAggregatedReportQuery
}

func (in arvanCloudAggregatedReportInput) validate() error {
	if err := in.Credentials.Validate(); err != nil {
		return err
	}
	return validateArvanCloudReportPeriod(in.Query.Period)
}

// ListArvanCloudAggregatedReportDetailsInput identifies the query for a page
// of aggregated report details.
type ListArvanCloudAggregatedReportDetailsInput = arvanCloudAggregatedReportInput

// ListArvanCloudAggregatedReportDetailsResult pairs one page of results with
// its pagination info, the same "queue carries exactly one JSON value"
// reason ListArvanCloudHighRequestIPsResult exists for.
type ListArvanCloudAggregatedReportDetailsResult struct {
	Details []domain.ArvanCloudAggregatedReportDetail
	Page    domain.ArvanCloudReportPageMeta
}

// ListArvanCloudAggregatedReportDetails is a fast operation.
type ListArvanCloudAggregatedReportDetails struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudAggregatedReportDetails builds the use case from its
// ports.
func NewListArvanCloudAggregatedReportDetails(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudAggregatedReportDetails {
	return &ListArvanCloudAggregatedReportDetails{queue: queue, provider: provider}
}

// Execute returns one page of aggregated reports across domains.
func (uc *ListArvanCloudAggregatedReportDetails) Execute(ctx context.Context, in ListArvanCloudAggregatedReportDetailsInput) (*ListArvanCloudAggregatedReportDetailsResult, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		details, page, err := uc.provider.ListArvanCloudAggregatedReportDetails(ctx, in.Credentials, in.Query)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud aggregated report details: %w", err)
		}
		return json.Marshal(ListArvanCloudAggregatedReportDetailsResult{Details: details, Page: page})
	})
	if err != nil {
		return nil, err
	}
	var result ListArvanCloudAggregatedReportDetailsResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud aggregated report details: %w", err)
	}
	return &result, nil
}

// GetArvanCloudAggregatedReportChartsInput identifies the query for
// aggregated report charts.
type GetArvanCloudAggregatedReportChartsInput = arvanCloudAggregatedReportInput

// GetArvanCloudAggregatedReportCharts is a fast operation.
type GetArvanCloudAggregatedReportCharts struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudAggregatedReportCharts builds the use case from its ports.
func NewGetArvanCloudAggregatedReportCharts(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudAggregatedReportCharts {
	return &GetArvanCloudAggregatedReportCharts{queue: queue, provider: provider}
}

// Execute returns charts of aggregated reports across domains.
func (uc *GetArvanCloudAggregatedReportCharts) Execute(ctx context.Context, in GetArvanCloudAggregatedReportChartsInput) (*domain.ArvanCloudReportChart, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		chart, err := uc.provider.GetArvanCloudAggregatedReportCharts(ctx, in.Credentials, in.Query)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud aggregated report charts: %w", err)
		}
		return json.Marshal(chart)
	})
	if err != nil {
		return nil, err
	}
	var chart domain.ArvanCloudReportChart
	if err := json.Unmarshal(raw, &chart); err != nil {
		return nil, fmt.Errorf("decoding arvancloud aggregated report charts: %w", err)
	}
	return &chart, nil
}

// GetArvanCloudAggregatedReportFiltersInput identifies the query for the
// available aggregated-report filter dimensions.
type GetArvanCloudAggregatedReportFiltersInput = arvanCloudAggregatedReportInput

// GetArvanCloudAggregatedReportFilters is a fast operation.
type GetArvanCloudAggregatedReportFilters struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudAggregatedReportFilters builds the use case from its
// ports.
func NewGetArvanCloudAggregatedReportFilters(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudAggregatedReportFilters {
	return &GetArvanCloudAggregatedReportFilters{queue: queue, provider: provider}
}

// Execute returns the filter dimensions available for aggregated reports.
func (uc *GetArvanCloudAggregatedReportFilters) Execute(ctx context.Context, in GetArvanCloudAggregatedReportFiltersInput) (*domain.ArvanCloudAggregatedReportFilters, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		filters, err := uc.provider.GetArvanCloudAggregatedReportFilters(ctx, in.Credentials, in.Query)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud aggregated report filters: %w", err)
		}
		return json.Marshal(filters)
	})
	if err != nil {
		return nil, err
	}
	var filters domain.ArvanCloudAggregatedReportFilters
	if err := json.Unmarshal(raw, &filters); err != nil {
		return nil, fmt.Errorf("decoding arvancloud aggregated report filters: %w", err)
	}
	return &filters, nil
}
