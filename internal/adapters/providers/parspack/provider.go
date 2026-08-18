package parspack

// This package implements ports.ParspackProvider. Each method is responsible
// for exactly two things: calling the right endpoint, and translating the
// provider's payload into the domain types. Provider-specific JSON shapes stay
// in this package — nothing above the adapter boundary should ever see them.
//
// The methods are grouped by the capability and the API surface they belong to
// (AGENTS.md 4.5), one file each:
//
//	client.go      shared transport, auth and error mapping for all surfaces
//	vms.go         VM lifecycle, cloud-server surface (issue #9)
//	keys.go        SSH keys, cloud-server surface (issue #10)
//	firewalls.go   firewalls, cloud-server surface (issue #11)
//	ssl.go         certificate ordering, SSL surface (issue #18)
//	cdn.go         CDN zones and their DNS records, CDN surface (issue #19)
//
// With cdn.go in place no port method is a stub any more, so this file holds
// no code — only the map above.
