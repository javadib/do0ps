package mcp

import (
	"context"
	"encoding/json"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// ArvanCloud SSL/TLS tools (issue #73): a domain's SSL settings plus the
// certificates (customer-uploaded or ArvanCloud-managed) attached to it, and
// the account-side workflow that drives a managed certificate to issuance.
// All fast operations except issue_arvancloud_managed_certificate, the one
// long operation (AGENTS.md 4.3) — it returns an operation_id, poll it with
// get_operation_status.
//
// Deliberately NOT the account-scoped Certum certificate ORDERING workflow
// (issue #74/AC14) — see domain/arvancloud_ssl.go's package comment.
const arvanCloudSslCertumNote = "This is the domain's OWN SSL settings and certificates, not ArvanCloud's separate account-level Certum certificate ordering/purchase workflow."

// arvanCloudSslSettingsToMap renders a domain.ArvanCloudSslSettings the way
// get/update_arvancloud_ssl_settings report it back to the caller. Certificate
// (the write-only "which certificate is active" selector) is intentionally
// omitted: it is never populated on a read, and echoing back whatever the
// caller last sent on an update would be misleading once the provider has
// since replaced/renewed the active certificate.
func arvanCloudSslSettingsToMap(s domain.ArvanCloudSslSettings) map[string]any {
	certs := make([]map[string]any, len(s.Certificates))
	for i, c := range s.Certificates {
		certs[i] = arvanCloudCertificateToMap(c)
	}
	orders := make([]map[string]any, len(s.Orders))
	for i, o := range s.Orders {
		orders[i] = arvanCloudCertificateOrderToMap(o)
	}
	return map[string]any{
		"fingerprint_status":   s.FingerprintStatus,
		"ssl_status":           s.SSLStatus,
		"certificate_mode":     string(s.CertificateMode),
		"tls_version":          string(s.TLSVersion),
		"hsts_status":          s.HSTSStatus,
		"hsts_max_age":         s.HSTSMaxAge,
		"hsts_subdomain":       s.HSTSSubdomain,
		"hsts_preload":         s.HSTSPreload,
		"quic_status":          s.QUICStatus,
		"verify_sni":           s.VerifySNI,
		"https_redirect":       s.HTTPSRedirect,
		"replace_http":         s.ReplaceHTTP,
		"certificate_key_type": string(s.CertificateKeyType),
		"certificates":         certs,
		"orders":               orders,
	}
}

// arvanCloudCertificateToMap renders a domain.ArvanCloudCertificate the way
// every certificate-returning tool reports it back to the caller.
func arvanCloudCertificateToMap(c domain.ArvanCloudCertificate) map[string]any {
	return map[string]any{
		"id":           c.ID,
		"type":         string(c.Type),
		"active":       c.Active,
		"key_type":     string(c.KeyType),
		"domain_names": c.DomainNames,
		"issuer":       c.Issuer,
		"is_revoked":   c.IsRevoked,
		"expiry_date":  c.ExpiryDate,
		"created_at":   c.CreatedAt,
		"updated_at":   c.UpdatedAt,
	}
}

// arvanCloudCertificateOrderToMap renders a domain.ArvanCloudCertificateOrder
// the way every order-returning tool reports it back to the caller.
func arvanCloudCertificateOrderToMap(o domain.ArvanCloudCertificateOrder) map[string]any {
	return map[string]any{
		"id":           o.ID,
		"order_id":     o.OrderID,
		"status":       string(o.Status),
		"domain_names": o.DomainNames,
		"errors": map[string]any{
			"type":   o.Errors.Type,
			"detail": o.Errors.Detail,
			"status": o.Errors.Status,
		},
		"expiry_date": o.ExpiryDate,
		"created_at":  o.CreatedAt,
		"updated_at":  o.UpdatedAt,
	}
}

// --- Per-domain SSL settings ------------------------------------------------

func getArvanCloudSslSettingsTool(uc *app.GetArvanCloudSslSettings) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "get_arvancloud_ssl_settings",
		Description: "Get a domain's SSL/TLS configuration: whether SSL is enabled, which certificate mode is active " +
			"(\"managed\" — ArvanCloud automatically issues/renews the certificate — or \"custom\" — a certificate the " +
			"caller uploaded), minimum TLS version, HSTS settings, QUIC, and the certificates/order history attached " +
			"to the domain. " + arvanCloudSslCertumNote + " This is a fast operation.",
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

			found, err := uc.Execute(ctx, app.GetArvanCloudSslSettingsInput{Credentials: args.domain(), Domain: args.Domain})
			if err != nil {
				return nil, err
			}
			return arvanCloudSslSettingsToMap(*found), nil
		},
	}
}

func arvanCloudTlsVersionProperty() map[string]any {
	return map[string]any{
		"type": "string",
		"enum": []string{
			string(domain.ArvanCloudTlsVersionDefault),
			string(domain.ArvanCloudTlsVersion10),
			string(domain.ArvanCloudTlsVersion11),
			string(domain.ArvanCloudTlsVersion12),
			string(domain.ArvanCloudTlsVersion13),
		},
		"description": "Minimum TLS protocol version the domain accepts. Empty string (\"\") means ArvanCloud's own " +
			"default (no minimum enforced beyond that). Example: \"TLSv1.2\" to reject TLS 1.0/1.1 connections.",
	}
}

func arvanCloudCertificateKeyTypeProperty(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"enum":        []string{string(domain.ArvanCloudCertificateKeyTypeRSA), string(domain.ArvanCloudCertificateKeyTypeEC)},
		"description": description,
	}
}

func updateArvanCloudSslSettingsTool(uc *app.UpdateArvanCloudSslSettings) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["fingerprint_status"] = map[string]any{"type": "boolean", "description": "Enable ArvanCloud's \"fingerprint\" feature for the domain."}
	props["ssl_status"] = map[string]any{"type": "boolean", "description": "Enable the domain's SSL module. When false, HTTPS is not served for the domain at all."}
	props["tls_version"] = arvanCloudTlsVersionProperty()
	props["hsts_status"] = map[string]any{"type": "boolean", "description": "Enable HTTP Strict Transport Security (HSTS) for the domain."}
	props["hsts_max_age"] = map[string]any{
		"type":        "string",
		"enum":        []string{"1mo", "2mo", "3mo", "4mo", "5mo", "6mo", "12mo", "24mo"},
		"description": "HSTS max-age directive duration. Only meaningful when hsts_status is true. Omit to leave unchanged.",
	}
	props["hsts_subdomain"] = map[string]any{"type": "boolean", "description": "Add the \"includeSubDomains\" directive to the HSTS header."}
	props["hsts_preload"] = map[string]any{"type": "boolean", "description": "Add the \"preload\" directive to the HSTS header."}
	props["quic_status"] = map[string]any{"type": "boolean", "description": "Enable the QUIC transport protocol for the domain."}
	props["verify_sni"] = map[string]any{"type": "boolean", "description": "Require the TLS SNI hostname to match the requested Host header."}
	props["https_redirect"] = map[string]any{"type": "boolean", "description": "Redirect HTTP requests to HTTPS."}
	props["replace_http"] = map[string]any{"type": "boolean", "description": "Rewrite \"http://\" to \"https://\" in the domain's own HTML/JavaScript responses."}
	props["certificate_key_type"] = arvanCloudCertificateKeyTypeProperty("Key algorithm ArvanCloud uses when it issues a managed certificate for the domain.")
	props["certificate"] = map[string]any{
		"type": "string",
		"description": "Selects which certificate serves the domain: send the literal string \"managed\" to switch to " +
			"ArvanCloud's automatically issued/renewed certificate, or an already-uploaded certificate's ID (from " +
			"upload_arvancloud_certificate or list_arvancloud_certificates) to switch to that one. Omit to leave the " +
			"active certificate unchanged.",
	}

	return Tool{
		Name: "update_arvancloud_ssl_settings",
		Description: "Update a domain's SSL/TLS configuration (minimum TLS version, HSTS, QUIC, SNI verification, " +
			"HTTPS redirect, and which certificate is active). " + arvanCloudSslCertumNote + " To switch between " +
			"ArvanCloud's managed certificate and a customer-uploaded one, set the certificate field. This is a fast " +
			"operation: the updated settings are returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "tls_version", "certificate_key_type"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				FingerprintStatus  bool   `json:"fingerprint_status"`
				SSLStatus          bool   `json:"ssl_status"`
				TLSVersion         string `json:"tls_version"`
				HSTSStatus         bool   `json:"hsts_status"`
				HSTSMaxAge         string `json:"hsts_max_age"`
				HSTSSubdomain      bool   `json:"hsts_subdomain"`
				HSTSPreload        bool   `json:"hsts_preload"`
				QUICStatus         bool   `json:"quic_status"`
				VerifySNI          bool   `json:"verify_sni"`
				HTTPSRedirect      bool   `json:"https_redirect"`
				ReplaceHTTP        bool   `json:"replace_http"`
				CertificateKeyType string `json:"certificate_key_type"`
				Certificate        string `json:"certificate"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			updated, err := uc.Execute(ctx, app.UpdateArvanCloudSslSettingsInput{
				Credentials: args.domain(),
				Domain:      args.Domain,
				Settings: domain.ArvanCloudSslSettings{
					FingerprintStatus:  args.FingerprintStatus,
					SSLStatus:          args.SSLStatus,
					TLSVersion:         domain.ArvanCloudTlsVersion(args.TLSVersion),
					HSTSStatus:         args.HSTSStatus,
					HSTSMaxAge:         args.HSTSMaxAge,
					HSTSSubdomain:      args.HSTSSubdomain,
					HSTSPreload:        args.HSTSPreload,
					QUICStatus:         args.QUICStatus,
					VerifySNI:          args.VerifySNI,
					HTTPSRedirect:      args.HTTPSRedirect,
					ReplaceHTTP:        args.ReplaceHTTP,
					CertificateKeyType: domain.ArvanCloudCertificateKeyType(args.CertificateKeyType),
					Certificate:        args.Certificate,
				},
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudSslSettingsToMap(*updated), nil
		},
	}
}

// --- Certificates ------------------------------------------------------

// arvanCloudCertificateIDArgs is embedded by every certificate tool below
// that is scoped to exactly one certificate by domain + id.
type arvanCloudCertificateIDArgs struct {
	arvanCloudDomainNameArgs
	CertificateID string `json:"certificate_id"`
}

func arvanCloudCertificateIDProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"description": "The certificate's provider-assigned ID (a UUID), as returned by upload_arvancloud_certificate or list_arvancloud_certificates.",
	}
}

func listArvanCloudCertificatesTool(uc *app.ListArvanCloudCertificates) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name:        "list_arvancloud_certificates",
		Description: "List every certificate attached to a domain, both ArvanCloud-managed and customer-uploaded. " + arvanCloudSslCertumNote + " This is a fast operation.",
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

			certs, err := uc.Execute(ctx, app.ListArvanCloudCertificatesInput{Credentials: args.domain(), Domain: args.Domain})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(certs))
			for i, c := range certs {
				out[i] = arvanCloudCertificateToMap(c)
			}
			return map[string]any{"certificates": out}, nil
		},
	}
}

func uploadArvanCloudCertificateTool(uc *app.UploadArvanCloudCertificate) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["certificate"] = map[string]any{
		"type":        "string",
		"description": "The certificate in PEM format (\"-----BEGIN CERTIFICATE-----...\"), including any intermediate chain the domain owner was given.",
	}
	props["private_key"] = map[string]any{
		"type": "string",
		"description": "The certificate's private key in PEM format (\"-----BEGIN PRIVATE KEY-----...\" or " +
			"\"-----BEGIN RSA PRIVATE KEY-----...\"). This is caller-supplied sensitive material passed straight " +
			"through to ArvanCloud — never generate, guess, or invent a value for this field; only send a key the " +
			"caller (the domain/certificate owner) actually provided.",
	}

	return Tool{
		Name: "upload_arvancloud_certificate",
		Description: "Upload a customer-owned SSL certificate and its private key for a domain (the \"custom\" " +
			"certificate_mode path — see update_arvancloud_ssl_settings). " + arvanCloudSslCertumNote +
			" IMPORTANT: private_key is caller-supplied sensitive material — never generate, guess, or invent this " +
			"value; only send one the caller actually provided. This is a fast operation, but the response carries " +
			"no certificate ID: call list_arvancloud_certificates afterward to find the new certificate, then " +
			"activate it with update_arvancloud_ssl_settings's certificate field.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "certificate", "private_key"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args struct {
				arvanCloudDomainNameArgs
				Certificate string `json:"certificate"`
				PrivateKey  string `json:"private_key"`
			}
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.UploadArvanCloudCertificateInput{
				Credentials:    args.domain(),
				Domain:         args.Domain,
				CertificatePEM: []byte(args.Certificate),
				PrivateKeyPEM:  []byte(args.PrivateKey),
			}); err != nil {
				return nil, err
			}
			return map[string]any{
				"uploaded": true,
				"domain":   args.Domain,
				"note":     "Call list_arvancloud_certificates to find the new certificate's ID.",
			}, nil
		},
	}
}

func getArvanCloudCertificateTool(uc *app.GetArvanCloudCertificate) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["certificate_id"] = arvanCloudCertificateIDProperty()

	return Tool{
		Name:        "get_arvancloud_certificate",
		Description: "Get the current state of one certificate attached to a domain by ID. " + arvanCloudSslCertumNote + " This is a fast operation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "certificate_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudCertificateIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			found, err := uc.Execute(ctx, app.GetArvanCloudCertificateInput{
				Credentials: args.domain(), Domain: args.Domain, CertificateID: args.CertificateID,
			})
			if err != nil {
				return nil, err
			}
			return arvanCloudCertificateToMap(*found), nil
		},
	}
}

func deleteArvanCloudCertificateTool(uc *app.DeleteArvanCloudCertificate) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["certificate_id"] = arvanCloudCertificateIDProperty()

	return Tool{
		Name: "delete_arvancloud_certificate",
		Description: "Permanently delete an unused certificate from a domain by ID. " + arvanCloudSslCertumNote +
			" This is a fast operation and cannot be undone. Deleting a certificate that no longer exists is " +
			"treated as already done rather than an error. The provider rejects deleting a certificate that is " +
			"currently active for the domain.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "certificate_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudCertificateIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteArvanCloudCertificateInput{
				Credentials: args.domain(), Domain: args.Domain, CertificateID: args.CertificateID,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "domain": args.Domain, "certificate_id": args.CertificateID}, nil
		},
	}
}

func revokeArvanCloudCertificateTool(uc *app.RevokeArvanCloudCertificate) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()
	props["certificate_id"] = arvanCloudCertificateIDProperty()

	return Tool{
		Name: "revoke_arvancloud_certificate",
		Description: "Revoke a certificate for security reasons (e.g. a suspected private key compromise). " +
			arvanCloudSslCertumNote + " This is a fast operation and cannot be undone.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "domain", "certificate_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args arvanCloudCertificateIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.RevokeArvanCloudCertificateInput{
				Credentials: args.domain(), Domain: args.Domain, CertificateID: args.CertificateID,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"revoked": true, "domain": args.Domain, "certificate_id": args.CertificateID}, nil
		},
	}
}

// --- Managed-certificate orders ---------------------------------------------

func listArvanCloudSslOrdersTool(uc *app.ListArvanCloudSslOrders) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "list_arvancloud_ssl_orders",
		Description: "Get a domain's managed-certificate order history: every attempt to issue an ArvanCloud-managed " +
			"certificate for the domain, with its current status. " + arvanCloudSslCertumNote + " This is a fast " +
			"operation. Use this to check on an order issue_arvancloud_managed_certificate started, or to find a " +
			"\"killed\" order to retry with retry_arvancloud_ssl_order.",
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

			orders, err := uc.Execute(ctx, app.ListArvanCloudSslOrdersInput{Credentials: args.domain(), Domain: args.Domain})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(orders))
			for i, o := range orders {
				out[i] = arvanCloudCertificateOrderToMap(o)
			}
			return map[string]any{"orders": out}, nil
		},
	}
}

func issueArvanCloudManagedCertificateTool(uc *app.IssueArvanCloudManagedCertificate) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "issue_arvancloud_managed_certificate",
		Description: "Request issuance of a FREE, automatically-issued-and-renewed ArvanCloud-managed SSL certificate " +
			"for a domain (the \"managed\" certificate_mode path — see update_arvancloud_ssl_settings). Use this " +
			"when the caller wants ArvanCloud to handle the certificate itself, rather than uploading their own via " +
			"upload_arvancloud_certificate. " + arvanCloudSslCertumNote + " This is a LONG operation: it returns " +
			"immediately with an operation_id and status \"pending\" — issuance involves domain-ownership validation " +
			"that ArvanCloud performs automatically and can take some time. Poll get_operation_status with that id " +
			"to learn when the certificate is ready, or call list_arvancloud_ssl_orders to see the order's progress " +
			"directly. Calling this again while an order is already in flight for the domain does not start a " +
			"second one.",
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

			out, err := uc.Execute(ctx, app.IssueArvanCloudManagedCertificateInput{Credentials: args.domain(), Domain: args.Domain})
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"operation_id": out.OperationID,
				"status":       out.Status.String(),
				"note":         "Certificate issuance runs in the background. Call get_operation_status with this operation_id to check progress.",
			}, nil
		},
	}
}

func retryArvanCloudSslOrderTool(uc *app.RetryArvanCloudSslOrder) Tool {
	props := credentialProperties()
	props["domain"] = arvanCloudDomainNameProperty()

	return Tool{
		Name: "retry_arvancloud_ssl_order",
		Description: "Manually retry a domain's managed-certificate order that reached the \"killed\" status (failed " +
			"despite many automatic retries — see list_arvancloud_ssl_orders). " + arvanCloudSslCertumNote +
			" This is a fast operation: it places the retry and returns within this call, but the retried order's " +
			"own issuance then proceeds asynchronously like the original — check its status with " +
			"list_arvancloud_ssl_orders afterward.",
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

			if err := uc.Execute(ctx, app.RetryArvanCloudSslOrderInput{Credentials: args.domain(), Domain: args.Domain}); err != nil {
				return nil, err
			}
			return map[string]any{"retried": true, "domain": args.Domain}, nil
		},
	}
}
