package mcp

import (
	"context"
	"encoding/json"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// ArvanCloud domain onboarding and lifecycle tools (issue #62): domain
// lifecycle, NS Setup (for "full" domains), CNAME Setup (for "partial"
// domains), and the remaining single-domain actions. All fast operations
// (AGENTS.md 4.3): every tool below returns its result within the call, with
// no operation_id to poll afterward.
//
// NS Setup vs. CNAME Setup is the one distinction a calling chatbot most
// needs to get right from natural language, so every tool description below
// that touches either restates it: NS Setup moves a domain's whole DNS to
// ArvanCloud (the registrar's nameservers point at ArvanCloud); CNAME Setup
// leaves the domain's DNS wherever it already is and routes only one
// subdomain's traffic through ArvanCloud via a CNAME record. A request like
// "my domain's DNS is hosted elsewhere, I just want the CDN on a subdomain"
// calls for CNAME Setup, not NS Setup.

// arvanCloudDomainToMap renders a domain.ArvanCloudDomain the way every
// domain-returning tool reports it back to the caller. Fields left empty by
// an endpoint that only echoes part of the resource (e.g. the NS Setup
// endpoints) come back as their zero value, which callers should read as
// "not reported by this call" rather than "cleared".
func arvanCloudDomainToMap(d domain.ArvanCloudDomain) map[string]any {
	return map[string]any{
		"id":           d.ID,
		"domain":       d.Name,
		"plan_level":   int(d.PlanLevel),
		"plan":         d.PlanLevel.String(),
		"type":         d.Type,
		"status":       d.Status,
		"ns_keys":      d.NSKeys,
		"current_ns":   d.CurrentNS,
		"cname_target": d.CnameTarget,
		"custom_cname": d.CustomCname,
		"dns_cloud":    d.DNSCloud,
		"created_at":   d.CreatedAt,
		"updated_at":   d.UpdatedAt,
	}
}

// arvanCloudDomainNameArgs is embedded by every tool below that is scoped to
// exactly one domain by name and needs nothing else.
type arvanCloudDomainNameArgs struct {
	credentialArgs
	Domain string `json:"domain"`
}

func arvanCloudDomainNameProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The domain name at ArvanCloud, e.g. \"example.com\". ArvanCloud addresses a domain by its name, not an internal ID.",
	}
}

func createArvanCloudDomainTool(uc *app.CreateArvanCloudDomain) Tool {
	props := credentialProperties()
	props["domain"] = map[string]any{
		"type":        "string",
		"description": "The domain to onboard onto ArvanCloud's CDN, e.g. \"example.com\" or a subdomain like \"cdn.example.com\".",
	}
	props["domain_type"] = map[string]any{
		"type": "string",
		"enum": []string{domain.ArvanCloudDomainTypeFull, domain.ArvanCloudDomainTypePartial},
		"description": "Onboarding mode. \"full\" (the default) moves the whole domain's DNS to ArvanCloud via NS " +
			"Setup — use this for a domain whose DNS the user wants ArvanCloud to manage entirely. \"partial\" uses " +
			"CNAME Setup instead: only this domain's traffic is routed through ArvanCloud via a CNAME record, while " +
			"the rest of its DNS stays wherever it already is hosted — use this when the user says something like " +
			"\"my domain's DNS is hosted elsewhere, I just want the CDN on a subdomain\".",
	}
	props["plan_level"] = map[string]any{
		"type": "integer",
		"enum": []int{0, 1, 2, 3, 4},
		"description": "CDN plan as an integer: 0 traffic, 1 basic, 2 growth, 3 professional, 4 enterprise. Defaults " +
			"to 0 (traffic) when omitted. A \"partial\" (CNAME Setup) domain_type requires 2 (growth) or higher.",
	}
	props["import_dns_records"] = map[string]any{
		"type": "boolean",
		"description": "When true (the default if this is omitted), ArvanCloud automatically creates A records for " +
			"the root (@) and www, and attempts to detect and add a wildcard (*) record from DNS resolution. Set to " +
			"false to onboard the domain without importing any existing records.",
	}

	return Tool{
		Name: "create_arvancloud_domain",
		Description: "Onboard a new domain onto ArvanCloud's CDN. This is a fast operation: the created domain, " +
			"including its status and (for a \"full\" domain) the nameservers to configure at the registrar, is " +
			"returned within this call. See the domain_type parameter for the full (NS Setup) vs. partial (CNAME " +
			"Setup) distinction.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				credentialArgs
				Domain           string `json:"domain"`
				DomainType       string `json:"domain_type"`
				PlanLevel        int    `json:"plan_level"`
				ImportDNSRecords *bool  `json:"import_dns_records"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			// The tool caller omitting import_dns_records must be read as
			// "true": that is ArvanCloud's own documented default for the
			// field, and this is the one place in the call chain where
			// "omitted" is still distinguishable from "explicitly false" —
			// domain.ArvanCloudDomainSpec below carries a plain bool, always
			// resolved by the time core sees it.
			importDNSRecords := true
			if args.ImportDNSRecords != nil {
				importDNSRecords = *args.ImportDNSRecords
			}

			created, err := uc.Execute(ctx, app.CreateArvanCloudDomainInput{
				Credentials: args.domain(),
				Spec: domain.ArvanCloudDomainSpec{
					Name:             args.Domain,
					DomainType:       args.DomainType,
					PlanLevel:        domain.ArvanCloudPlanLevel(args.PlanLevel),
					ImportDNSRecords: importDNSRecords,
				},
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudDomainToMap(*created), nil
		},
	}
}

func listArvanCloudDomainsTool(uc *app.ListArvanCloudDomains) Tool {
	props := credentialProperties()

	return Tool{
		Name: "list_arvancloud_domains",
		Description: "List every domain onboarded onto ArvanCloud's CDN, visible to the given credentials. This is " +
			"a fast operation: the list is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args credentialArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			domains, err := uc.Execute(ctx, app.ListArvanCloudDomainsInput{Credentials: args.domain()})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(domains))
			for i, d := range domains {
				out[i] = arvanCloudDomainToMap(d)
			}
			return map[string]any{"domains": out}, nil
		},
	}
}

func getArvanCloudDomainTool(uc *app.GetArvanCloudDomain) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "get_arvancloud_domain",
		Description: "Get the current state of one domain onboarded onto ArvanCloud's CDN, by name. This is a fast " +
			"operation: the result is returned within this call.",
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

			found, err := uc.Execute(ctx, app.GetArvanCloudDomainInput{Credentials: args.domain(), DomainName: args.Domain})
			if err != nil {
				return nil, err
			}
			return arvanCloudDomainToMap(*found), nil
		},
	}
}

func deleteArvanCloudDomainTool(uc *app.DeleteArvanCloudDomain) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "delete_arvancloud_domain",
		Description: "Permanently remove a domain from ArvanCloud's CDN by name. This is a fast operation and " +
			"cannot be undone. Deleting a domain that no longer exists is treated as already done rather than an " +
			"error.",
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

			if err := uc.Execute(ctx, app.DeleteArvanCloudDomainInput{Credentials: args.domain(), DomainName: args.Domain}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "domain": args.Domain}, nil
		},
	}
}

func setArvanCloudNSKeysTool(uc *app.SetArvanCloudNSKeys) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["ns_keys"] = map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"minItems":    2,
		"maxItems":    2,
		"description": "The exact pair of nameservers to point the domain's registrar at, e.g. [\"h.ns.arvancloud.ir\", \"s.ns.arvancloud.ir\"].",
	}

	return Tool{
		Name: "set_arvancloud_ns_keys",
		Description: "Set custom NS records for a \"full\" (NS Setup) ArvanCloud domain — the registrar must then " +
			"be pointed at these nameservers. Use this only for a domain onboarded with domain_type \"full\"; a " +
			"\"partial\" (CNAME Setup) domain does not use NS records at all. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "ns_keys"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				NSKeys []string `json:"ns_keys"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.SetArvanCloudNSKeysInput{
				Credentials: args.domain(), DomainName: args.Domain, NSKeys: args.NSKeys,
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudDomainToMap(*updated), nil
		},
	}
}

func resetArvanCloudNSKeysTool(uc *app.ResetArvanCloudNSKeys) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "reset_arvancloud_ns_keys",
		Description: "Reset a \"full\" (NS Setup) ArvanCloud domain's NS records back to ArvanCloud's default " +
			"nameservers. This is a fast operation.",
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

			reset, err := uc.Execute(ctx, app.ResetArvanCloudNSKeysInput{Credentials: args.domain(), DomainName: args.Domain})
			if err != nil {
				return nil, err
			}
			return arvanCloudDomainToMap(*reset), nil
		},
	}
}

func checkArvanCloudNSStatusTool(uc *app.CheckArvanCloudNSStatus) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "check_arvancloud_ns_status",
		Description: "Check whether a \"full\" (NS Setup) ArvanCloud domain's registrar has actually been " +
			"repointed at ArvanCloud yet. Returns both the nameservers ArvanCloud expects (ns_keys) and what it " +
			"currently sees configured at the registrar (current_ns) — compare the two to tell whether the domain " +
			"still needs repointing. This is a fast operation.",
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

			status, err := uc.Execute(ctx, app.CheckArvanCloudNSStatusInput{Credentials: args.domain(), DomainName: args.Domain})
			if err != nil {
				return nil, err
			}
			return arvanCloudDomainToMap(*status), nil
		},
	}
}

func useArvanCloudOptionalNSKeysTool(uc *app.UseArvanCloudOptionalNSKeys) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "use_arvancloud_optional_ns_keys",
		Description: "Switch a \"full\" (NS Setup) ArvanCloud domain to ArvanCloud's alternate NS key set — useful " +
			"when a registrar rejects or blocks the primary set. This is a fast operation.",
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

			updated, err := uc.Execute(ctx, app.UseArvanCloudOptionalNSKeysInput{Credentials: args.domain(), DomainName: args.Domain})
			if err != nil {
				return nil, err
			}
			return arvanCloudDomainToMap(*updated), nil
		},
	}
}

func setArvanCloudCnameTargetTool(uc *app.SetArvanCloudCnameTarget) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["address"] = map[string]any{
		"type":        "string",
		"description": "The CNAME record value to point this domain at, e.g. \"custom.cdn.example.net\".",
	}

	return Tool{
		Name: "set_arvancloud_cname_target",
		Description: "Set a custom CNAME record for a \"partial\" (CNAME Setup) ArvanCloud domain. Use this when " +
			"the domain's DNS is hosted elsewhere and only this domain/subdomain's traffic is routed through " +
			"ArvanCloud via CNAME — not for a \"full\" (NS Setup) domain, which uses NS records instead. This is a " +
			"fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "address"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				Address string `json:"address"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.SetArvanCloudCnameTargetInput{
				Credentials: args.domain(), DomainName: args.Domain, Address: args.Address,
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudDomainToMap(*updated), nil
		},
	}
}

func resetArvanCloudCnameTargetTool(uc *app.ResetArvanCloudCnameTarget) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "reset_arvancloud_cname_target",
		Description: "Reset a \"partial\" (CNAME Setup) ArvanCloud domain's CNAME record back to ArvanCloud's " +
			"default value. This is a fast operation.",
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

			reset, err := uc.Execute(ctx, app.ResetArvanCloudCnameTargetInput{Credentials: args.domain(), DomainName: args.Domain})
			if err != nil {
				return nil, err
			}
			return arvanCloudDomainToMap(*reset), nil
		},
	}
}

func convertArvanCloudToCnameSetupTool(uc *app.ConvertArvanCloudToCnameSetup) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "convert_arvancloud_to_cname_setup",
		Description: "Convert an ArvanCloud domain's onboarding mode to CNAME Setup (\"partial\"). Use this for a " +
			"domain currently on NS Setup (\"full\") when the user wants to move it to leaving its DNS hosted " +
			"elsewhere and routing only this domain/subdomain's traffic through ArvanCloud via CNAME instead — the " +
			"case where the user says something like \"my domain's DNS is hosted elsewhere, I just want the CDN on " +
			"a subdomain\". This is a fast operation.",
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

			converted, err := uc.Execute(ctx, app.ConvertArvanCloudToCnameSetupInput{Credentials: args.domain(), DomainName: args.Domain})
			if err != nil {
				return nil, err
			}
			return arvanCloudDomainToMap(*converted), nil
		},
	}
}

func checkArvanCloudCnameStatusTool(uc *app.CheckArvanCloudCnameStatus) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "check_arvancloud_cname_status",
		Description: "Check whether a \"partial\" (CNAME Setup) ArvanCloud domain's CNAME record has been " +
			"activated yet. This is a fast operation.",
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

			status, err := uc.Execute(ctx, app.CheckArvanCloudCnameStatusInput{Credentials: args.domain(), DomainName: args.Domain})
			if err != nil {
				return nil, err
			}
			return arvanCloudDomainToMap(*status), nil
		},
	}
}

func cloneArvanCloudDomainConfigTool(uc *app.CloneArvanCloudDomainConfig) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["from_domain"] = map[string]any{
		"type":        "string",
		"description": "The already-configured ArvanCloud domain to copy the CDN configuration (cache rules, firewall, ...) FROM, e.g. \"example.com\".",
	}

	return Tool{
		Name: "clone_arvancloud_domain_config",
		Description: "Copy another ArvanCloud domain's CDN configuration onto this one. This is a fast operation " +
			"and returns no data other than confirmation — use get_arvancloud_domain afterward to see the result.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "from_domain"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				FromDomain string `json:"from_domain"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.CloneArvanCloudDomainConfigInput{
				Credentials: args.domain(), DomainName: args.Domain, FromDomain: args.FromDomain,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"cloned": true, "domain": args.Domain, "from_domain": args.FromDomain}, nil
		},
	}
}

func regenerateArvanCloudDomainConfigTool(uc *app.RegenerateArvanCloudDomainConfig) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "regenerate_arvancloud_domain_config",
		Description: "Re-publish an ArvanCloud domain's current CDN configuration to the edge servers. This is a " +
			"fast operation: the call itself returns immediately, though the actual propagation to edge servers " +
			"happens asynchronously afterward on ArvanCloud's own side, with nothing exposed here to poll for it.",
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

			if err := uc.Execute(ctx, app.RegenerateArvanCloudDomainConfigInput{Credentials: args.domain(), DomainName: args.Domain}); err != nil {
				return nil, err
			}
			return map[string]any{"regenerated": true, "domain": args.Domain}, nil
		},
	}
}

func holdArvanCloudDomainTool(uc *app.HoldArvanCloudDomain) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "hold_arvancloud_domain",
		Description: "Pause CDN service for an ArvanCloud domain, taking it offline until unhold_arvancloud_domain " +
			"resumes it. This is a fast operation.",
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

			if err := uc.Execute(ctx, app.HoldArvanCloudDomainInput{Credentials: args.domain(), DomainName: args.Domain}); err != nil {
				return nil, err
			}
			return map[string]any{"held": true, "domain": args.Domain}, nil
		},
	}
}

func unholdArvanCloudDomainTool(uc *app.UnholdArvanCloudDomain) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "unhold_arvancloud_domain",
		Description: "Resume CDN service for an ArvanCloud domain previously paused with hold_arvancloud_domain. " +
			"This is a fast operation.",
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

			if err := uc.Execute(ctx, app.UnholdArvanCloudDomainInput{Credentials: args.domain(), DomainName: args.Domain}); err != nil {
				return nil, err
			}
			return map[string]any{"unheld": true, "domain": args.Domain}, nil
		},
	}
}
