package mcp

import (
	"context"
	"encoding/json"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// challengeSetToMap renders a domain.SSLChallengeSet the way every
// challenge-returning tool reports it back to the caller.
func challengeSetToMap(set domain.SSLChallengeSet) map[string]any {
	methods := make(map[string]any, len(set.Methods))
	for key, m := range set.Methods {
		challenges := make([]map[string]any, len(m.Challenges))
		for i, c := range m.Challenges {
			challenges[i] = map[string]any{
				"domain":    c.Domain,
				"token":     c.Token,
				"verified":  c.Verified,
				"type":      c.Type,
				"file_name": c.FileName,
				"file_path": c.FilePath,
			}
		}
		methods[key] = map[string]any{"method": m.Method, "name": m.Name, "challenges": challenges}
	}

	allowed := make([]map[string]any, len(set.AllowedMethods))
	for i, a := range set.AllowedMethods {
		allowed[i] = map[string]any{"method": a.Method, "name": a.Name, "prefixes": a.Prefixes}
	}

	return map[string]any{
		"methods":         methods,
		"allowed_methods": allowed,
		"deadline":        set.Deadline,
	}
}

// certificateToMap renders a domain.SSLCertificate the way every
// certificate-returning tool reports it back to the caller.
func certificateToMap(cert domain.SSLCertificate) map[string]any {
	out := map[string]any{"ready": cert.Ready}
	if cert.Ready {
		out["certificate"] = cert.Certificate
		out["ca_bundle"] = cert.CABundle
	}
	return out
}

func orderIDProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The provider's encrypted order ID, as returned by create_ssl_order.",
	}
}

func listSSLProductsTool(uc *app.ListSSLProducts) Tool {
	props := credentialProperties()

	return Tool{
		Name: "list_ssl_products",
		Description: "List every SSL certificate product available to order at Parspack (DV, OV, EV, wildcard, " +
			"multi-domain), with pricing. Use the returned slug as product_slug when calling create_ssl_order. This " +
			"is a fast operation.",
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

			products, err := uc.Execute(ctx, app.ListSSLProductsInput{Credentials: args.domain()})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(products))
			for i, p := range products {
				prices := make(map[string]any, len(p.Prices))
				for cycle, price := range p.Prices {
					prices[cycle] = map[string]any{"price": price.Price, "setup_fee": price.SetupFee, "currency": price.Currency}
				}
				out[i] = map[string]any{
					"slug":          p.Slug,
					"title":         p.Title,
					"description":   p.Description,
					"product_group": p.ProductGroup,
					"wildcard":      p.Wildcard,
					"multi_domain":  p.MultiDomain,
					"available":     p.Available,
					"prices":        prices,
				}
			}
			return map[string]any{"products": out}, nil
		},
	}
}

type sanArg struct {
	Name string `json:"name"`
	IsIP bool   `json:"is_ip"`
	WWW  bool   `json:"www"`
}

type createSSLOrderArgs struct {
	credentialArgs
	ProductSlug  string   `json:"product_slug"`
	Domain       string   `json:"domain"`
	BillingCycle string   `json:"billing_cycle"`
	WWW          bool     `json:"www"`
	PromoCode    string   `json:"promocode"`
	SANs         []sanArg `json:"sans"`
}

func createSSLOrderTool(uc *app.CreateSSLOrder) Tool {
	props := credentialProperties()
	props["product_slug"] = map[string]any{
		"type":        "string",
		"description": "The SSL product to order, as returned by list_ssl_products, e.g. \"dv-ssl\".",
	}
	props["domain"] = map[string]any{
		"type":        "string",
		"description": "The primary domain to secure, e.g. \"example.com\". No protocol or www prefix.",
	}
	props["billing_cycle"] = map[string]any{
		"type":        "string",
		"enum":        []string{"annually", "biennially", "triennially"},
		"description": "Billing cycle for the certificate.",
	}
	props["www"] = map[string]any{
		"type":        "boolean",
		"description": "Also secure the www subdomain (both example.com and www.example.com).",
	}
	props["promocode"] = map[string]any{
		"type":        "string",
		"description": "Optional promotional code for a discount.",
	}
	props["sans"] = map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":  map[string]any{"type": "string", "description": "Additional domain or IP address to secure, e.g. \"api.example.com\"."},
				"is_ip": map[string]any{"type": "boolean", "description": "Whether name is an IP address rather than a domain."},
				"www":   map[string]any{"type": "boolean", "description": "Also secure the www subdomain of this SAN."},
			},
			"required": []string{"name"},
		},
		"description": "Additional Subject Alternative Names for a multi-domain certificate. Omit for a single-domain order.",
	}

	return Tool{
		Name: "create_ssl_order",
		Description: "Place a new SSL certificate order at Parspack. This is a fast operation: it returns the " +
			"order_id and its invoice immediately. The invoice must be paid before process_ssl_order can run.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "product_slug", "domain", "billing_cycle"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args createSSLOrderArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			sans := make([]domain.SSLSAN, len(args.SANs))
			for i, s := range args.SANs {
				sans[i] = domain.SSLSAN{Name: s.Name, IsIP: s.IsIP, WWW: s.WWW}
			}

			order, err := uc.Execute(ctx, app.CreateSSLOrderInput{
				Credentials: args.domain(),
				Spec: domain.SSLOrderSpec{
					ProductSlug:  args.ProductSlug,
					Domain:       args.Domain,
					BillingCycle: args.BillingCycle,
					WWW:          args.WWW,
					PromoCode:    args.PromoCode,
					SANs:         sans,
				},
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"order_id":       order.ID,
				"invoice_id":     order.InvoiceID,
				"invoice_status": order.InvoiceStatus,
				"note":           "Pay the invoice, then call process_ssl_order with the CSR and contact details to continue.",
			}, nil
		},
	}
}

type processSSLOrderArgs struct {
	credentialArgs
	OrderID             string `json:"order_id"`
	CSR                 string `json:"csr"`
	FirstName           string `json:"first_name"`
	LastName            string `json:"last_name"`
	Country             string `json:"country"`
	City                string `json:"city"`
	Address             string `json:"address"`
	Phone               string `json:"phone"`
	Email               string `json:"email"`
	PostalCode          string `json:"postal_code"`
	JurisdictionCountry string `json:"jurisdiction_country"`
	BusinessCategory    string `json:"business_category"`
	RegistrationNumber  string `json:"registration_number"`
}

func processSSLOrderTool(uc *app.ProcessSSLOrder) Tool {
	props := credentialProperties()
	props["order_id"] = orderIDProperty()
	props["csr"] = map[string]any{
		"type":        "string",
		"description": "The Certificate Signing Request in PEM format, matching the order's domain.",
	}
	props["first_name"] = map[string]any{"type": "string", "description": "Applicant first name."}
	props["last_name"] = map[string]any{"type": "string", "description": "Applicant last name."}
	props["country"] = map[string]any{"type": "string", "description": "ISO 3166-1 alpha-2 country code, e.g. \"US\"."}
	props["city"] = map[string]any{"type": "string", "description": "Applicant city."}
	props["address"] = map[string]any{"type": "string", "description": "Applicant street address."}
	props["phone"] = map[string]any{"type": "string", "description": "Applicant phone number with country code, digits only, e.g. \"12025551234\"."}
	props["email"] = map[string]any{"type": "string", "description": "Applicant email address, used for certificate delivery and, for EMAIL verification, the ownership challenge link."}
	props["postal_code"] = map[string]any{"type": "string", "description": "Applicant postal/ZIP code. Optional."}
	props["jurisdiction_country"] = map[string]any{
		"type":        "string",
		"description": "Jurisdiction country code for OV/EV products (ISO 3166-1 alpha-2). Optional, ignored for DV products.",
	}
	props["business_category"] = map[string]any{
		"type":        "string",
		"description": "Business category for OV/EV products, one of \"Private Organization\", \"Government Entity\", \"Business Entity\", \"Non-Commercial Entity\". Optional, ignored for DV products.",
	}
	props["registration_number"] = map[string]any{
		"type":        "string",
		"description": "Company registration number for OV/EV products. Optional, ignored for DV products.",
	}

	return Tool{
		Name: "process_ssl_order",
		Description: "Submit the CSR and contact details for a paid SSL order at Parspack, and receive the " +
			"domain-ownership verification challenges to complete next. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "order_id", "csr", "first_name", "last_name", "country", "city", "address", "phone", "email"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args processSSLOrderArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			set, err := uc.Execute(ctx, app.ProcessSSLOrderInput{
				Credentials: args.domain(),
				OrderID:     args.OrderID,
				CSR:         args.CSR,
				Contact: domain.SSLContact{
					FirstName:           args.FirstName,
					LastName:            args.LastName,
					Country:             args.Country,
					City:                args.City,
					Address:             args.Address,
					Phone:               args.Phone,
					Email:               args.Email,
					PostalCode:          args.PostalCode,
					JurisdictionCountry: args.JurisdictionCountry,
					BusinessCategory:    args.BusinessCategory,
					RegistrationNumber:  args.RegistrationNumber,
				},
			})
			if err != nil {
				return nil, err
			}
			return challengeSetToMap(*set), nil
		},
	}
}

type orderIDArgs struct {
	credentialArgs
	OrderID string `json:"order_id"`
}

func getSSLChallengeTool(uc *app.GetSSLChallenge) Tool {
	props := credentialProperties()
	props["order_id"] = orderIDProperty()

	return Tool{
		Name: "get_ssl_challenge",
		Description: "Show the domain-ownership verification challenges of a processed SSL order at Parspack, " +
			"e.g. to display verification instructions again. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "order_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args orderIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			set, err := uc.Execute(ctx, app.GetSSLChallengeInput{Credentials: args.domain(), OrderID: args.OrderID})
			if err != nil {
				return nil, err
			}
			return challengeSetToMap(*set), nil
		},
	}
}

type reloadSSLChallengeArgs struct {
	credentialArgs
	OrderID     string `json:"order_id"`
	Method      string `json:"method"`
	EmailPrefix string `json:"email_prefix"`
}

func reloadSSLChallengeTool(uc *app.ReloadSSLChallenge) Tool {
	props := credentialProperties()
	props["order_id"] = orderIDProperty()
	props["method"] = map[string]any{
		"type":        "string",
		"enum":        []string{"DNS_TXT", "FILE", "ADMIN", "DNS_CNAME"},
		"description": "The verification method to switch to. Switching invalidates any previously shown challenge tokens.",
	}
	props["email_prefix"] = map[string]any{
		"type":        "string",
		"enum":        []string{"ADMIN", "ADMINISTRATOR", "POSTMASTER", "HOSTMASTER", "WEBMASTER"},
		"description": "Mailbox prefix to send the verification email to, e.g. \"ADMIN\" for admin@<domain>. Only used when method is \"ADMIN\".",
	}

	return Tool{
		Name: "reload_ssl_challenge",
		Description: "Switch the domain-ownership verification method of a processed SSL order at Parspack " +
			"(e.g. from FILE to DNS_TXT), generating new challenge tokens. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "order_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args reloadSSLChallengeArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			set, err := uc.Execute(ctx, app.ReloadSSLChallengeInput{
				Credentials: args.domain(),
				OrderID:     args.OrderID,
				Method:      args.Method,
				EmailPrefix: args.EmailPrefix,
			})
			if err != nil {
				return nil, err
			}
			return challengeSetToMap(*set), nil
		},
	}
}

type verifySSLChallengeArgs struct {
	credentialArgs
	OrderID string `json:"order_id"`
	Method  string `json:"method"`
}

func verifySSLChallengeTool(uc *app.VerifySSLChallenge) Tool {
	props := credentialProperties()
	props["order_id"] = orderIDProperty()
	props["method"] = map[string]any{
		"type":        "string",
		"enum":        []string{"DNS_TXT", "FILE", "ADMIN", "DNS_CNAME"},
		"description": "The verification method that was just completed (DNS record published, file uploaded, or email link clicked).",
	}

	return Tool{
		Name: "verify_ssl_challenge",
		Description: "Verify a completed domain-ownership challenge for a Parspack SSL order. On success for a DV " +
			"product the certificate is often ready immediately; otherwise poll get_ssl_certificate. OV/EV products " +
			"instead require manual document review. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "order_id", "method"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args verifySSLChallengeArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			result, err := uc.Execute(ctx, app.VerifySSLChallengeInput{
				Credentials: args.domain(),
				OrderID:     args.OrderID,
				Method:      args.Method,
			})
			if err != nil {
				return nil, err
			}
			out := map[string]any{
				"step":               result.Step,
				"certificate_ready":  result.CertificateReady,
				"requires_documents": result.RequiresDocuments,
			}
			if result.CertificateReady {
				out["certificate"] = result.Certificate
				out["ca_bundle"] = result.CABundle
			}
			if result.RequiresDocuments {
				out["product_group"] = result.ProductGroup
				out["message"] = result.Message
			}
			return out, nil
		},
	}
}

func getSSLCertificateTool(uc *app.GetSSLCertificate) Tool {
	props := credentialProperties()
	props["order_id"] = orderIDProperty()

	return Tool{
		Name: "get_ssl_certificate",
		Description: "Download the issued certificate for a Parspack SSL order. If ready is false, the " +
			"certificate is still being issued — wait and call this again later. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "order_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args orderIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			cert, err := uc.Execute(ctx, app.GetSSLCertificateInput{Credentials: args.domain(), OrderID: args.OrderID})
			if err != nil {
				return nil, err
			}
			return certificateToMap(*cert), nil
		},
	}
}

type reissueSSLCertificateArgs struct {
	credentialArgs
	OrderID string `json:"order_id"`
	CSR     string `json:"csr"`
}

func reissueSSLCertificateTool(uc *app.ReissueSSLCertificate) Tool {
	props := credentialProperties()
	props["order_id"] = orderIDProperty()
	props["csr"] = map[string]any{
		"type":        "string",
		"description": "The new Certificate Signing Request in PEM format, for the same domain as the original order.",
	}

	return Tool{
		Name: "reissue_ssl_certificate",
		Description: "Reissue an already-issued Parspack SSL certificate with a new CSR, e.g. after rotating the " +
			"private key. This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "order_id", "csr"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args reissueSSLCertificateArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			cert, err := uc.Execute(ctx, app.ReissueSSLCertificateInput{
				Credentials: args.domain(),
				OrderID:     args.OrderID,
				CSR:         args.CSR,
			})
			if err != nil {
				return nil, err
			}
			return certificateToMap(*cert), nil
		},
	}
}
