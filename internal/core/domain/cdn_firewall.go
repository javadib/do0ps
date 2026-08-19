package domain

// The types below model the CDN-edge "Firewall" tag of Parspack's CDN API
// surface (AGENTS.md 4.5, issue #24): access-management rules, IP-reputation
// blocking and DDoS mitigation actions, all scoped to a CDN zone. Field sets
// are confirmed against docs/api-specs/parspack-cdn.openapi.yaml lines
// 3634-5027.
//
// These are entirely distinct from domain.Firewall (issue #11): that type is
// the cloud-server/VM-network-level security-group firewall, unrelated to
// the CDN edge. Every type here is prefixed CDN to keep the two apart.

// CDNAccessRule is an access-management rule at the CDN edge for one zone:
// it matches traffic on Type+Value (or, when BulklistID is set, on every
// value inside a managed bulk list instead of a single Value) and applies
// Action to matches. Confirmed against
// .../firewalls/access-management(/{access_management_id}).
type CDNAccessRule struct {
	ID       string
	ZoneUUID string
	Type     string // "ip", "user_agent", "country", "zone", "referral"
	Value    string // required unless BulklistID is set; for type "country" this is the country id
	Action   string // "dynamic", "block", "captcha", "allow", "bypass"
	Status   bool
	Priority int // assigned by the provider; not settable via create/update

	BulklistID   string // optional; when set, Value is ignored and the rule matches every value in the referenced bulk list
	BulklistName string // populated on read when the rule references a bulk list; ignored on write
}

// cdnAccessRuleTypes and cdnAccessRuleActions are the enums confirmed
// against the access-management create/update request bodies.
var (
	cdnAccessRuleTypes   = []string{"ip", "user_agent", "country", "zone", "referral"}
	cdnAccessRuleActions = []string{"dynamic", "block", "captcha", "allow", "bypass"}
)

// ValidCDNAccessRuleType reports whether s is one of the match types the CDN
// access-management API accepts.
func ValidCDNAccessRuleType(s string) bool { return contains(cdnAccessRuleTypes, s) }

// ValidCDNAccessRuleAction reports whether s is one of the actions the CDN
// access-management API accepts.
func ValidCDNAccessRuleAction(s string) bool { return contains(cdnAccessRuleActions, s) }

// CDNIPReputationSettings controls IP-reputation-based blocking for one CDN
// zone. Confirmed against GET/PUT .../firewalls/ip-reputation; the provider
// requires every field on update.
type CDNIPReputationSettings struct {
	Enabled       bool
	TrustTime     int    // seconds a challenged IP is trusted after passing, e.g. 3600
	TreatScore    string // sensitivity threshold: "very-low", "low", "medium", "high", "very-high"
	Challenge     string // action applied to flagged IPs: "js", "block", "recaptcha"
	AttackBanTime int    // seconds an IP that trips the threshold is banned for
}

// cdnIPReputationTreatScores and cdnIPReputationChallenges are the enums
// confirmed against the ip-reputation update request body.
var (
	cdnIPReputationTreatScores = []string{"very-low", "low", "medium", "high", "very-high"}
	cdnIPReputationChallenges  = []string{"js", "block", "recaptcha"}
)

// ValidCDNIPReputationTreatScore reports whether s is one of the sensitivity
// thresholds the CDN ip-reputation API accepts.
func ValidCDNIPReputationTreatScore(s string) bool { return contains(cdnIPReputationTreatScores, s) }

// ValidCDNIPReputationChallenge reports whether s is one of the challenge
// actions the CDN ip-reputation API accepts.
func ValidCDNIPReputationChallenge(s string) bool { return contains(cdnIPReputationChallenges, s) }

// CDNDDoSActionSettings controls the CDN's automatic DDoS mitigation action
// for one zone. Confirmed against GET/PUT .../ddos-actions; TrustTime and
// BanTime are optional on update (provider defaults are 3600 and 900
// respectively when omitted).
type CDNDDoSActionSettings struct {
	Action    string // "none", "js", "recaptcha", "block"
	TrustTime int    // seconds, 0-86400, default 3600
	BanTime   int    // seconds, 0-86400, default 900
}

// cdnDDoSActions is the enum confirmed against the ddos-actions update
// request body.
var cdnDDoSActions = []string{"none", "js", "recaptcha", "block"}

// ValidCDNDDoSAction reports whether s is one of the actions the CDN
// ddos-actions API accepts.
func ValidCDNDDoSAction(s string) bool { return contains(cdnDDoSActions, s) }
