package domain

import "time"

// The types below are the shared shapes provider adapters translate into.
// Each provider sits behind its own dedicated port for now (see AGENTS.md 4.1),
// but every adapter returns these structs, so unifying those ports later is a
// mechanical change rather than a data-model rewrite.

// ServerStatus is the lifecycle state of a compute instance.
type ServerStatus int

const (
	ServerStatusUnknown ServerStatus = iota
	ServerStatusProvisioning
	ServerStatusRunning
	ServerStatusStopped
	ServerStatusDeleting
	ServerStatusError
)

// String returns the value reported to MCP callers.
func (s ServerStatus) String() string {
	switch s {
	case ServerStatusProvisioning:
		return "provisioning"
	case ServerStatusRunning:
		return "running"
	case ServerStatusStopped:
		return "stopped"
	case ServerStatusDeleting:
		return "deleting"
	case ServerStatusError:
		return "error"
	default:
		return "unknown"
	}
}

// Server is a compute instance at a provider.
type Server struct {
	ID        string
	Provider  string
	Name      string
	Region    string
	Status    ServerStatus
	CPUCores  int
	RAMMB     int
	DiskGB    int
	IPv4      string
	IPv6      string
	CreatedAt time.Time
}

// ServerSpec is the normalized request for a new compute instance.
type ServerSpec struct {
	Name     string
	Region   string
	Image    string
	PlanID   string // provider-specific plan/flavor when the caller names one
	CPUCores int
	RAMMB    int
	DiskGB   int
	SSHKeyID string
}

// DNSRecordType is the type of a DNS record.
type DNSRecordType int

const (
	DNSRecordTypeUnknown DNSRecordType = iota
	DNSRecordTypeA
	DNSRecordTypeAAAA
	DNSRecordTypeCNAME
	DNSRecordTypeTXT
	DNSRecordTypeMX
	DNSRecordTypeNS
	DNSRecordTypeSRV
)

// String returns the canonical DNS record type name.
func (t DNSRecordType) String() string {
	switch t {
	case DNSRecordTypeA:
		return "A"
	case DNSRecordTypeAAAA:
		return "AAAA"
	case DNSRecordTypeCNAME:
		return "CNAME"
	case DNSRecordTypeTXT:
		return "TXT"
	case DNSRecordTypeMX:
		return "MX"
	case DNSRecordTypeNS:
		return "NS"
	case DNSRecordTypeSRV:
		return "SRV"
	default:
		return "UNKNOWN"
	}
}

// ParseDNSRecordType converts a record type name into a DNSRecordType.
func ParseDNSRecordType(s string) (DNSRecordType, error) {
	switch s {
	case "A":
		return DNSRecordTypeA, nil
	case "AAAA":
		return DNSRecordTypeAAAA, nil
	case "CNAME":
		return DNSRecordTypeCNAME, nil
	case "TXT":
		return DNSRecordTypeTXT, nil
	case "MX":
		return DNSRecordTypeMX, nil
	case "NS":
		return DNSRecordTypeNS, nil
	case "SRV":
		return DNSRecordTypeSRV, nil
	default:
		return DNSRecordTypeUnknown, ErrInvalidInput
	}
}

// DNSZone is a domain hosted at a provider.
type DNSZone struct {
	ID       string
	Provider string
	Name     string
}

// DNSRecord is a single record inside a zone.
type DNSRecord struct {
	ID       string
	ZoneID   string
	Name     string
	Type     DNSRecordType
	Value    string
	TTL      int
	Priority int // MX/SRV only
}
