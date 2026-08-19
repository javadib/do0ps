package domain

// The types below model the ModSec tag (Web Application Firewall custom
// rules) of Parspack's CDN API, confirmed against
// docs/api-specs/parspack-cdn.openapi.yaml lines 6677-8223. ModSec is scoped
// to a CDN zone, same as DNS records (AGENTS.md 4.1): a zone selects some mix
// of provider-managed "standard" rule sets and its own "custom" rules, and
// custom rules can reference reusable "data" values (e.g. shared
// regex/allow-list content). This is part of issue #24's CDN-capabilities-
// beyond-Zone/DNS scope.

// CDNModSecRuleSetItem is one entry of a zone's ModSec rule-set selection —
// either a provider-managed "standard" rule set or one of the zone's own
// "custom" rules — with whether it is currently applied to the zone.
// Confirmed against GET .../firewalls/mod-security ("standards"/"customs").
type CDNModSecRuleSetItem struct {
	ID       string
	Name     string
	Selected bool
}

// CDNModSecStatus is the ModSec (WAF) rule-set selection state of a zone:
// which standard rule sets Parspack offers and which of the zone's custom
// rules exist, each flagged with whether it is currently selected/enabled.
// This is not a simple enabled/disabled flag — Parspack models it as a
// per-rule-set selection. Confirmed against GET/POST
// .../firewalls/mod-security (indexModSecRules / setZoneModSecRules).
type CDNModSecStatus struct {
	Standards []CDNModSecRuleSetItem
	Customs   []CDNModSecRuleSetItem
}

// CDNModSecData is a reusable value a custom ModSec rule can reference (e.g.
// a shared regex/allow-list payload), scoped to a zone. Value is the raw
// (already base64-decoded) content — the wire format base64-encodes it in
// both directions, but nothing above the adapter boundary deals with that.
// List responses (dataIndex) do not include Value, only ID and Name; Value
// is only populated by GetCDNModSecData (dataShow). Confirmed against
// .../firewalls/mod-security/data and .../data/{id} (dataIndex / dataStore /
// dataShow / dataUpdate / dataDelete).
type CDNModSecData struct {
	ID    string
	Name  string
	Value string
}

// CDNModSecRule is one of a zone's own custom ModSec (WAF) rules. RuleValue
// is the raw (already base64-decoded) rule body. ModSecDataIDs is what
// create/update accept — the CDNModSecData entries the rule should reference,
// by ID; ModSecData is what GetCDNModSecRule returns instead — those entries
// expanded with their name and value. Confirmed against
// .../firewalls/mod-security/rules and .../rules/{mod_sec_rule}
// (indexModSecCustomRule / storeModSecCustomRule / showModSecCustomRule /
// updateModSecCustomRule / deleteModSecCustomRule). Two asymmetries the spec
// itself documents: the list endpoint reports Name and Status but not
// RuleValue or ModSecData; the show (get) endpoint reports RuleValue and the
// expanded ModSecData but not Name or Status. A single CDNModSecRule value
// is never fully populated by one provider call.
type CDNModSecRule struct {
	ID            string
	Name          string // populated by list, not by get
	Status        string // e.g. "verified"; populated by list, not by get
	RuleValue     string
	ModSecDataIDs []string        // input to create/update
	ModSecData    []CDNModSecData // output of get, expanded from ModSecDataIDs
}
