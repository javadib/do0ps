package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// ArvanCloud DNS record, DNSSEC and Secondary DNS tools (issue #63). All
// fast operations (AGENTS.md 4.3): every tool below returns its result
// within the call, with no operation_id to poll afterward.
//
// A DNS record here is addressed by a per-record UUID (the "id" argument),
// unlike Parspack's list_dns_records/create_dns_record/... tools, which
// address a record by host+type — see
// internal/core/domain/arvancloud_dns.go's package comment for why this
// needed a fresh domain type rather than reusing domain.DNSRecord.

// arvanCloudDNSRecordValueArgs is one entry of a create/update tool's
// "values" array. Every field belongs to a specific subset of the 13 record
// types (documented per field in the JSON Schema built by
// arvanCloudDNSRecordValueItemSchema); only the fields relevant to the
// record's own "type" argument are read.
type arvanCloudDNSRecordValueArgs struct {
	IP             string `json:"ip"`
	Port           int    `json:"port"`
	Weight         int    `json:"weight"`
	OriginalWeight int    `json:"original_weight"`
	Country        string `json:"country"`
	Host           string `json:"host"`
	HostHeader     string `json:"host_header"`
	Location       string `json:"location"`
	Priority       int    `json:"priority"`
	Target         string `json:"target"`
	Text           string `json:"text"`
	Domain         string `json:"domain"`
	Usage          string `json:"usage"`
	Selector       string `json:"selector"`
	MatchingType   string `json:"matching_type"`
	Certificate    string `json:"certificate"`
	Value          string `json:"value"`
	Tag            string `json:"tag"`
}

func (a arvanCloudDNSRecordValueArgs) toDomain() domain.ArvanCloudDNSRecordValue {
	return domain.ArvanCloudDNSRecordValue{
		IP: a.IP, Port: a.Port, Weight: a.Weight, OriginalWeight: a.OriginalWeight, Country: a.Country,
		Host: a.Host, HostHeader: a.HostHeader, Location: a.Location,
		Priority: a.Priority, Target: a.Target, Text: a.Text, Domain: a.Domain,
		Usage: a.Usage, Selector: a.Selector, MatchingType: a.MatchingType, Certificate: a.Certificate,
		CAAValue: a.Value, Tag: a.Tag,
	}
}

// arvanCloudIPFilterModeArgs mirrors domain.ArvanCloudDNSRecordIPFilterMode.
type arvanCloudIPFilterModeArgs struct {
	Count     string `json:"count"`
	Order     string `json:"order"`
	GeoFilter string `json:"geo_filter"`
}

// arvanCloudDNSRecordArgs is shared by create_arvancloud_dns_record and
// update_arvancloud_dns_record: both take a domain-scoped record body.
type arvanCloudDNSRecordArgs struct {
	arvanCloudDomainNameArgs
	Name          string                         `json:"name"`
	Type          string                         `json:"type"`
	TTL           int                            `json:"ttl"`
	Cloud         bool                           `json:"cloud"`
	UpstreamHTTPS string                         `json:"upstream_https"`
	IPFilterMode  *arvanCloudIPFilterModeArgs    `json:"ip_filter_mode"`
	Values        []arvanCloudDNSRecordValueArgs `json:"values"`
}

func (a arvanCloudDNSRecordArgs) toDomainRecord() (domain.ArvanCloudDNSRecord, error) {
	t, err := domain.ParseArvanCloudDNSRecordType(a.Type)
	if err != nil {
		return domain.ArvanCloudDNSRecord{}, fmt.Errorf("record type %q is not one of the 13 ArvanCloud accepts: %w", a.Type, err)
	}

	values := make([]domain.ArvanCloudDNSRecordValue, len(a.Values))
	for i, v := range a.Values {
		values[i] = v.toDomain()
	}

	rec := domain.ArvanCloudDNSRecord{
		Name: a.Name, Type: t, TTL: a.TTL, Cloud: a.Cloud, UpstreamHTTPS: a.UpstreamHTTPS, Values: values,
	}
	if a.IPFilterMode != nil {
		rec.IPFilterMode = domain.ArvanCloudDNSRecordIPFilterMode{
			Count: a.IPFilterMode.Count, Order: a.IPFilterMode.Order, GeoFilter: a.IPFilterMode.GeoFilter,
		}
	}
	return rec, nil
}

// arvanCloudDNSRecordToMap renders a domain.ArvanCloudDNSRecord the way every
// record-returning tool reports it back to the caller.
func arvanCloudDNSRecordToMap(rec domain.ArvanCloudDNSRecord) map[string]any {
	values := make([]map[string]any, len(rec.Values))
	for i, v := range rec.Values {
		values[i] = map[string]any{
			"ip": v.IP, "port": v.Port, "weight": v.Weight, "original_weight": v.OriginalWeight, "country": v.Country,
			"host": v.Host, "host_header": v.HostHeader, "location": v.Location,
			"priority": v.Priority, "target": v.Target, "text": v.Text, "domain": v.Domain,
			"usage": v.Usage, "selector": v.Selector, "matching_type": v.MatchingType, "certificate": v.Certificate,
			"value": v.CAAValue, "tag": v.Tag,
		}
	}

	return map[string]any{
		"id":             rec.ID,
		"name":           rec.Name,
		"type":           rec.Type.String(),
		"ttl":            rec.TTL,
		"cloud":          rec.Cloud,
		"upstream_https": rec.UpstreamHTTPS,
		"ip_filter_mode": map[string]any{
			"count": rec.IPFilterMode.Count, "order": rec.IPFilterMode.Order, "geo_filter": rec.IPFilterMode.GeoFilter,
		},
		"is_protected": rec.IsProtected,
		"usage":        rec.Usage,
		"values":       values,
		"created_at":   rec.CreatedAt,
		"updated_at":   rec.UpdatedAt,
	}
}

// arvanCloudDNSRecordTypeEnum lists the 13 record types this port accepts,
// in the lowercase wire form the CDN API itself uses.
var arvanCloudDNSRecordTypeEnum = []string{
	"a", "aaaa", "cname", "aname", "mx", "srv", "txt", "spf", "dkim", "ns", "ptr", "tlsa", "caa",
}

// arvanCloudDNSRecordTTLEnum is the fixed TTL enum every record must use.
var arvanCloudDNSRecordTTLEnum = []int{120, 180, 300, 600, 900, 1800, 3600, 7200, 18000, 43200, 86400, 172800, 432000}

// arvanCloudDNSRecordValueItemSchema is the JSON Schema for one entry of a
// create/update tool's "values" array. Every property is documented with
// which of the 13 record types it applies to, since only a subset is
// meaningful for any given record.
func arvanCloudDNSRecordValueItemSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"ip": map[string]any{
				"type":        "string",
				"description": "IPv4 (for type \"a\") or IPv6 (for type \"aaaa\") address, e.g. \"198.51.100.42\". Required for a/aaaa.",
			},
			"port": map[string]any{
				"type":        "integer",
				"description": "Target port, 1-65535, e.g. 5060. Meaningful for a, aaaa, cname, aname and srv (required for srv).",
			},
			"weight": map[string]any{
				"type":        "integer",
				"description": "Relative weight among multiple values, 0-1000. Meaningful for a, aaaa and srv.",
			},
			"country": map[string]any{
				"type":        "string",
				"description": "ISO 3166 alpha-2 country code for geo-targeting this value, e.g. \"US\". Meaningful for a and aaaa.",
			},
			"host": map[string]any{
				"type":        "string",
				"description": "Target hostname. Required for cname, mx and ns (the field is named \"host\" for all three per the API).",
			},
			"host_header": map[string]any{
				"type":        "string",
				"enum":        []string{domain.ArvanCloudHostHeaderSource, domain.ArvanCloudHostHeaderDest},
				"description": "Host header rewrite mode. Meaningful for cname and aname.",
			},
			"location": map[string]any{
				"type":        "string",
				"description": "Target FQDN. Required for aname (aname's equivalent of cname's \"host\").",
			},
			"priority": map[string]any{
				"type":        "integer",
				"description": "Priority, lower is preferred, 0-9999. Required for mx; optional for srv.",
			},
			"target": map[string]any{
				"type":        "string",
				"description": "Target hostname. Required for srv.",
			},
			"text": map[string]any{
				"type":        "string",
				"description": "Text payload, max 500 characters. Required for txt, spf and dkim.",
			},
			"domain": map[string]any{
				"type":        "string",
				"description": "Pointer target hostname. Optional; meaningful for ptr only.",
			},
			"usage": map[string]any{
				"type":        "string",
				"description": "TLSA certificate usage field. Required for tlsa.",
			},
			"selector": map[string]any{
				"type":        "string",
				"description": "TLSA selector field. Required for tlsa.",
			},
			"matching_type": map[string]any{
				"type":        "string",
				"description": "TLSA matching type field. Required for tlsa.",
			},
			"certificate": map[string]any{
				"type":        "string",
				"description": "TLSA certificate association data. Required for tlsa.",
			},
			"value": map[string]any{
				"type":        "string",
				"description": "A domain string naming the certificate authority allowed to issue for this name. Required for caa.",
			},
			"tag": map[string]any{
				"type":        "string",
				"enum":        []string{domain.ArvanCloudCAATagIssue, domain.ArvanCloudCAATagIssueWild, domain.ArvanCloudCAATagIODEF},
				"description": "CAA authorization tag naming what the value authorizes. Required for caa.",
			},
		},
	}
}

// arvanCloudDNSRecordBodyProperties are the fields shared by
// create_arvancloud_dns_record and update_arvancloud_dns_record.
func arvanCloudDNSRecordBodyProperties(props map[string]any) {
	props["name"] = map[string]any{
		"type":        "string",
		"description": "Record hostname, max 250 chars, e.g. \"@\" for the zone apex, \"www\", or a full subdomain.",
	}
	props["type"] = map[string]any{
		"type":        "string",
		"enum":        arvanCloudDNSRecordTypeEnum,
		"description": "DNS record type. Each type has its own required fields inside \"values\" — see that property.",
	}
	props["ttl"] = map[string]any{
		"type":        "integer",
		"enum":        arvanCloudDNSRecordTTLEnum,
		"description": "Time to live in seconds, e.g. 3600 for one hour. Must be one of ArvanCloud's fixed values — an arbitrary value is rejected.",
	}
	props["cloud"] = map[string]any{
		"type":        "boolean",
		"description": "CDN-proxy toggle: true routes this record's traffic through ArvanCloud's CDN, false answers with the raw value. Defaults to false.",
	}
	props["upstream_https"] = map[string]any{
		"type":        "string",
		"enum":        []string{domain.ArvanCloudUpstreamHTTPSDefault, domain.ArvanCloudUpstreamHTTPSAuto, domain.ArvanCloudUpstreamHTTPSHTTP, domain.ArvanCloudUpstreamHTTPSHTTPS},
		"description": "How the CDN edge connects to the origin over HTTPS. Leave unset to use ArvanCloud's default.",
	}
	props["ip_filter_mode"] = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"count":      map[string]any{"type": "string", "enum": []string{domain.ArvanCloudIPFilterCountSingle, domain.ArvanCloudIPFilterCountMulti}, "description": "Whether to answer with one IP or every matching IP."},
			"order":      map[string]any{"type": "string", "enum": []string{domain.ArvanCloudIPFilterOrderNone, domain.ArvanCloudIPFilterOrderWeighted, domain.ArvanCloudIPFilterOrderRR}, "description": "How multiple candidate IPs are ordered before \"count\" is applied."},
			"geo_filter": map[string]any{"type": "string", "enum": []string{domain.ArvanCloudIPFilterGeoNone, domain.ArvanCloudIPFilterGeoLocation, domain.ArvanCloudIPFilterGeoCountry}, "description": "Whether IP selection is narrowed by the resolver's geographic origin."},
		},
		"description": "Controls multi-value a/aaaa selection and geo-targeting. Leave unset for ArvanCloud's default.",
	}
	props["values"] = map[string]any{
		"type":        "array",
		"items":       arvanCloudDNSRecordValueItemSchema(),
		"description": "The record's value(s). a and aaaa records take one entry per IP (multiple weighted/geo-targeted addresses); every other type takes exactly one entry.",
	}
}

func listArvanCloudDNSRecordsTool(uc *app.ListArvanCloudDNSRecords) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "list_arvancloud_dns_records",
		Description: "List every DNS record of an ArvanCloud domain. This is a fast operation: the list is returned " +
			"within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainNameArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			records, err := uc.Execute(ctx, app.ListArvanCloudDNSRecordsInput{Credentials: args.domain(), DomainName: args.Domain})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(records))
			for i, rec := range records {
				out[i] = arvanCloudDNSRecordToMap(rec)
			}
			return map[string]any{"domain": args.Domain, "records": out}, nil
		},
	}
}

func getArvanCloudDNSRecordTool(uc *app.GetArvanCloudDNSRecord) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = map[string]any{
		"type":        "string",
		"description": "The record's UUID, as returned by list_arvancloud_dns_records or create_arvancloud_dns_record.",
	}

	return Tool{
		Name: "get_arvancloud_dns_record",
		Description: "Get one DNS record of an ArvanCloud domain by its UUID. This is a fast operation: the result " +
			"is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				ID string `json:"id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			found, err := uc.Execute(ctx, app.GetArvanCloudDNSRecordInput{Credentials: args.domain(), DomainName: args.Domain, ID: args.ID})
			if err != nil {
				return nil, err
			}
			return arvanCloudDNSRecordToMap(*found), nil
		},
	}
}

func createArvanCloudDNSRecordTool(uc *app.CreateArvanCloudDNSRecord) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	arvanCloudDNSRecordBodyProperties(props)

	return Tool{
		Name: "create_arvancloud_dns_record",
		Description: "Create a DNS record in an ArvanCloud domain. This is a fast operation: the created record, " +
			"including its provider-assigned id, is returned within this call. See the \"type\" and \"values\" " +
			"parameters for which value fields each of the 13 supported record types requires.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "name", "type", "ttl", "values"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDNSRecordArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}
			rec, err := args.toDomainRecord()
			if err != nil {
				return nil, err
			}

			created, err := uc.Execute(ctx, app.CreateArvanCloudDNSRecordInput{
				Credentials: args.domain(), DomainName: args.Domain, Record: rec,
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudDNSRecordToMap(*created), nil
		},
	}
}

func updateArvanCloudDNSRecordTool(uc *app.UpdateArvanCloudDNSRecord) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = map[string]any{
		"type":        "string",
		"description": "The record's UUID, as returned by list_arvancloud_dns_records or create_arvancloud_dns_record.",
	}
	arvanCloudDNSRecordBodyProperties(props)

	return Tool{
		Name: "update_arvancloud_dns_record",
		Description: "Replace a DNS record's configuration in an ArvanCloud domain, by its UUID. This is a fast " +
			"operation: the updated record is returned within this call. A protected record (is_protected in " +
			"get_arvancloud_dns_record's result) may be refused by the provider.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "id", "name", "type", "ttl", "values"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDNSRecordArgs
				ID string `json:"id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}
			rec, err := args.toDomainRecord()
			if err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.UpdateArvanCloudDNSRecordInput{
				Credentials: args.domain(), DomainName: args.Domain, ID: args.ID, Record: rec,
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudDNSRecordToMap(*updated), nil
		},
	}
}

func deleteArvanCloudDNSRecordTool(uc *app.DeleteArvanCloudDNSRecord) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = map[string]any{
		"type":        "string",
		"description": "The record's UUID, as returned by list_arvancloud_dns_records or create_arvancloud_dns_record.",
	}

	return Tool{
		Name: "delete_arvancloud_dns_record",
		Description: "Permanently remove a DNS record from an ArvanCloud domain by its UUID. This is a fast " +
			"operation and cannot be undone. Deleting a record that no longer exists is treated as already done " +
			"rather than an error. A protected record, or one still referenced elsewhere (e.g. by an issued " +
			"certificate), may be refused by the provider.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				ID string `json:"id"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteArvanCloudDNSRecordInput{Credentials: args.domain(), DomainName: args.Domain, ID: args.ID}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "domain": args.Domain, "id": args.ID}, nil
		},
	}
}

func toggleArvanCloudDNSRecordCloudTool(uc *app.ToggleArvanCloudDNSRecordCloud) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["id"] = map[string]any{
		"type":        "string",
		"description": "The record's UUID, as returned by list_arvancloud_dns_records or create_arvancloud_dns_record.",
	}
	props["cloud"] = map[string]any{
		"type":        "boolean",
		"description": "true routes this record's traffic through ArvanCloud's CDN; false answers with the record's raw value.",
	}

	return Tool{
		Name: "toggle_arvancloud_dns_record_cloud",
		Description: "Toggle the CDN-proxy (\"cloud\") status of one DNS record without changing anything else " +
			"about it. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "id", "cloud"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				ID    string `json:"id"`
				Cloud bool   `json:"cloud"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.ToggleArvanCloudDNSRecordCloudInput{
				Credentials: args.domain(), DomainName: args.Domain, ID: args.ID, Cloud: args.Cloud,
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudDNSRecordToMap(*updated), nil
		},
	}
}

func importArvanCloudDNSRecordsTool(uc *app.ImportArvanCloudDNSRecords) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["zone_file"] = map[string]any{
		"type": "string",
		"description": "The full contents of a BIND zone file (plain text, e.g. lines like \"www IN A 198.51.100.1\") " +
			"to bulk-import as DNS records for this domain.",
	}

	return Tool{
		Name: "import_arvancloud_dns_records",
		Description: "Bulk-create DNS records for an ArvanCloud domain by importing a BIND zone file's contents. " +
			"This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "zone_file"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				ZoneFile string `json:"zone_file"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.ImportArvanCloudDNSRecordsInput{
				Credentials: args.domain(), DomainName: args.Domain, ZoneFile: []byte(args.ZoneFile),
			}); err != nil {
				return nil, err
			}
			return map[string]any{"imported": true, "domain": args.Domain}, nil
		},
	}
}

func exportArvanCloudDNSRecordsTool(uc *app.ExportArvanCloudDNSRecords) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "export_arvancloud_dns_records",
		Description: "Export an ArvanCloud domain's DNS records as a BIND zone file's raw text. This is a fast " +
			"operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainNameArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			content, err := uc.Execute(ctx, app.ExportArvanCloudDNSRecordsInput{Credentials: args.domain(), DomainName: args.Domain})
			if err != nil {
				return nil, err
			}
			return map[string]any{"domain": args.Domain, "zone_file": content}, nil
		},
	}
}

func getArvanCloudDNSSecStatusTool(uc *app.GetArvanCloudDNSSecStatus) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "get_arvancloud_dnssec_status",
		Description: "Get an ArvanCloud domain's current DNSSEC status, including the DS record to publish at the " +
			"registrar when enabled. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainNameArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			status, err := uc.Execute(ctx, app.GetArvanCloudDNSSecStatusInput{Credentials: args.domain(), DomainName: args.Domain})
			if err != nil {
				return nil, err
			}
			return map[string]any{"domain": args.Domain, "enabled": status.Enabled, "ds": status.DS}, nil
		},
	}
}

func updateArvanCloudDNSSecStatusTool(uc *app.UpdateArvanCloudDNSSecStatus) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["enable"] = map[string]any{
		"type":        "boolean",
		"description": "true enables DNSSEC for the domain, false disables it.",
	}
	props["rotate"] = map[string]any{
		"type":        "boolean",
		"description": "true rotates the domain's DNSSEC signing keys. Defaults to false.",
	}

	return Tool{
		Name: "update_arvancloud_dnssec_status",
		Description: "Enable or disable DNSSEC for an ArvanCloud domain, optionally rotating its signing keys. " +
			"This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "enable"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				Enable bool `json:"enable"`
				Rotate bool `json:"rotate"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			status, err := uc.Execute(ctx, app.UpdateArvanCloudDNSSecStatusInput{
				Credentials: args.domain(), DomainName: args.Domain, Enable: args.Enable, Rotate: args.Rotate,
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{"domain": args.Domain, "enabled": status.Enabled, "ds": status.DS}, nil
		},
	}
}

// arvanCloudSecondaryDNSToMap renders a domain.ArvanCloudSecondaryDNSConfig
// the way every Secondary DNS tool reports it back to the caller.
func arvanCloudSecondaryDNSToMap(domainName string, cfg domain.ArvanCloudSecondaryDNSConfig) map[string]any {
	skipped := make([]map[string]any, len(cfg.SkippedRecords))
	for i, s := range cfg.SkippedRecords {
		skipped[i] = map[string]any{"name": s.Name, "type": s.Type, "value": s.Value}
	}
	return map[string]any{
		"domain":          domainName,
		"status":          cfg.Status,
		"nameserver":      cfg.Nameserver,
		"soa_serial":      cfg.SOASerial,
		"error":           cfg.ErrorMessage,
		"skipped_records": skipped,
		"created_at":      cfg.CreatedAt,
		"updated_at":      cfg.UpdatedAt,
	}
}

func getArvanCloudSecondaryDNSTool(uc *app.GetArvanCloudSecondaryDNS) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "get_arvancloud_secondary_dns",
		Description: "Get an ArvanCloud domain's current Secondary DNS configuration — ArvanCloud acting as a " +
			"secondary nameserver, transferring zone data from another primary nameserver. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainNameArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			cfg, err := uc.Execute(ctx, app.GetArvanCloudSecondaryDNSInput{Credentials: args.domain(), DomainName: args.Domain})
			if err != nil {
				return nil, err
			}
			return arvanCloudSecondaryDNSToMap(args.Domain, *cfg), nil
		},
	}
}

func setArvanCloudSecondaryDNSTool(uc *app.SetArvanCloudSecondaryDNS) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["status"] = map[string]any{
		"type":        "boolean",
		"description": "true enables Secondary DNS for the domain, false disables it while keeping the configured nameserver.",
	}
	props["nameserver"] = map[string]any{
		"type":        "string",
		"description": "The primary nameserver ArvanCloud transfers the zone from via AXFR/IXFR, e.g. \"ns1.example.com\".",
	}

	return Tool{
		Name: "set_arvancloud_secondary_dns",
		Description: "Create or replace an ArvanCloud domain's Secondary DNS configuration. This is a fast " +
			"operation: the stored configuration is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "status", "nameserver"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				Status     bool   `json:"status"`
				Nameserver string `json:"nameserver"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			cfg, err := uc.Execute(ctx, app.SetArvanCloudSecondaryDNSInput{
				Credentials: args.domain(), DomainName: args.Domain,
				Config: domain.ArvanCloudSecondaryDNSConfig{Status: args.Status, Nameserver: args.Nameserver},
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudSecondaryDNSToMap(args.Domain, *cfg), nil
		},
	}
}

func removeArvanCloudSecondaryDNSTool(uc *app.RemoveArvanCloudSecondaryDNS) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "remove_arvancloud_secondary_dns",
		Description: "Remove an ArvanCloud domain's Secondary DNS configuration entirely. This is a fast operation " +
			"and cannot be undone. Removing a configuration that no longer exists is treated as already done rather " +
			"than an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudDomainNameArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.RemoveArvanCloudSecondaryDNSInput{Credentials: args.domain(), DomainName: args.Domain}); err != nil {
				return nil, err
			}
			return map[string]any{"removed": true, "domain": args.Domain}, nil
		},
	}
}
