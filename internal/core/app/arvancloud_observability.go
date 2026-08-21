package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// This file holds the Log Forwarders and Metric Exporters use cases for
// ArvanCloud (issue #76): both push data to an external system rather than
// exposing it through this package's own Reports use cases
// (arvancloud_reports.go, issue #75) — see
// domain/arvancloud_observability.go's package comment for the full
// spec-ambiguity resolution notes. Every one of them is a fast operation
// (ports.ArvanCloudProvider, AGENTS.md 4.3): each dispatches onto the queue
// and blocks for the result within the same tool call.
//
// What IS validated client-side:
//
//   - A log forwarder's Type against domain.ValidArvanCloudLogForwarderType,
//     ConnectionType against domain.ValidArvanCloudConnectionType, and
//     Settings carrying exactly the one branch ConnectionType selects, with
//     that branch's own required fields populated
//     (validateArvanCloudLogForwarderSettingsInput).
//   - A metric exporter's Type against
//     domain.ValidArvanCloudMetricExporterType and Interval against
//     domain.ValidArvanCloudMetricExporterInterval.

// --- Log Forwarders --------------------------------------------------------

// ListArvanCloudLogForwardersInput identifies the domain and filter/paging
// query for a log forwarder listing.
type ListArvanCloudLogForwardersInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	Query       domain.ArvanCloudLogForwarderListQuery
}

// ListArvanCloudLogForwardersResult pairs one page of results with its
// pagination info, the return shape ListArvanCloudLogForwarders' Execute
// needs since the queue's Dispatch carries exactly one JSON value — the same
// pattern ListArvanCloudHighRequestIPsResult uses.
type ListArvanCloudLogForwardersResult struct {
	Forwarders []domain.ArvanCloudLogForwarder
	Page       domain.ArvanCloudReportPageMeta
}

// ListArvanCloudLogForwarders is a fast operation.
type ListArvanCloudLogForwarders struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudLogForwarders builds the use case from its ports.
func NewListArvanCloudLogForwarders(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudLogForwarders {
	return &ListArvanCloudLogForwarders{queue: queue, provider: provider}
}

// Execute returns one page of the domain's log forwarders.
func (uc *ListArvanCloudLogForwarders) Execute(ctx context.Context, in ListArvanCloudLogForwardersInput) (*ListArvanCloudLogForwardersResult, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Domain == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	for _, t := range in.Query.Types {
		if !domain.ValidArvanCloudLogForwarderType(t) {
			return nil, fmt.Errorf("types[] value %q is not one of access/waf/dns/error/event: %w", t, domain.ErrInvalidInput)
		}
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		forwarders, page, err := uc.provider.ListArvanCloudLogForwarders(ctx, in.Credentials, in.Domain, in.Query)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud log forwarders of domain %q: %w", in.Domain, err)
		}
		return json.Marshal(ListArvanCloudLogForwardersResult{Forwarders: forwarders, Page: page})
	})
	if err != nil {
		return nil, err
	}

	var result ListArvanCloudLogForwardersResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud log forwarder list: %w", err)
	}
	return &result, nil
}

// validateArvanCloudLogForwarderSettingsInput checks that settings carries
// exactly the one branch connectionType selects, with that branch's own
// required fields populated — the same "exactly one branch, with its own
// required fields" shape validateArvanCloudHealthCheckRequestConfigInput
// checks for request_config.
func validateArvanCloudLogForwarderSettingsInput(connectionType domain.ArvanCloudConnectionType, settings domain.ArvanCloudLogForwarderSettings) error {
	populated := 0
	if settings.S3 != nil {
		populated++
	}
	if settings.Datadog != nil {
		populated++
	}
	if settings.Kafka != nil {
		populated++
	}
	if settings.Loggly != nil {
		populated++
	}
	if settings.Syslog != nil {
		populated++
	}
	if settings.ArvanLogs != nil {
		populated++
	}
	if populated == 0 {
		return fmt.Errorf("settings is required: %w", domain.ErrInvalidInput)
	}
	if populated > 1 {
		return fmt.Errorf("settings must carry exactly one connection type's fields, got %d: %w", populated, domain.ErrInvalidInput)
	}

	switch {
	case domain.IsArvanCloudS3ConnectionType(connectionType):
		if settings.S3 == nil {
			return fmt.Errorf("settings.s3 is required when connection_type is %q: %w", connectionType, domain.ErrInvalidInput)
		}
		s := settings.S3
		if s.S3Endpoint == "" || s.AccessKey == "" || s.SecretKey == "" || s.BucketName == "" {
			return fmt.Errorf("settings.s3_endpoint, access_key, secret_key and bucket_name are all required for connection_type %q: %w", connectionType, domain.ErrInvalidInput)
		}
	case connectionType == domain.ArvanCloudConnectionTypeDatadog:
		if settings.Datadog == nil {
			return fmt.Errorf("settings.datadog is required when connection_type is \"datadog\": %w", domain.ErrInvalidInput)
		}
		if settings.Datadog.URL == "" || settings.Datadog.APIKey == "" {
			return fmt.Errorf("settings.url and api_key are required for connection_type \"datadog\": %w", domain.ErrInvalidInput)
		}
	case connectionType == domain.ArvanCloudConnectionTypeKafka:
		if settings.Kafka == nil {
			return fmt.Errorf("settings.kafka is required when connection_type is \"kafka\": %w", domain.ErrInvalidInput)
		}
		if len(settings.Kafka.KafkaBrokers) == 0 || settings.Kafka.KafkaTopicToWrite == "" {
			return fmt.Errorf("settings.kafka_brokers (non-empty) and kafka_topic_to_write are required for connection_type \"kafka\": %w", domain.ErrInvalidInput)
		}
	case connectionType == domain.ArvanCloudConnectionTypeLoggly:
		if settings.Loggly == nil {
			return fmt.Errorf("settings.loggly is required when connection_type is \"loggly\": %w", domain.ErrInvalidInput)
		}
		if settings.Loggly.Token == "" || settings.Loggly.URL == "" {
			return fmt.Errorf("settings.token and url are required for connection_type \"loggly\": %w", domain.ErrInvalidInput)
		}
	case connectionType == domain.ArvanCloudConnectionTypeSyslog:
		if settings.Syslog == nil {
			return fmt.Errorf("settings.syslog is required when connection_type is \"syslog\": %w", domain.ErrInvalidInput)
		}
		s := settings.Syslog
		if !domain.ValidArvanCloudLogForwarderSyslogType(string(s.LogType)) {
			return fmt.Errorf("settings.logtype %q is not \"syslogudp\" or \"syslogtcp\": %w", s.LogType, domain.ErrInvalidInput)
		}
		if s.Host == "" || s.Port < 1 || s.Port > 65535 {
			return fmt.Errorf("settings.host is required and settings.port must be between 1 and 65535 for connection_type \"syslog\": %w", domain.ErrInvalidInput)
		}
	case connectionType == domain.ArvanCloudConnectionTypeArvanLogs:
		if settings.ArvanLogs == nil {
			return fmt.Errorf("settings.arvan_logs is required when connection_type is \"arvan_logs\": %w", domain.ErrInvalidInput)
		}
	default:
		return fmt.Errorf("connection_type %q is not one of arvan_s3/alibaba_s3/amazon_s3/custom_s3/loggly/datadog/syslog/kafka/arvan_logs: %w", connectionType, domain.ErrInvalidInput)
	}
	return nil
}

// validateArvanCloudLogForwarderInput checks the fields every create/update
// log forwarder call shares, per this file's package comment.
func validateArvanCloudLogForwarderInput(lf domain.ArvanCloudLogForwarder) error {
	if lf.Name == "" {
		return fmt.Errorf("name is required: %w", domain.ErrInvalidInput)
	}
	if lf.Description == "" {
		return fmt.Errorf("description is required: %w", domain.ErrInvalidInput)
	}
	if !domain.ValidArvanCloudLogForwarderType(string(lf.Type)) {
		return fmt.Errorf("type %q is not one of access/waf/dns/error/event: %w", lf.Type, domain.ErrInvalidInput)
	}
	if !domain.ValidArvanCloudConnectionType(string(lf.ConnectionType)) {
		return fmt.Errorf("connection_type %q is not one of arvan_s3/alibaba_s3/amazon_s3/custom_s3/loggly/datadog/syslog/kafka/arvan_logs: %w", lf.ConnectionType, domain.ErrInvalidInput)
	}
	return validateArvanCloudLogForwarderSettingsInput(lf.ConnectionType, lf.Settings)
}

// CreateArvanCloudLogForwarderInput is the normalized form of a
// create_arvancloud_log_forwarder tool call.
type CreateArvanCloudLogForwarderInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	Forwarder   domain.ArvanCloudLogForwarder
}

// CreateArvanCloudLogForwarder creates a new log forwarder. This is a fast
// operation.
type CreateArvanCloudLogForwarder struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewCreateArvanCloudLogForwarder builds the use case from its ports.
func NewCreateArvanCloudLogForwarder(queue ports.Queue, provider ports.ArvanCloudProvider) *CreateArvanCloudLogForwarder {
	return &CreateArvanCloudLogForwarder{queue: queue, provider: provider}
}

// Execute validates the request and creates the forwarder, returning it as
// stored.
func (uc *CreateArvanCloudLogForwarder) Execute(ctx context.Context, in CreateArvanCloudLogForwarderInput) (*domain.ArvanCloudLogForwarder, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Domain == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if err := validateArvanCloudLogForwarderInput(in.Forwarder); err != nil {
		return nil, err
	}

	return dispatchArvanCloudLogForwarder(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudLogForwarder, error) {
		created, err := uc.provider.CreateArvanCloudLogForwarder(ctx, in.Credentials, in.Domain, in.Forwarder)
		if err != nil {
			return nil, fmt.Errorf("creating arvancloud log forwarder on domain %q: %w", in.Domain, err)
		}
		return created, nil
	})
}

// arvanCloudLogForwarderIDInput is embedded by every use case below that is
// scoped to exactly one log forwarder by domain + id.
type arvanCloudLogForwarderIDInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	ID          string
}

func (in arvanCloudLogForwarderIDInput) validate() error {
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

// GetArvanCloudLogForwarderInput identifies the log forwarder to look up.
type GetArvanCloudLogForwarderInput = arvanCloudLogForwarderIDInput

// GetArvanCloudLogForwarder is a fast operation.
type GetArvanCloudLogForwarder struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudLogForwarder builds the use case from its ports.
func NewGetArvanCloudLogForwarder(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudLogForwarder {
	return &GetArvanCloudLogForwarder{queue: queue, provider: provider}
}

// Execute returns the current state of one log forwarder.
func (uc *GetArvanCloudLogForwarder) Execute(ctx context.Context, in GetArvanCloudLogForwarderInput) (*domain.ArvanCloudLogForwarder, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudLogForwarder(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudLogForwarder, error) {
		found, err := uc.provider.GetArvanCloudLogForwarder(ctx, in.Credentials, in.Domain, in.ID)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud log forwarder %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return found, nil
	})
}

// UpdateArvanCloudLogForwarderInput identifies the log forwarder to update
// and its new field values.
type UpdateArvanCloudLogForwarderInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	ID          string
	Forwarder   domain.ArvanCloudLogForwarder
}

// UpdateArvanCloudLogForwarder changes a log forwarder. This is a fast
// operation.
type UpdateArvanCloudLogForwarder struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUpdateArvanCloudLogForwarder builds the use case from its ports.
func NewUpdateArvanCloudLogForwarder(queue ports.Queue, provider ports.ArvanCloudProvider) *UpdateArvanCloudLogForwarder {
	return &UpdateArvanCloudLogForwarder{queue: queue, provider: provider}
}

// Execute updates the forwarder and returns it as stored afterward.
func (uc *UpdateArvanCloudLogForwarder) Execute(ctx context.Context, in UpdateArvanCloudLogForwarderInput) (*domain.ArvanCloudLogForwarder, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Domain == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if in.ID == "" {
		return nil, fmt.Errorf("id is required: %w", domain.ErrInvalidInput)
	}
	if err := validateArvanCloudLogForwarderInput(in.Forwarder); err != nil {
		return nil, err
	}

	return dispatchArvanCloudLogForwarder(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudLogForwarder, error) {
		updated, err := uc.provider.UpdateArvanCloudLogForwarder(ctx, in.Credentials, in.Domain, in.ID, in.Forwarder)
		if err != nil {
			return nil, fmt.Errorf("updating arvancloud log forwarder %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return updated, nil
	})
}

// DeleteArvanCloudLogForwarderInput identifies the log forwarder to remove.
type DeleteArvanCloudLogForwarderInput = arvanCloudLogForwarderIDInput

// DeleteArvanCloudLogForwarder is a fast operation. Deleting a forwarder the
// provider no longer has is treated as already done rather than an error,
// matching DeleteArvanCloudHealthCheck's tolerant-delete contract.
type DeleteArvanCloudLogForwarder struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewDeleteArvanCloudLogForwarder builds the use case from its ports.
func NewDeleteArvanCloudLogForwarder(queue ports.Queue, provider ports.ArvanCloudProvider) *DeleteArvanCloudLogForwarder {
	return &DeleteArvanCloudLogForwarder{queue: queue, provider: provider}
}

// Execute deletes the forwarder, tolerating one that is already gone.
func (uc *DeleteArvanCloudLogForwarder) Execute(ctx context.Context, in DeleteArvanCloudLogForwarderInput) error {
	if err := in.validate(); err != nil {
		return err
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteArvanCloudLogForwarder(ctx, in.Credentials, in.Domain, in.ID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting arvancloud log forwarder %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// SetArvanCloudLogForwarderStatusInput identifies the log forwarder and the
// new status to set.
type SetArvanCloudLogForwarderStatusInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	ID          string
	Status      bool
}

// SetArvanCloudLogForwarderStatus enables or disables a log forwarder. This
// is a fast operation.
type SetArvanCloudLogForwarderStatus struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewSetArvanCloudLogForwarderStatus builds the use case from its ports.
func NewSetArvanCloudLogForwarderStatus(queue ports.Queue, provider ports.ArvanCloudProvider) *SetArvanCloudLogForwarderStatus {
	return &SetArvanCloudLogForwarderStatus{queue: queue, provider: provider}
}

// Execute toggles the forwarder's status and returns it as stored
// afterward.
func (uc *SetArvanCloudLogForwarderStatus) Execute(ctx context.Context, in SetArvanCloudLogForwarderStatusInput) (*domain.ArvanCloudLogForwarder, error) {
	if err := (arvanCloudLogForwarderIDInput{Credentials: in.Credentials, Domain: in.Domain, ID: in.ID}).validate(); err != nil {
		return nil, err
	}

	return dispatchArvanCloudLogForwarder(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudLogForwarder, error) {
		updated, err := uc.provider.SetArvanCloudLogForwarderStatus(ctx, in.Credentials, in.Domain, in.ID, in.Status)
		if err != nil {
			return nil, fmt.Errorf("setting arvancloud log forwarder %q status on domain %q: %w", in.ID, in.Domain, err)
		}
		return updated, nil
	})
}

// dispatchArvanCloudLogForwarder runs fn on the queue and decodes its result
// back into a *domain.ArvanCloudLogForwarder, the shape every log forwarder
// use case above but list/delete returns — the same helper pattern
// dispatchArvanCloudHealthCheck provides for health checks.
func dispatchArvanCloudLogForwarder(
	ctx context.Context, queue ports.Queue, fn func(ctx context.Context) (*domain.ArvanCloudLogForwarder, error),
) (*domain.ArvanCloudLogForwarder, error) {
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

	var result domain.ArvanCloudLogForwarder
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud log forwarder: %w", err)
	}
	return &result, nil
}

// --- Metric Exporters -------------------------------------------------------

// ListArvanCloudMetricExportersInput identifies the filter/paging query for
// an account-wide metric exporter listing — NOT scoped to a single domain,
// per domain/arvancloud_observability.go's package comment.
type ListArvanCloudMetricExportersInput struct {
	Credentials domain.ProviderCredentials
	Query       domain.ArvanCloudMetricExporterListQuery
}

// ListArvanCloudMetricExportersResult pairs one page of results with its
// pagination info, the same "queue carries exactly one JSON value" reason
// ListArvanCloudLogForwardersResult exists for.
type ListArvanCloudMetricExportersResult struct {
	Exporters []domain.ArvanCloudMetricExporter
	Page      domain.ArvanCloudReportPageMeta
}

// ListArvanCloudMetricExporters is a fast operation.
type ListArvanCloudMetricExporters struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudMetricExporters builds the use case from its ports.
func NewListArvanCloudMetricExporters(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudMetricExporters {
	return &ListArvanCloudMetricExporters{queue: queue, provider: provider}
}

// Execute returns one page of metric exporters across the whole account.
func (uc *ListArvanCloudMetricExporters) Execute(ctx context.Context, in ListArvanCloudMetricExportersInput) (*ListArvanCloudMetricExportersResult, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	for _, t := range in.Query.Types {
		if !domain.ValidArvanCloudMetricExporterType(t) {
			return nil, fmt.Errorf("types[] value %q is not one of access/dns/error/event: %w", t, domain.ErrInvalidInput)
		}
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		exporters, page, err := uc.provider.ListArvanCloudMetricExporters(ctx, in.Credentials, in.Query)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud metric exporters: %w", err)
		}
		return json.Marshal(ListArvanCloudMetricExportersResult{Exporters: exporters, Page: page})
	})
	if err != nil {
		return nil, err
	}

	var result ListArvanCloudMetricExportersResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud metric exporter list: %w", err)
	}
	return &result, nil
}

// ListArvanCloudMetricExporterTypesInput carries only credentials: this
// endpoint is account-independent, mirroring
// ListArvanCloudHealthCheckZonesInput's own shape.
type ListArvanCloudMetricExporterTypesInput struct {
	Credentials domain.ProviderCredentials
}

// ListArvanCloudMetricExporterTypes is a fast operation.
type ListArvanCloudMetricExporterTypes struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudMetricExporterTypes builds the use case from its ports.
func NewListArvanCloudMetricExporterTypes(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudMetricExporterTypes {
	return &ListArvanCloudMetricExporterTypes{queue: queue, provider: provider}
}

// Execute returns the catalog of metric groups and individual metrics
// available when creating a metric exporter.
func (uc *ListArvanCloudMetricExporterTypes) Execute(ctx context.Context, in ListArvanCloudMetricExporterTypesInput) (*domain.ArvanCloudMetricExporterMetrics, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		metrics, err := uc.provider.ListArvanCloudMetricExporterTypes(ctx, in.Credentials)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud metric exporter types: %w", err)
		}
		return json.Marshal(metrics)
	})
	if err != nil {
		return nil, err
	}

	var metrics domain.ArvanCloudMetricExporterMetrics
	if err := json.Unmarshal(raw, &metrics); err != nil {
		return nil, fmt.Errorf("decoding arvancloud metric exporter types: %w", err)
	}
	return &metrics, nil
}

// validateArvanCloudMetricExporterInput checks the fields every
// create/update metric exporter call shares.
func validateArvanCloudMetricExporterInput(me domain.ArvanCloudMetricExporter) error {
	if me.Name == "" {
		return fmt.Errorf("name is required: %w", domain.ErrInvalidInput)
	}
	if !domain.ValidArvanCloudMetricExporterType(string(me.Type)) {
		return fmt.Errorf("type %q is not one of access/dns/error/event: %w", me.Type, domain.ErrInvalidInput)
	}
	if !domain.ValidArvanCloudMetricExporterInterval(string(me.Interval)) {
		return fmt.Errorf("interval %q is not one of 10s/30s/60s: %w", me.Interval, domain.ErrInvalidInput)
	}
	return nil
}

// CreateArvanCloudMetricExporterInput is the normalized form of a
// create_arvancloud_metric_exporter tool call.
type CreateArvanCloudMetricExporterInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	Exporter    domain.ArvanCloudMetricExporter
}

// CreateArvanCloudMetricExporter creates a new metric exporter, scoped to a
// domain even though listing is account-wide. This is a fast operation.
type CreateArvanCloudMetricExporter struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewCreateArvanCloudMetricExporter builds the use case from its ports.
func NewCreateArvanCloudMetricExporter(queue ports.Queue, provider ports.ArvanCloudProvider) *CreateArvanCloudMetricExporter {
	return &CreateArvanCloudMetricExporter{queue: queue, provider: provider}
}

// Execute validates the request and creates the exporter, returning it as
// stored.
func (uc *CreateArvanCloudMetricExporter) Execute(ctx context.Context, in CreateArvanCloudMetricExporterInput) (*domain.ArvanCloudMetricExporter, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Domain == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if err := validateArvanCloudMetricExporterInput(in.Exporter); err != nil {
		return nil, err
	}

	return dispatchArvanCloudMetricExporter(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudMetricExporter, error) {
		created, err := uc.provider.CreateArvanCloudMetricExporter(ctx, in.Credentials, in.Domain, in.Exporter)
		if err != nil {
			return nil, fmt.Errorf("creating arvancloud metric exporter on domain %q: %w", in.Domain, err)
		}
		return created, nil
	})
}

// arvanCloudMetricExporterIDInput is embedded by every use case below that
// is scoped to exactly one metric exporter by domain + id.
type arvanCloudMetricExporterIDInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	ID          string
}

func (in arvanCloudMetricExporterIDInput) validate() error {
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

// GetArvanCloudMetricExporterInput identifies the metric exporter to look
// up.
type GetArvanCloudMetricExporterInput = arvanCloudMetricExporterIDInput

// GetArvanCloudMetricExporter is a fast operation.
type GetArvanCloudMetricExporter struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetArvanCloudMetricExporter builds the use case from its ports.
func NewGetArvanCloudMetricExporter(queue ports.Queue, provider ports.ArvanCloudProvider) *GetArvanCloudMetricExporter {
	return &GetArvanCloudMetricExporter{queue: queue, provider: provider}
}

// Execute returns the current state of one metric exporter.
func (uc *GetArvanCloudMetricExporter) Execute(ctx context.Context, in GetArvanCloudMetricExporterInput) (*domain.ArvanCloudMetricExporter, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	return dispatchArvanCloudMetricExporter(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudMetricExporter, error) {
		found, err := uc.provider.GetArvanCloudMetricExporter(ctx, in.Credentials, in.Domain, in.ID)
		if err != nil {
			return nil, fmt.Errorf("getting arvancloud metric exporter %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return found, nil
	})
}

// UpdateArvanCloudMetricExporterInput identifies the metric exporter to
// update and its new field values.
type UpdateArvanCloudMetricExporterInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	ID          string
	Exporter    domain.ArvanCloudMetricExporter
}

// UpdateArvanCloudMetricExporter changes a metric exporter. This is a fast
// operation.
type UpdateArvanCloudMetricExporter struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewUpdateArvanCloudMetricExporter builds the use case from its ports.
func NewUpdateArvanCloudMetricExporter(queue ports.Queue, provider ports.ArvanCloudProvider) *UpdateArvanCloudMetricExporter {
	return &UpdateArvanCloudMetricExporter{queue: queue, provider: provider}
}

// Execute updates the exporter and returns it as stored afterward.
func (uc *UpdateArvanCloudMetricExporter) Execute(ctx context.Context, in UpdateArvanCloudMetricExporterInput) (*domain.ArvanCloudMetricExporter, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.Domain == "" {
		return nil, fmt.Errorf("domain is required: %w", domain.ErrInvalidInput)
	}
	if in.ID == "" {
		return nil, fmt.Errorf("id is required: %w", domain.ErrInvalidInput)
	}
	if err := validateArvanCloudMetricExporterInput(in.Exporter); err != nil {
		return nil, err
	}

	return dispatchArvanCloudMetricExporter(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudMetricExporter, error) {
		updated, err := uc.provider.UpdateArvanCloudMetricExporter(ctx, in.Credentials, in.Domain, in.ID, in.Exporter)
		if err != nil {
			return nil, fmt.Errorf("updating arvancloud metric exporter %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return updated, nil
	})
}

// DeleteArvanCloudMetricExporterInput identifies the metric exporter to
// remove.
type DeleteArvanCloudMetricExporterInput = arvanCloudMetricExporterIDInput

// DeleteArvanCloudMetricExporter is a fast operation. Deleting an exporter
// the provider no longer has is treated as already done rather than an
// error, matching DeleteArvanCloudLogForwarder's tolerant-delete contract.
type DeleteArvanCloudMetricExporter struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewDeleteArvanCloudMetricExporter builds the use case from its ports.
func NewDeleteArvanCloudMetricExporter(queue ports.Queue, provider ports.ArvanCloudProvider) *DeleteArvanCloudMetricExporter {
	return &DeleteArvanCloudMetricExporter{queue: queue, provider: provider}
}

// Execute deletes the exporter, tolerating one that is already gone.
func (uc *DeleteArvanCloudMetricExporter) Execute(ctx context.Context, in DeleteArvanCloudMetricExporterInput) error {
	if err := in.validate(); err != nil {
		return err
	}

	_, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		if err := uc.provider.DeleteArvanCloudMetricExporter(ctx, in.Credentials, in.Domain, in.ID); err != nil {
			if isNotFound(err) {
				return json.RawMessage(`{}`), nil
			}
			return nil, fmt.Errorf("deleting arvancloud metric exporter %q on domain %q: %w", in.ID, in.Domain, err)
		}
		return json.RawMessage(`{}`), nil
	})
	return err
}

// SetArvanCloudMetricExporterStatusInput identifies the metric exporter and
// the new status to set.
type SetArvanCloudMetricExporterStatusInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
	ID          string
	Status      bool
}

// SetArvanCloudMetricExporterStatus enables or disables a metric exporter.
// This is a fast operation.
type SetArvanCloudMetricExporterStatus struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewSetArvanCloudMetricExporterStatus builds the use case from its ports.
func NewSetArvanCloudMetricExporterStatus(queue ports.Queue, provider ports.ArvanCloudProvider) *SetArvanCloudMetricExporterStatus {
	return &SetArvanCloudMetricExporterStatus{queue: queue, provider: provider}
}

// Execute toggles the exporter's status and returns it as stored
// afterward.
func (uc *SetArvanCloudMetricExporterStatus) Execute(ctx context.Context, in SetArvanCloudMetricExporterStatusInput) (*domain.ArvanCloudMetricExporter, error) {
	if err := (arvanCloudMetricExporterIDInput{Credentials: in.Credentials, Domain: in.Domain, ID: in.ID}).validate(); err != nil {
		return nil, err
	}

	return dispatchArvanCloudMetricExporter(ctx, uc.queue, func(ctx context.Context) (*domain.ArvanCloudMetricExporter, error) {
		updated, err := uc.provider.SetArvanCloudMetricExporterStatus(ctx, in.Credentials, in.Domain, in.ID, in.Status)
		if err != nil {
			return nil, fmt.Errorf("setting arvancloud metric exporter %q status on domain %q: %w", in.ID, in.Domain, err)
		}
		return updated, nil
	})
}

// dispatchArvanCloudMetricExporter runs fn on the queue and decodes its
// result back into a *domain.ArvanCloudMetricExporter, the same helper
// pattern dispatchArvanCloudLogForwarder provides for log forwarders.
func dispatchArvanCloudMetricExporter(
	ctx context.Context, queue ports.Queue, fn func(ctx context.Context) (*domain.ArvanCloudMetricExporter, error),
) (*domain.ArvanCloudMetricExporter, error) {
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

	var result domain.ArvanCloudMetricExporter
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud metric exporter: %w", err)
	}
	return &result, nil
}
