package domain

// ArvanCloudAccelerationSettings models ArvanCloud CDN's Acceleration schema
// (docs/api-specs/arvancloud-cdn-4.0.yml's Acceleration/AccelerationUpdate
// schemas, confirmed at line ~8796).
//
// Kept in its own file rather than folded into arvancloud_rules.go because it
// is shared by two unrelated resources on two different issues:
//
//   - AC11 (issue #71, arvancloud_rules.go): PageRule.acceleration and
//     PageRuleDiff.acceleration both embed this exact shape.
//   - AC12 (issue #72, not yet implemented as of AC11 landing): the
//     standalone GET/PATCH /domains/{domain}/acceleration endpoint
//     (acceleration.show / acceleration.update) returns and accepts this
//     same shape as its own top-level resource.
//
// AC11 landed first, so this type is defined here. Whoever implements AC12
// next should import and reuse ArvanCloudAccelerationSettings rather than
// redefining it — if both issues end up touching this file/type in the same
// window, whichever PR merges second is responsible for reconciling.
type ArvanCloudAccelerationSettings struct {
	// Status is the domain/page-rule-level acceleration switch. Must be one
	// of ValidArvanCloudAccelerationStatus's values. "inherit" defers to the
	// parent scope's own setting (meaningful on PageRule.acceleration/
	// PageRuleDiff.acceleration; the standalone acceleration.update endpoint
	// only accepts "on"/"off" per AccelerationUpdate's narrower override of
	// this same field).
	Status ArvanCloudAccelerationStatus
	// Extensions is which file extensions acceleration applies to, e.g.
	// ["css", "js"]. Each entry must be one of
	// ValidArvanCloudAccelerationExtension's values. Empty means none.
	Extensions []ArvanCloudAccelerationExtension
}

// ArvanCloudAccelerationStatus is ArvanCloudAccelerationSettings.Status's
// enum (Acceleration.status).
type ArvanCloudAccelerationStatus string

const (
	ArvanCloudAccelerationInherit ArvanCloudAccelerationStatus = "inherit"
	ArvanCloudAccelerationOn      ArvanCloudAccelerationStatus = "on"
	ArvanCloudAccelerationOff     ArvanCloudAccelerationStatus = "off"
)

var arvanCloudAccelerationStatuses = []string{
	string(ArvanCloudAccelerationInherit),
	string(ArvanCloudAccelerationOn),
	string(ArvanCloudAccelerationOff),
}

// ValidArvanCloudAccelerationStatus reports whether s is one of
// Acceleration.status's three values.
func ValidArvanCloudAccelerationStatus(s string) bool {
	return contains(arvanCloudAccelerationStatuses, s)
}

// ArvanCloudAccelerationExtension is one entry of
// ArvanCloudAccelerationSettings.Extensions (Acceleration.extensions' item
// enum).
type ArvanCloudAccelerationExtension string

const (
	ArvanCloudAccelerationExtensionCSS  ArvanCloudAccelerationExtension = "css"
	ArvanCloudAccelerationExtensionGIF  ArvanCloudAccelerationExtension = "gif"
	ArvanCloudAccelerationExtensionJPEG ArvanCloudAccelerationExtension = "jpeg"
	ArvanCloudAccelerationExtensionJS   ArvanCloudAccelerationExtension = "js"
	ArvanCloudAccelerationExtensionPNG  ArvanCloudAccelerationExtension = "png"
)

var arvanCloudAccelerationExtensions = []string{
	string(ArvanCloudAccelerationExtensionCSS),
	string(ArvanCloudAccelerationExtensionGIF),
	string(ArvanCloudAccelerationExtensionJPEG),
	string(ArvanCloudAccelerationExtensionJS),
	string(ArvanCloudAccelerationExtensionPNG),
}

// ValidArvanCloudAccelerationExtension reports whether s is one of
// Acceleration.extensions' five item values.
func ValidArvanCloudAccelerationExtension(s string) bool {
	return contains(arvanCloudAccelerationExtensions, s)
}
