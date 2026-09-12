package mcp

import (
	"context"
	"encoding/json"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// ArvanCloud Troubleshoot tools (issue #79/AC19): a domain-scoped built-in
// diagnostic tool that checks common configuration issues (DNS records,
// HTTPS redirection, certificate status, domain expiration, etc.) and
// reports which passed ("safe") and which found problems ("troubled").
// All fast operations (AGENTS.md 4.3).

// arvanCloudTroubleshootDetailToMap renders one
// domain.ArvanCloudTroubleshootDetail.
func arvanCloudTroubleshootDetailToMap(d domain.ArvanCloudTroubleshootDetail) map[string]any {
	return map[string]any{
		"id":      string(d.ID),
		"status":  string(d.Status),
		"details": d.Details,
	}
}

// arvanCloudTroubleshootToMap renders one domain.ArvanCloudTroubleshoot.
func arvanCloudTroubleshootToMap(t domain.ArvanCloudTroubleshoot) map[string]any {
	details := make([]map[string]any, len(t.Details))
	for i, d := range t.Details {
		details[i] = arvanCloudTroubleshootDetailToMap(d)
	}
	return map[string]any{
		"id":         t.ID,
		"details":    details,
		"created_at": t.CreatedAt,
	}
}

// listArvanCloudTroubleshootsTool lists past troubleshoot runs for a domain.
func listArvanCloudTroubleshootsTool(uc *app.ListArvanCloudTroubleshoots) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "list_arvancloud_troubleshoots",
		Description: "List past troubleshoot (diagnostic) runs for a domain on ArvanCloud CDN. Each run contains a set of predefined checks " +
			"(DNS records, HTTPS redirection, active certificate, domain expiration, etc.) with pass/fail results. " +
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
			list, err := uc.Execute(ctx, app.ListArvanCloudTroubleshootsInput{
				Credentials: args.domain(),
				Domain:      args.Domain,
			})
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, len(list))
			for i, t := range list {
				out[i] = arvanCloudTroubleshootToMap(t)
			}
			return out, nil
		},
	}
}

// runArvanCloudTroubleshootTool starts a new troubleshoot run and returns the
// completed result synchronously.
func runArvanCloudTroubleshootTool(uc *app.RunArvanCloudTroubleshoot) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "run_arvancloud_troubleshoot",
		Description: "Start a new troubleshoot (diagnostic) run for a domain on ArvanCloud CDN. This performs a set of predefined checks " +
			"(DNS records, HTTPS redirection, active certificate, domain expiration, origin SSL port, etc.) " +
			"and returns the completed result synchronously — no polling required. " +
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
			result, err := uc.Execute(ctx, app.RunArvanCloudTroubleshootInput{
				Credentials: args.domain(),
				Domain:      args.Domain,
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudTroubleshootToMap(*result), nil
		},
	}
}

// getLatestArvanCloudTroubleshootTool returns the most recent troubleshoot run.
func getLatestArvanCloudTroubleshootTool(uc *app.GetLatestArvanCloudTroubleshoot) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "get_latest_arvancloud_troubleshoot",
		Description: "Get the most recent troubleshoot (diagnostic) run for a domain on ArvanCloud CDN. Returns the latest set of predefined " +
			"checks with pass/fail results. This is a fast operation.",
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
			result, err := uc.Execute(ctx, app.GetLatestArvanCloudTroubleshootInput{
				Credentials: args.domain(),
				Domain:      args.Domain,
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudTroubleshootToMap(*result), nil
		},
	}
}
