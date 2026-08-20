package domain

// The types below model ArvanCloud's "Lists" capability (issue #64), a
// reusable, account-scoped collection of values that other CDN capabilities
// (firewall, WAF, DDoS protection, rate limiting — AC5-AC8) reference by ID
// from their own filter/source fields. Confirmed against
// docs/api-specs/arvancloud-cdn-4.0.yml's "List" tag (the lists.*
// operationIds) and the DynamicField/DynamicFieldValue schemas.
//
// This is the direct conceptual analog of Parspack's Bulklist
// (cdn_bulklist.go) — same idea, different vendor name — but it is
// deliberately not called ArvanCloudBulklist: the CDN API itself calls the
// resource a "List", backed by a path named "dynamic-fields", so naming it
// ArvanCloudDynamicField keeps the type traceable back to the spec rather
// than inventing vocabulary ArvanCloud doesn't use (issue #64's scope note).

// ArvanCloudDynamicFieldType is the kind of value a list holds, confirmed
// against the DynamicField schema's "type" enum. It determines how every
// item in the list's Values is interpreted, and is required (and, per the
// spec, effectively fixed) once a list is created.
type ArvanCloudDynamicFieldType string

const (
	ArvanCloudDynamicFieldTypeIP     ArvanCloudDynamicFieldType = "ip"
	ArvanCloudDynamicFieldTypeNumber ArvanCloudDynamicFieldType = "number"
	ArvanCloudDynamicFieldTypeByte   ArvanCloudDynamicFieldType = "byte"
)

var arvanCloudDynamicFieldTypes = []string{
	string(ArvanCloudDynamicFieldTypeIP),
	string(ArvanCloudDynamicFieldTypeNumber),
	string(ArvanCloudDynamicFieldTypeByte),
}

// ValidArvanCloudDynamicFieldType reports whether s is one of the list value
// types the CDN API accepts.
//
// Note on a spec inconsistency: the list-filter query parameter
// (DynamicFieldType, used by lists.index's "type" filter) declares the enum
// [ip, bytes, number] — "bytes" plural, where every body schema
// (DynamicField, DynamicFieldUpdateRequest) uses "byte" singular. That looks
// like a documentation slip rather than a second real type, since a list's
// own Type field can only ever be the body enum's value. This validator, and
// every use of ArvanCloudDynamicFieldType in this port, uses "byte"
// (singular) accordingly, including when building a list-filter request.
func ValidArvanCloudDynamicFieldType(s string) bool {
	return contains(arvanCloudDynamicFieldTypes, s)
}

// ArvanCloudDynamicFieldScope reports who owns a list: "private" ones belong
// to the caller's account, "public" ones are provided by ArvanCloud itself
// (e.g. well-known bad-IP lists) and cannot be created, updated or deleted
// by a caller. The spec marks "scope" read-only on DynamicField: it is
// reported back, never accepted on create/update.
type ArvanCloudDynamicFieldScope string

const (
	ArvanCloudDynamicFieldScopePublic  ArvanCloudDynamicFieldScope = "public"
	ArvanCloudDynamicFieldScopePrivate ArvanCloudDynamicFieldScope = "private"
)

var arvanCloudDynamicFieldScopes = []string{
	string(ArvanCloudDynamicFieldScopePublic),
	string(ArvanCloudDynamicFieldScopePrivate),
}

// ValidArvanCloudDynamicFieldScope reports whether s is one of the scopes
// the CDN API reports, useful for validating the optional scope filter on
// ListArvanCloudDynamicFields.
func ValidArvanCloudDynamicFieldScope(s string) bool {
	return contains(arvanCloudDynamicFieldScopes, s)
}

// ArvanCloudDynamicFieldValue is one item in a list. Confirmed against the
// DynamicFieldValue schema: {id, value, desc, created_at}.
type ArvanCloudDynamicFieldValue struct {
	// ID is the item's provider-assigned UUID. This is what
	// RemoveArvanCloudDynamicFieldItem's item_id identifies (confirmed
	// against the destroy endpoint's item_id parameter: {type: string,
	// format: uuid}) — not an index into Values and not the value itself.
	// Empty when submitting a new item on AddArvanCloudDynamicFieldItems,
	// since the provider assigns it.
	ID string

	// Value is the item's actual value: an IP string, a number, or a
	// base64-encoded byte string, depending on the parent list's Type. It is
	// typed any because its wire shape depends on that Type (the spec's
	// DynamicFieldType oneOf of an IP string, a byte string, or a JSON
	// number) — a scalar simple enough that this project's tagged-union
	// approach for a multi-field value (ArvanCloudDNSRecordValue, issue #63)
	// would be overkill here.
	Value any

	// Desc is a caller-supplied note about this item, empty when none was
	// given.
	Desc string

	// CreatedAt is the provider-reported timestamp, kept as the string the
	// API returns rather than parsed (same convention as
	// ArvanCloudDomain.CreatedAt).
	CreatedAt string
}

// ArvanCloudDynamicField is a "List": a reusable, account-scoped collection
// of values of one Type, referenced by ID from other CDN capabilities'
// filter/source fields. Confirmed against the DynamicField schema.
type ArvanCloudDynamicField struct {
	// ID is the list's provider-assigned UUID.
	ID string

	// Name is the caller-chosen label for the list. Required on create.
	// There is no field to change it afterward: the update endpoint
	// (DynamicFieldUpdateRequest) only accepts Description and Type.
	Name string

	// Description is a caller-supplied note, empty when none was given.
	// Settable on both create and update.
	Description string

	// Namespace is a provider-assigned grouping key, read-only.
	Namespace string

	// Type is the kind of value every item in Values holds. Required on
	// create. The update endpoint also requires Type in its request body
	// despite the CDN API giving no documented way to actually change a
	// list's value type after creation — see
	// UpdateArvanCloudDynamicField's doc comment.
	Type ArvanCloudDynamicFieldType

	// Scope reports who owns the list: "private" (the caller's account) or
	// "public" (provided by ArvanCloud). Read-only.
	Scope ArvanCloudDynamicFieldScope

	// Values are the list's items.
	Values []ArvanCloudDynamicFieldValue

	// AllowedPlans are the pricing plan levels permitted to use this list —
	// read-only informational data, not a setting.
	AllowedPlans []int

	// CreatedAt and UpdatedAt are provider-reported timestamps, kept as the
	// RFC 3339 strings the API returns.
	CreatedAt string
	UpdatedAt string
}
