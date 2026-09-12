package domain

// The types below model ArvanCloud's Troubleshoot capability (issue #79):
// a domain-scoped built-in diagnostic tool that checks common configuration
// issues (DNS records, HTTPS redirection, certificate status, etc.) and
// reports whether each check passed or found a problem. Confirmed against
// docs/api-specs/arvancloud-cdn-4.0.yml's "Troubleshoot" tag (the
// troubleshoots.* operationIds) and the Troubleshoot schema.

// ArvanCloudTroubleshootDetailID identifies which diagnostic check a
// TroubleshootDetail entry represents (Troubleshoot.details[].id).
type ArvanCloudTroubleshootDetailID string

const (
	ArvanCloudTroubleshootDetailRootDNSRecord      ArvanCloudTroubleshootDetailID = "root_dns_record"
	ArvanCloudTroubleshootDetailWwwDNSRecord        ArvanCloudTroubleshootDetailID = "www_dns_record"
	ArvanCloudTroubleshootDetailMxDNSRecord         ArvanCloudTroubleshootDetailID = "mx_dns_record"
	ArvanCloudTroubleshootDetailHTTPSRedirection    ArvanCloudTroubleshootDetailID = "https_redirection"
	ArvanCloudTroubleshootDetailDomainActiveStatus  ArvanCloudTroubleshootDetailID = "domain_active_status"
	ArvanCloudTroubleshootDetailActiveCertificate   ArvanCloudTroubleshootDetailID = "active_certificate"
	ArvanCloudTroubleshootDetailCloudIcon           ArvanCloudTroubleshootDetailID = "cloud_icon"
	ArvanCloudTroubleshootDetailDomainExpirationDays ArvanCloudTroubleshootDetailID = "domain_expiration_days"
	ArvanCloudTroubleshootDetailOriginSSLPor        ArvanCloudTroubleshootDetailID = "origin_ssl_port"
)

var arvanCloudTroubleshootDetailIDs = []string{
	string(ArvanCloudTroubleshootDetailRootDNSRecord),
	string(ArvanCloudTroubleshootDetailWwwDNSRecord),
	string(ArvanCloudTroubleshootDetailMxDNSRecord),
	string(ArvanCloudTroubleshootDetailHTTPSRedirection),
	string(ArvanCloudTroubleshootDetailDomainActiveStatus),
	string(ArvanCloudTroubleshootDetailActiveCertificate),
	string(ArvanCloudTroubleshootDetailCloudIcon),
	string(ArvanCloudTroubleshootDetailDomainExpirationDays),
	string(ArvanCloudTroubleshootDetailOriginSSLPor),
}

// ValidArvanCloudTroubleshootDetailID reports whether s is one of the
// Troubleshoot.details[].id enum values.
func ValidArvanCloudTroubleshootDetailID(s string) bool {
	return contains(arvanCloudTroubleshootDetailIDs, s)
}

// ArvanCloudTroubleshootDetailStatus is the result of one diagnostic check
// (Troubleshoot.details[].status).
type ArvanCloudTroubleshootDetailStatus string

const (
	ArvanCloudTroubleshootDetailStatusSafe     ArvanCloudTroubleshootDetailStatus = "safe"
	ArvanCloudTroubleshootDetailStatusTroubled ArvanCloudTroubleshootDetailStatus = "troubled"
)

var arvanCloudTroubleshootDetailStatuses = []string{
	string(ArvanCloudTroubleshootDetailStatusSafe),
	string(ArvanCloudTroubleshootDetailStatusTroubled),
}

// ValidArvanCloudTroubleshootDetailStatus reports whether s is one of the
// Troubleshoot.details[].status enum values.
func ValidArvanCloudTroubleshootDetailStatus(s string) bool {
	return contains(arvanCloudTroubleshootDetailStatuses, s)
}

// ArvanCloudTroubleshootDetail is one diagnostic finding within a
// troubleshoot run (the Troubleshoot schema's details[] items).
type ArvanCloudTroubleshootDetail struct {
	// ID identifies which check was performed. Must be one of
	// ValidArvanCloudTroubleshootDetailID's values.
	ID ArvanCloudTroubleshootDetailID
	// Status reports whether this check passed ("safe") or found a problem
	// ("troubled").
	Status ArvanCloudTroubleshootDetailStatus
	// Details is a human-readable explanation of the finding.
	Details string
}

// ArvanCloudTroubleshoot is a single troubleshoot run on a domain
// (the Troubleshoot schema).
type ArvanCloudTroubleshoot struct {
	// ID is the provider-assigned UUID for this run.
	ID string
	// Details is the list of diagnostic findings from this run.
	Details []ArvanCloudTroubleshootDetail
	// CreatedAt is the ISO 8601 timestamp of when this run was started.
	CreatedAt string
}
