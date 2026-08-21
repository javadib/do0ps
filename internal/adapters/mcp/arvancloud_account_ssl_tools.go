package mcp

import (
	"context"
	"encoding/json"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// ArvanCloud account-level Certum certificate ordering tools (issue
// #74/AC14): a paid, CA-issued certificate product purchased against the
// account and installed onto one or more domains. All fast operations
// except issue_arvancloud_account_certificate, the one long operation
// (AGENTS.md 4.3) — it returns an operation_id, poll it with
// get_operation_status.
//
// Deliberately NOT the domain's OWN SSL settings and managed/uploaded
// certificate workflow (issue #73) — see arvancloud_ssl_tools.go's own
// note.
const arvanCloudAccountCertumNote = "This is ArvanCloud's separate account-level Certum certificate ordering/purchase " +
	"workflow, not the domain's OWN SSL settings and certificates (get/update_arvancloud_ssl_settings and friends)."

// arvanCloudCertificateProductToMap renders a domain.ArvanCloudCertificateProduct
// the way list_arvancloud_certificate_products reports it back to the caller.
func arvanCloudCertificateProductToMap(p domain.ArvanCloudCertificateProduct) map[string]any {
	return map[string]any{
		"id":           p.ID,
		"name":         p.Name,
		"is_multi":     p.IsMulti,
		"has_wildcard": p.HasWildcard,
		"limit":        p.Limit,
		"price":        p.Price,
	}
}

// arvanCloudAccountCertificateOrderToMap renders a
// domain.ArvanCloudAccountCertificateOrder the way every order-returning
// tool in this file reports it back to the caller.
func arvanCloudAccountCertificateOrderToMap(o domain.ArvanCloudAccountCertificateOrder) map[string]any {
	errs := o.Errors
	if errs == nil {
		errs = map[string]any{}
	}
	return map[string]any{
		"id":           o.ID,
		"order_id":     o.OrderID,
		"status":       string(o.Status),
		"domain_names": o.DomainNames,
		"product":      o.Product,
		"errors":       errs,
		"is_revoked":   o.IsRevoked,
		"expiry_date":  o.ExpiryDate,
		"created_at":   o.CreatedAt,
		"updated_at":   o.UpdatedAt,
	}
}

func listArvanCloudCertificateProductsTool(uc *app.ListArvanCloudCertificateProducts) Tool {
	props := credentialProperties()

	return Tool{
		Name: "list_arvancloud_certificate_products",
		Description: "List every purchasable Certum certificate product, including each one's price. " + arvanCloudAccountCertumNote +
			" Call this before issue_arvancloud_account_certificate so the caller (and, through it, the end user) sees " +
			"the cost up front — this project has no payment-confirmation UI of its own. This is a fast operation.",
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

			products, err := uc.Execute(ctx, app.ListArvanCloudCertificateProductsInput{Credentials: args.domain()})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(products))
			for i, p := range products {
				out[i] = arvanCloudCertificateProductToMap(p)
			}
			return map[string]any{"products": out}, nil
		},
	}
}

// arvanCloudCertificateIssueDomainArgs is one entry of an
// issue_arvancloud_account_certificate call's domains array.
type arvanCloudCertificateIssueDomainArgs struct {
	DomainID    string   `json:"domain_id"`
	DomainNames []string `json:"domain_names"`
}

func issueArvanCloudAccountCertificateTool(uc *app.IssueArvanCloudAccountCertificate) Tool {
	props := credentialProperties()
	props["domains"] = map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"domain_id": map[string]any{
					"type": "string",
					"description": "The target domain's INTERNAL ID (a UUID from list_arvancloud_domains/get_arvancloud_domain's " +
						"\"id\" field) — NOT the domain name. This is the one ArvanCloud operation that addresses a domain " +
						"this way instead of by hostname.",
				},
				"domain_names": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Hostnames on this domain the certificate should cover, e.g. [\"example.com\", \"www.example.com\"].",
				},
			},
			"required": []string{"domain_id", "domain_names"},
		},
		"description": "Which domain(s) and hostnames the certificate should cover. At least one entry is required.",
	}
	props["product_id"] = map[string]any{
		"type":        "string",
		"description": "The certificate product to purchase, from list_arvancloud_certificate_products's \"id\" field.",
	}
	props["common_name"] = map[string]any{
		"type":        "string",
		"description": "The certificate's primary subject name, e.g. \"example.com\". Optional — the provider derives one from the domains when omitted.",
	}
	props["private_key_size"] = map[string]any{
		"type":        "integer",
		"enum":        []int{2048, 4096},
		"description": "Key size ArvanCloud/Certum generates for the certificate. Optional; when omitted the provider chooses its own default.",
	}

	return Tool{
		Name: "issue_arvancloud_account_certificate",
		Description: "Purchase and request issuance of a PAID Certum-branded SSL certificate against the account (see " +
			"list_arvancloud_certificate_products for pricing — check that first, this is a real cost). " + arvanCloudAccountCertumNote +
			" This is a LONG operation: it returns immediately with an operation_id and status \"pending\" — issuance " +
			"involves CA validation that can take some time. Poll get_operation_status with that id to learn when the " +
			"certificate is ready, then call install_arvancloud_account_certificate to actually activate it for " +
			"serving traffic (issuance alone does not). Calling this again for the same domain names while an order " +
			"is already in flight does not start a second one.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domains", "product_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				credentialArgs
				Domains        []arvanCloudCertificateIssueDomainArgs `json:"domains"`
				ProductID      string                                 `json:"product_id"`
				CommonName     string                                 `json:"common_name"`
				PrivateKeySize int                                    `json:"private_key_size"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			domains := make([]domain.ArvanCloudCertificateIssueDomain, len(args.Domains))
			for i, d := range args.Domains {
				domains[i] = domain.ArvanCloudCertificateIssueDomain{DomainID: d.DomainID, DomainNames: d.DomainNames}
			}

			out, err := uc.Execute(ctx, app.IssueArvanCloudAccountCertificateInput{
				Credentials: args.domain(),
				Request: domain.ArvanCloudCertificateOrderIssueRequest{
					Domains:        domains,
					ProductID:      args.ProductID,
					CommonName:     args.CommonName,
					PrivateKeySize: args.PrivateKeySize,
				},
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"operation_id": out.OperationID,
				"status":       out.Status.String(),
				"note":         "Certificate issuance runs in the background. Call get_operation_status with this operation_id to check progress, then install_arvancloud_account_certificate once it succeeds.",
			}, nil
		},
	}
}

func listArvanCloudAccountCertificateOrdersTool(uc *app.ListArvanCloudAccountCertificateOrders) Tool {
	props := credentialProperties()

	return Tool{
		Name: "list_arvancloud_account_certificate_orders",
		Description: "Get the account's full Certum certificate order history (account-wide, not scoped to one domain). " +
			arvanCloudAccountCertumNote + " This is a fast operation.",
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

			orders, err := uc.Execute(ctx, app.ListArvanCloudAccountCertificateOrdersInput{Credentials: args.domain()})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(orders))
			for i, o := range orders {
				out[i] = arvanCloudAccountCertificateOrderToMap(o)
			}
			return map[string]any{"orders": out}, nil
		},
	}
}

// arvanCloudAccountCertificateOrderIDArgs is embedded by every tool below
// that is scoped to exactly one order by id.
type arvanCloudAccountCertificateOrderIDArgs struct {
	credentialArgs
	OrderID string `json:"order_id"`
}

func arvanCloudAccountCertificateOrderIDProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The order's provider-assigned ID (a UUID), from list_arvancloud_account_certificate_orders or issue_arvancloud_account_certificate.",
	}
}

func getArvanCloudAccountCertificateOrderTool(uc *app.GetArvanCloudAccountCertificateOrder) Tool {
	props := credentialProperties()
	props["order_id"] = arvanCloudAccountCertificateOrderIDProperty()
	props["keep_private_key"] = map[string]any{
		"type": "boolean",
		"description": "Whether the private key ArvanCloud/Certum generated for this certificate should be retained on " +
			"ArvanCloud's servers. Set to false to permanently delete it after this call — it can then no longer be " +
			"retrieved or viewed, so make sure it has been stored securely first if it is still needed. Omit to leave " +
			"the provider's own default (retained) in effect.",
	}

	return Tool{
		Name:        "get_arvancloud_account_certificate_order",
		Description: "Get the current state of one Certum certificate order by ID. " + arvanCloudAccountCertumNote + " This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "order_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudAccountCertificateOrderIDArgs
				KeepPrivateKey *bool `json:"keep_private_key"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			found, err := uc.Execute(ctx, app.GetArvanCloudAccountCertificateOrderInput{
				Credentials: args.domain(), OrderID: args.OrderID, KeepPrivateKey: args.KeepPrivateKey,
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudAccountCertificateOrderToMap(*found), nil
		},
	}
}

func revokeArvanCloudAccountCertificateTool(uc *app.RevokeArvanCloudAccountCertificate) Tool {
	props := credentialProperties()
	props["order_id"] = arvanCloudAccountCertificateOrderIDProperty()

	return Tool{
		Name: "revoke_arvancloud_account_certificate",
		Description: "Revoke a Certum certificate order's certificate (e.g. a suspected private key compromise). " +
			arvanCloudAccountCertumNote + " This is a fast operation and cannot be undone.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "order_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudAccountCertificateOrderIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.RevokeArvanCloudAccountCertificateInput{Credentials: args.domain(), OrderID: args.OrderID})
			if err != nil {
				return nil, err
			}
			return arvanCloudAccountCertificateOrderToMap(*updated), nil
		},
	}
}

func reissueArvanCloudAccountCertificateTool(uc *app.ReissueArvanCloudAccountCertificate) Tool {
	props := credentialProperties()
	props["order_id"] = arvanCloudAccountCertificateOrderIDProperty()

	return Tool{
		Name: "reissue_arvancloud_account_certificate",
		Description: "Reissue a Certum certificate order's certificate — e.g. after a key compromise or a common-name " +
			"change, or as the manual recovery path for an order that failed permanently (status \"killed\"; there is " +
			"no separate \"retry\" action at the account level, unlike retry_arvancloud_ssl_order for the domain-scoped " +
			"flow). " + arvanCloudAccountCertumNote + " This is a fast operation: it places the reissue and returns " +
			"within this call, but the reissued certificate's own validation then proceeds asynchronously like the " +
			"original — check its status with get_arvancloud_account_certificate_order afterward.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "order_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudAccountCertificateOrderIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.ReissueArvanCloudAccountCertificateInput{Credentials: args.domain(), OrderID: args.OrderID})
			if err != nil {
				return nil, err
			}
			return arvanCloudAccountCertificateOrderToMap(*updated), nil
		},
	}
}

func installArvanCloudAccountCertificateTool(uc *app.InstallArvanCloudAccountCertificate) Tool {
	props := credentialProperties()
	props["order_id"] = arvanCloudAccountCertificateOrderIDProperty()

	return Tool{
		Name: "install_arvancloud_account_certificate",
		Description: "Install an ISSUED Certum certificate onto ArvanCloud's edge servers — the step that actually " +
			"activates it for serving HTTPS traffic; issuing the certificate (issue_arvancloud_account_certificate) " +
			"does not do this by itself. " + arvanCloudAccountCertumNote + " This is a fast operation: it completes " +
			"within this call and reports any per-domain warnings rather than an operation to poll.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "order_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudAccountCertificateOrderIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			result, err := uc.Execute(ctx, app.InstallArvanCloudAccountCertificateInput{Credentials: args.domain(), OrderID: args.OrderID})
			if err != nil {
				return nil, err
			}

			warnings := make([]map[string]any, len(result.Warnings))
			for i, w := range result.Warnings {
				warnings[i] = map[string]any{"domain": w.Domain, "reason": w.Reason}
			}
			return map[string]any{
				"success":  result.Success,
				"message":  result.Message,
				"warnings": warnings,
			}, nil
		},
	}
}
