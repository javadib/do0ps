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

// Server is a compute instance at a provider. Field set and naming are
// confirmed against the "vm" resource of github.com/abrhacom/terraform-
// provider-abrha, the Go-API-Abrha-compatible platform Parspack's
// cloud-server API is built on (see AGENTS.md 4.5).
type Server struct {
	ID       string
	URN      string // uniform resource name reported by the provider
	Provider string
	Name     string
	Region   string
	Image    string
	PlanID   string // provider size slug, e.g. "s-1vcpu-1gb"; maps to the confirmed "size" attribute
	Status   ServerStatus
	CPUCores int
	RAMMB    int
	DiskGB   int

	IPv4        string
	IPv4Private string // private networking address, empty when PrivateNetworking is false
	IPv6        string

	Locked            bool
	PrivateNetworking bool
	Backups           bool
	VPCUUID           string // empty when the server is not in a VPC

	PriceHourly  float64
	PriceMonthly float64

	CreatedAt time.Time
}

// BackupPolicy configures the automatic backup schedule requested at server
// creation. Only meaningful when ServerSpec.Backups is true.
type BackupPolicy struct {
	Plan     string // "daily", "weekly", or "monthly"
	Weekday  string // "SUN".."SAT"; weekly plan only
	Monthday int    // 1-28; monthly plan only
}

// ServerSpec is the normalized request for a new compute instance.
type ServerSpec struct {
	Name     string
	Region   string
	Image    string // OS image slug, or the ID of a VMSnapshot to restore from
	PlanID   string // provider size slug when the caller names one directly
	CPUCores int
	RAMMB    int
	DiskGB   int

	Backups      bool
	BackupPolicy *BackupPolicy
	EnableIPv6   bool
	VPCUUID      string
	SSHKeys      []string // registered SSH key IDs or fingerprints to install
	UserData     string   // cloud-init script run on first boot
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

// SSHKey is a public key registered with the provider so it can be installed
// on new servers at create time (see ServerSpec.SSHKeys). Field set confirmed
// against the "ssh_key" resource of terraform-provider-abrha.
type SSHKey struct {
	ID          string
	Name        string
	PublicKey   string
	Fingerprint string // computed by the provider, read-only
}

// FirewallRule is one inbound or outbound access rule of a Firewall.
type FirewallRule struct {
	Protocol string // "tcp", "udp", or "icmp"
	// PortRange is a single port, a range ("8000-9000"), or "1-65535" for all
	// ports. Required when Protocol is "tcp" or "udp", ignored for "icmp".
	PortRange string
	// Addresses holds source addresses for an inbound rule or destination
	// addresses for an outbound rule, as IPv4 addresses or CIDRs.
	Addresses []string
}

// Firewall is a rules-based network firewall applied to a set of servers.
// Field set confirmed against the "firewall" resource of
// terraform-provider-abrha. Distinct from the CDN-level firewall tag of the
// Parspack CDN API (AGENTS.md 4.1) — this one is cloud-server/VM-network
// level.
type Firewall struct {
	ID            string
	Name          string
	Status        string // "waiting", "succeeded", or "failed"
	ServerIDs     []string
	InboundRules  []FirewallRule
	OutboundRules []FirewallRule
	CreatedAt     time.Time
}

// LoadBalancerHealthCheck configures how a LoadBalancer probes its backend
// servers.
type LoadBalancerHealthCheck struct {
	Protocol               string
	Port                   int
	Path                   string
	CheckIntervalSeconds   int
	ResponseTimeoutSeconds int
	UnhealthyThreshold     int
	HealthyThreshold       int
}

// ForwardingRule maps a LoadBalancer's public-facing port to a port on each
// backend server.
type ForwardingRule struct {
	EntryProtocol  string
	EntryPort      int
	TargetProtocol string
	TargetPort     int
}

// LoadBalancer distributes traffic across a set of backend servers. Field set
// confirmed (via github.com/abrhacom/go-api-abrha, the Go REST client
// terraform-provider-abrha depends on — its own load balancer docs page is
// currently empty) against the "loadbalancer" resource of
// terraform-provider-abrha. Distinct from the CDN-level load balance tag of
// the Parspack CDN API (AGENTS.md 4.1) — this one is cloud-server/VM-network
// level.
type LoadBalancer struct {
	ID                string
	Name              string
	Algorithm         string
	Region            string
	IP                string
	Status            string
	ForwardingRules   []ForwardingRule
	HealthCheck       *LoadBalancerHealthCheck
	ServerIDs         []string
	RedirectHTTPToTLS bool
	VPCUUID           string
	CreatedAt         time.Time
}

// ReservedIP is a static public IPv4 address that exists independently of any
// server and can be reassigned between them. Mirrors the two-resource split
// of terraform-provider-abrha's "reserved_ip" and "reserved_ip_assignment":
// ServerID is empty while the IP is reserved but unassigned.
type ReservedIP struct {
	IPAddress string
	Region    string
	ServerID  string
	URN       string
}

// VPC is an isolated private network that servers and load balancers can
// optionally be placed into (see ServerSpec.VPCUUID). Field set confirmed
// against the "vpc" resource of terraform-provider-abrha.
type VPC struct {
	ID          string
	Name        string
	Region      string
	Description string
	IPRange     string
	Default     bool // whether this is the account's default VPC for Region
	CreatedAt   time.Time
}

// VMSnapshot is a point-in-time image of a server's disk. Its ID can be used
// as the Image value of a new ServerSpec to restore from it. Field set
// confirmed against the "vm_snapshot" resource of terraform-provider-abrha.
type VMSnapshot struct {
	ID        string
	Name      string
	ServerID  string
	Regions   []string // region slugs where the snapshot is available
	MinDiskGB int      // minimum disk size a server created from this snapshot needs
	SizeGB    int      // billable size of the snapshot
	CreatedAt time.Time
}

// VMAction is an asynchronous operation a server's disk is running, e.g.
// snapshotting or restoring from a snapshot. Parspack reports these through
// the VM actions endpoints (github.com/abrhacom/go-api-abrha/vm_actions.go),
// and snapshot creation/restore both complete behind one.
type VMAction struct {
	ID          string
	ServerID    string // the VM the action runs on
	Type        string // provider verb, e.g. "snapshot" or "restore"
	Status      string // "in-progress", "completed", or "errored"
	StartedAt   time.Time
	CompletedAt time.Time
}

// VMAction statuses reported by Abrha-based APIs.
const (
	VMActionStatusInProgress = "in-progress"
	VMActionStatusCompleted  = "completed"
	VMActionStatusErrored    = "errored"
)
