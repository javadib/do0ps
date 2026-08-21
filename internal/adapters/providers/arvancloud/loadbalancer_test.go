package arvancloud

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

// --- Regions ----------------------------------------------------------------

// TestListArvanCloudLBRegions pins the request shape and response parsing of
// GET /load-balancers/regions, the account-independent endpoint (no
// domainName in the path).
func TestListArvanCloudLBRegions(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[{"id":"region-1","region":"LAH","name":"Lahijan"}]}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	regions, err := provider.ListArvanCloudLBRegions(context.Background(), creds())
	if err != nil {
		t.Fatalf("ListArvanCloudLBRegions() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/load-balancers/regions" {
		t.Fatalf("request = %+v, want a single GET /load-balancers/regions", records)
	}
	if len(regions) != 1 || regions[0].Region != "LAH" || regions[0].Name != "Lahijan" {
		t.Errorf("regions = %+v, want the parsed region", regions)
	}
}

// TestListArvanCloudDomainLBRegions pins the request shape of GET
// /domains/{domain}/load-balancers/regions, the per-domain equivalent.
func TestListArvanCloudDomainLBRegions(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[{"id":"region-1","region":"THR","name":"Tehran"}]}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	regions, err := provider.ListArvanCloudDomainLBRegions(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("ListArvanCloudDomainLBRegions() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/load-balancers/regions" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/load-balancers/regions", records)
	}
	if len(regions) != 1 || regions[0].Region != "THR" {
		t.Errorf("regions = %+v, want the parsed region", regions)
	}
}

// --- Settings -----------------------------------------------------------

// TestGetArvanCloudLBSettings pins the request shape and response parsing of
// GET /domains/{domain}/load-balancers/settings.
func TestGetArvanCloudLBSettings(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"method":"cluster_rr","next_upstream_tcp":"on","protocol":"https","grpc_status":true,"keepalive":"on","max_fails":3,"fail_timeout":"45s"}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	settings, err := provider.GetArvanCloudLBSettings(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("GetArvanCloudLBSettings() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/load-balancers/settings" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/load-balancers/settings", records)
	}
	if settings.Method != domain.ArvanCloudPoolMethodClusterRR || !settings.GRPCStatus || settings.MaxFails != 3 {
		t.Errorf("settings = %+v, want the parsed settings", settings)
	}
}

// TestUpdateArvanCloudLBSettings pins the request body of PATCH
// /domains/{domain}/load-balancers/settings, including that grpc_status is
// sent explicitly even when false.
func TestUpdateArvanCloudLBSettings(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"method":"cluster_chash","grpc_status":false,"keepalive":"off"}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	settings := domain.ArvanCloudLoadBalancerSettings{
		Method:     domain.ArvanCloudPoolMethodClusterCHash,
		GRPCStatus: false,
		Keepalive:  domain.ArvanCloudLBOff,
	}
	updated, err := provider.UpdateArvanCloudLBSettings(context.Background(), creds(), "example.com", settings)
	if err != nil {
		t.Fatalf("UpdateArvanCloudLBSettings() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPatch || records[0].path != "/domains/example.com/load-balancers/settings" {
		t.Fatalf("request = %+v, want a single PATCH /domains/example.com/load-balancers/settings", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	grpcStatus, hasGRPCStatus := body["grpc_status"]
	if !hasGRPCStatus || grpcStatus != false {
		t.Errorf(`request body["grpc_status"] = %#v, hasKey=%v, want an explicit false`, grpcStatus, hasGRPCStatus)
	}
	if updated.Method != domain.ArvanCloudPoolMethodClusterCHash {
		t.Errorf("updated.Method = %q, want %q", updated.Method, domain.ArvanCloudPoolMethodClusterCHash)
	}
}

// --- Load balancers -------------------------------------------------------

// TestListArvanCloudLoadBalancers pins the request shape and response
// parsing of GET /domains/{domain}/load-balancers.
func TestListArvanCloudLoadBalancers(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[{"id":"lb-1","name":"lb1","status":true,"method":"failover"}]}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	lbs, err := provider.ListArvanCloudLoadBalancers(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("ListArvanCloudLoadBalancers() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/load-balancers" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/load-balancers", records)
	}
	if len(lbs) != 1 || lbs[0].ID != "lb-1" || lbs[0].Method != domain.ArvanCloudLoadBalancerMethodFailover {
		t.Errorf("lbs = %+v, want the parsed load balancer", lbs)
	}
}

// TestCreateArvanCloudLoadBalancer pins the request body of POST
// /domains/{domain}/load-balancers.
func TestCreateArvanCloudLoadBalancer(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"lb-1","name":"lb1","status":true,"method":"cluster_rr","time_slice":"30s"}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	lb := domain.ArvanCloudLoadBalancer{
		Name:      "lb1",
		Status:    true,
		Method:    domain.ArvanCloudLoadBalancerMethodClusterRR,
		TimeSlice: "30s",
	}
	created, err := provider.CreateArvanCloudLoadBalancer(context.Background(), creds(), "example.com", lb)
	if err != nil {
		t.Fatalf("CreateArvanCloudLoadBalancer() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/load-balancers" {
		t.Fatalf("request = %+v, want a single POST /domains/example.com/load-balancers", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["name"] != "lb1" || body["method"] != "cluster_rr" || body["time_slice"] != "30s" {
		t.Errorf("request body = %+v, want name/method/time_slice sent", body)
	}
	if created.ID != "lb-1" {
		t.Errorf("created.ID = %q, want %q", created.ID, "lb-1")
	}
}

// TestGetArvanCloudLoadBalancer pins the request shape and response parsing
// of GET /domains/{domain}/load-balancers/{id}, including a nested pool.
func TestGetArvanCloudLoadBalancer(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"lb-1","name":"lb1","status":true,"method":"failover","pools":[
			{"id":"pool-1","name":"pool1","status":true,"method":"cluster_rr","priority":1,
			 "regions":[{"id":"r1","region":"LAH","name":"Lahijan"}],
			 "origins":[{"id":"origin-1","address":"203.0.113.10","port":443,"weight":100,"protocol":"https","status":true}]}
		]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	lb, err := provider.GetArvanCloudLoadBalancer(context.Background(), creds(), "example.com", "lb-1")
	if err != nil {
		t.Fatalf("GetArvanCloudLoadBalancer() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/load-balancers/lb-1" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/load-balancers/lb-1", records)
	}
	if len(lb.Pools) != 1 || lb.Pools[0].ID != "pool-1" {
		t.Fatalf("lb.Pools = %+v, want one parsed pool", lb.Pools)
	}
	if len(lb.Pools[0].Regions) != 1 || lb.Pools[0].Regions[0] != "LAH" {
		t.Errorf("lb.Pools[0].Regions = %+v, want [\"LAH\"] (region code, not the full region object)", lb.Pools[0].Regions)
	}
	if len(lb.Pools[0].Origins) != 1 || lb.Pools[0].Origins[0].Address != "203.0.113.10" {
		t.Errorf("lb.Pools[0].Origins = %+v, want one parsed origin", lb.Pools[0].Origins)
	}
}

// TestUpdateArvanCloudLoadBalancer pins the request body of PATCH
// /domains/{domain}/load-balancers/{id}.
func TestUpdateArvanCloudLoadBalancer(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"lb-1","name":"lb1","status":false,"method":"cluster_chash"}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	lb := domain.ArvanCloudLoadBalancer{Name: "lb1", Status: false, Method: domain.ArvanCloudLoadBalancerMethodClusterCHash}
	updated, err := provider.UpdateArvanCloudLoadBalancer(context.Background(), creds(), "example.com", "lb-1", lb)
	if err != nil {
		t.Fatalf("UpdateArvanCloudLoadBalancer() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPatch || records[0].path != "/domains/example.com/load-balancers/lb-1" {
		t.Fatalf("request = %+v, want a single PATCH /domains/example.com/load-balancers/lb-1", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	status, hasStatus := body["status"]
	if !hasStatus || status != false {
		t.Errorf(`request body["status"] = %#v, hasKey=%v, want an explicit false`, status, hasStatus)
	}
	if updated.Status || updated.Method != domain.ArvanCloudLoadBalancerMethodClusterCHash {
		t.Errorf("updated = %+v, want the parsed load balancer", updated)
	}
}

// TestDeleteArvanCloudLoadBalancer pins the request shape of DELETE
// /domains/{domain}/load-balancers/{id}.
func TestDeleteArvanCloudLoadBalancer(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte { return []byte(`{"message":"Deleted successfully"}`) }, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.DeleteArvanCloudLoadBalancer(context.Background(), creds(), "example.com", "lb-1"); err != nil {
		t.Fatalf("DeleteArvanCloudLoadBalancer() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodDelete || records[0].path != "/domains/example.com/load-balancers/lb-1" {
		t.Fatalf("request = %+v, want a single DELETE /domains/example.com/load-balancers/lb-1", records)
	}
}

// TestDeleteArvanCloudLoadBalancerNotFound proves a 404 surfaces as
// domain.ErrNotFound, consistent with the tolerant-delete contract at the
// use-case layer (app.DeleteArvanCloudLoadBalancer).
func TestDeleteArvanCloudLoadBalancerNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not found"}`))
	}))
	defer srv.Close()

	provider := newTestProvider(t, srv)
	err := provider.DeleteArvanCloudLoadBalancer(context.Background(), creds(), "example.com", "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("DeleteArvanCloudLoadBalancer() error = %v, want domain.ErrNotFound", err)
	}
}

// --- Pools ------------------------------------------------------------------

// TestListArvanCloudLBPools pins the request shape and response parsing of
// GET .../load-balancers/{id}/pools.
func TestListArvanCloudLBPools(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[{"id":"pool-1","name":"pool1","status":true,"method":"cluster_rr"}]}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	pools, err := provider.ListArvanCloudLBPools(context.Background(), creds(), "example.com", "lb-1")
	if err != nil {
		t.Fatalf("ListArvanCloudLBPools() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/load-balancers/lb-1/pools" {
		t.Fatalf("request = %+v, want a single GET .../load-balancers/lb-1/pools", records)
	}
	if len(pools) != 1 || pools[0].ID != "pool-1" {
		t.Errorf("pools = %+v, want the parsed pool", pools)
	}
}

// TestCreateArvanCloudLBPool pins the request body of POST
// .../load-balancers/{id}/pools, including a nested origin.
func TestCreateArvanCloudLBPool(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"pool-1","name":"pool1","status":true,"method":"cluster_rr",
			"origins":[{"id":"origin-1","address":"203.0.113.10","port":443,"weight":100,"protocol":"https","status":true}]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	pool := domain.ArvanCloudLoadBalancerPool{
		Name:            "pool1",
		Status:          true,
		Method:          domain.ArvanCloudPoolMethodClusterRR,
		Keepalive:       domain.ArvanCloudLBOn,
		NextUpstreamTCP: domain.ArvanCloudLBOff,
		Regions:         []string{"LAH"},
		Origins: []domain.ArvanCloudLoadBalancerOrigin{
			{Address: "203.0.113.10", Port: 443, Weight: 100, Protocol: domain.ArvanCloudOriginProtocolHTTPS, Status: true},
		},
	}
	created, err := provider.CreateArvanCloudLBPool(context.Background(), creds(), "example.com", "lb-1", pool)
	if err != nil {
		t.Fatalf("CreateArvanCloudLBPool() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/load-balancers/lb-1/pools" {
		t.Fatalf("request = %+v, want a single POST .../load-balancers/lb-1/pools", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["name"] != "pool1" || body["method"] != "cluster_rr" {
		t.Errorf("request body = %+v, want name/method sent", body)
	}
	regions, ok := body["regions"].([]any)
	if !ok || len(regions) != 1 || regions[0] != "LAH" {
		t.Errorf("request body[regions] = %+v, want [\"LAH\"] (plain codes, not region objects)", body["regions"])
	}
	origins, ok := body["origins"].([]any)
	if !ok || len(origins) != 1 {
		t.Fatalf("request body[origins] = %+v, want one origin", body["origins"])
	}
	originBody, ok := origins[0].(map[string]any)
	if !ok || originBody["address"] != "203.0.113.10" {
		t.Errorf("request body[origins][0] = %+v, want the origin sent", origins[0])
	}
	if len(created.Origins) != 1 || created.Origins[0].ID != "origin-1" {
		t.Errorf("created.Origins = %+v, want the parsed origin", created.Origins)
	}
}

// TestGetArvanCloudLBPool pins the request shape and response parsing of GET
// .../pools/{id}.
func TestGetArvanCloudLBPool(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"pool-1","name":"pool1","status":true,"method":"cluster_rr","monitoring_status":"healthy"}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	pool, err := provider.GetArvanCloudLBPool(context.Background(), creds(), "example.com", "lb-1", "pool-1")
	if err != nil {
		t.Fatalf("GetArvanCloudLBPool() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/load-balancers/lb-1/pools/pool-1" {
		t.Fatalf("request = %+v, want a single GET .../pools/pool-1", records)
	}
	if pool.ID != "pool-1" || pool.MonitoringStatus != "healthy" {
		t.Errorf("pool = %+v, want the parsed pool", pool)
	}
}

// TestDeleteArvanCloudLBPool pins the request shape of DELETE
// .../pools/{id}.
func TestDeleteArvanCloudLBPool(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte { return []byte(`{"message":"Deleted successfully"}`) }, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.DeleteArvanCloudLBPool(context.Background(), creds(), "example.com", "lb-1", "pool-1"); err != nil {
		t.Fatalf("DeleteArvanCloudLBPool() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodDelete || records[0].path != "/domains/example.com/load-balancers/lb-1/pools/pool-1" {
		t.Fatalf("request = %+v, want a single DELETE .../pools/pool-1", records)
	}
}

// TestUpdateArvanCloudLBPoolWithOriginsReplacesOrigins is the acceptance
// criteria's explicit regression test: UpdateArvanCloudLBPoolWithOrigins
// (PUT, load-balancers.pools.update) sends the pool's origins in its request
// body, so the provider replaces the pool's full origin set with whatever is
// given here — proven by asserting the outgoing request body actually
// carries an "origins" array with the expected content.
func TestUpdateArvanCloudLBPoolWithOriginsReplacesOrigins(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"pool-1","name":"pool1","status":true,"method":"cluster_rr",
			"origins":[{"id":"origin-2","address":"203.0.113.20","port":8443,"weight":50,"protocol":"https","status":true}]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	pool := domain.ArvanCloudLoadBalancerPool{
		Name:            "pool1",
		Status:          true,
		Method:          domain.ArvanCloudPoolMethodClusterRR,
		Keepalive:       domain.ArvanCloudLBOn,
		NextUpstreamTCP: domain.ArvanCloudLBOff,
		Origins: []domain.ArvanCloudLoadBalancerOrigin{
			{Address: "203.0.113.20", Port: 8443, Weight: 50, Protocol: domain.ArvanCloudOriginProtocolHTTPS, Status: true},
		},
	}
	updated, err := provider.UpdateArvanCloudLBPoolWithOrigins(context.Background(), creds(), "example.com", "lb-1", "pool-1", pool)
	if err != nil {
		t.Fatalf("UpdateArvanCloudLBPoolWithOrigins() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPut || records[0].path != "/domains/example.com/load-balancers/lb-1/pools/pool-1" {
		t.Fatalf("request = %+v, want a single PUT .../pools/pool-1", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	origins, ok := body["origins"].([]any)
	if !ok || len(origins) != 1 {
		t.Fatalf("request body[origins] = %+v, want exactly one origin sent — PUT must replace origins", body["origins"])
	}
	originBody, ok := origins[0].(map[string]any)
	if !ok || originBody["address"] != "203.0.113.20" {
		t.Errorf("request body[origins][0] = %+v, want the new origin sent", origins[0])
	}
	if len(updated.Origins) != 1 || updated.Origins[0].ID != "origin-2" {
		t.Errorf("updated.Origins = %+v, want the replaced origin set echoed back", updated.Origins)
	}
}

// TestUpdateArvanCloudLBPoolSettingsLeavesOriginsUntouched is the
// acceptance criteria's explicit regression test, the counterpart to
// TestUpdateArvanCloudLBPoolWithOriginsReplacesOrigins:
// UpdateArvanCloudLBPoolSettings (PATCH, load-balancers.pools.updatePool)
// must never send an "origins" field at all — proving a settings-only
// change cannot accidentally wipe a pool's existing origins the way PUT
// would. This is issue #69's acceptance criteria's explicitly called-out
// highest-risk regression.
func TestUpdateArvanCloudLBPoolSettingsLeavesOriginsUntouched(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"pool-1","name":"pool1-renamed","status":true,"method":"cluster_rr",
			"origins":[{"id":"origin-1","address":"203.0.113.10","port":443,"weight":100,"protocol":"https","status":true}]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	pool := domain.ArvanCloudLoadBalancerPool{
		Name:            "pool1-renamed",
		Status:          true,
		Method:          domain.ArvanCloudPoolMethodClusterRR,
		Keepalive:       domain.ArvanCloudLBOn,
		NextUpstreamTCP: domain.ArvanCloudLBOff,
		// A caller updating settings-only would naturally leave Origins at
		// its zero value (nil) rather than re-fetching and resending the
		// existing set — that must still not touch the provider's stored
		// origins, hence no "origins" key in the request body at all.
	}
	updated, err := provider.UpdateArvanCloudLBPoolSettings(context.Background(), creds(), "example.com", "lb-1", "pool-1", pool)
	if err != nil {
		t.Fatalf("UpdateArvanCloudLBPoolSettings() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPatch || records[0].path != "/domains/example.com/load-balancers/lb-1/pools/pool-1" {
		t.Fatalf("request = %+v, want a single PATCH .../pools/pool-1", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if _, hasOrigins := body["origins"]; hasOrigins {
		t.Errorf("request body = %+v, must NOT include an \"origins\" key — PATCH must leave origins untouched", body)
	}
	if body["name"] != "pool1-renamed" {
		t.Errorf(`request body["name"] = %#v, want "pool1-renamed"`, body["name"])
	}
	// The response's origins (as the provider actually stored them,
	// untouched by this call) are still decoded normally.
	if len(updated.Origins) != 1 || updated.Origins[0].ID != "origin-1" {
		t.Errorf("updated.Origins = %+v, want the pre-existing origin echoed back untouched", updated.Origins)
	}
}

// TestReprioritizeArvanCloudLBPoolBefore pins the "before" shape of POST
// .../load-balancers/{id}/prioritize (PrioritizePoolBefore:
// pool_id/before_pool_id — NOT the rule_id/after_rule_id/before_rule_id
// shape the other reprioritize endpoints use), and that the response decodes
// as the load balancer resource, not a bare confirmation.
func TestReprioritizeArvanCloudLBPoolBefore(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"lb-1","name":"lb1","status":true,"method":"failover"}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	updated, err := provider.ReprioritizeArvanCloudLBPool(context.Background(), creds(), "example.com", "lb-1", "pool-1", "", "pool-2")
	if err != nil {
		t.Fatalf("ReprioritizeArvanCloudLBPool() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/load-balancers/lb-1/prioritize" {
		t.Fatalf("request = %+v, want a single POST .../load-balancers/lb-1/prioritize", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["pool_id"] != "pool-1" || body["before_pool_id"] != "pool-2" {
		t.Errorf("request body = %+v, want pool_id/before_pool_id set", body)
	}
	if _, hasAfter := body["after_pool_id"]; hasAfter {
		t.Errorf("request body = %+v, must omit after_pool_id when unset", body)
	}
	if _, hasRuleID := body["rule_id"]; hasRuleID {
		t.Errorf("request body = %+v, must use pool_id, not the rule_id shape the other reprioritize endpoints use", body)
	}
	if updated.ID != "lb-1" {
		t.Errorf("updated.ID = %q, want %q (the load balancer resource, not a bare confirmation)", updated.ID, "lb-1")
	}
}

// TestReprioritizeArvanCloudLBPoolAfter pins the "after" shape
// (PrioritizePoolAfter: pool_id/after_pool_id), the counterpart to
// TestReprioritizeArvanCloudLBPoolBefore.
func TestReprioritizeArvanCloudLBPoolAfter(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"lb-1","name":"lb1","status":true,"method":"failover"}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if _, err := provider.ReprioritizeArvanCloudLBPool(context.Background(), creds(), "example.com", "lb-1", "pool-1", "pool-2", ""); err != nil {
		t.Fatalf("ReprioritizeArvanCloudLBPool() error = %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["pool_id"] != "pool-1" || body["after_pool_id"] != "pool-2" {
		t.Errorf("request body = %+v, want pool_id/after_pool_id set", body)
	}
	if _, hasBefore := body["before_pool_id"]; hasBefore {
		t.Errorf("request body = %+v, must omit before_pool_id when unset", body)
	}
}

// --- Origins ------------------------------------------------------------

// TestListArvanCloudLBPoolOrigins pins the request shape and response
// parsing of GET .../pools/{id}/origins.
func TestListArvanCloudLBPoolOrigins(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[{"id":"origin-1","address":"203.0.113.10","port":443,"weight":100,"protocol":"https","status":true,"health_check_status":"healthy"}]}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	origins, err := provider.ListArvanCloudLBPoolOrigins(context.Background(), creds(), "example.com", "lb-1", "pool-1")
	if err != nil {
		t.Fatalf("ListArvanCloudLBPoolOrigins() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/load-balancers/lb-1/pools/pool-1/origins" {
		t.Fatalf("request = %+v, want a single GET .../pools/pool-1/origins", records)
	}
	if len(origins) != 1 || origins[0].HealthCheckStatus != domain.ArvanCloudOriginHealthCheckHealthy {
		t.Errorf("origins = %+v, want the parsed origin", origins)
	}
}

// TestCreateArvanCloudLBPoolOrigin pins the request body of POST
// .../pools/{id}/origins.
func TestCreateArvanCloudLBPoolOrigin(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"origin-1","address":"203.0.113.10","port":443,"weight":100,"protocol":"https","status":true}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	origin := domain.ArvanCloudLoadBalancerOrigin{
		Address:  "203.0.113.10",
		Port:     443,
		Weight:   100,
		Protocol: domain.ArvanCloudOriginProtocolHTTPS,
		Status:   true,
	}
	created, err := provider.CreateArvanCloudLBPoolOrigin(context.Background(), creds(), "example.com", "lb-1", "pool-1", origin)
	if err != nil {
		t.Fatalf("CreateArvanCloudLBPoolOrigin() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/load-balancers/lb-1/pools/pool-1/origins" {
		t.Fatalf("request = %+v, want a single POST .../pools/pool-1/origins", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	status, hasStatus := body["status"]
	if !hasStatus || status != true {
		t.Errorf(`request body["status"] = %#v, hasKey=%v, want an explicit true`, status, hasStatus)
	}
	if body["address"] != "203.0.113.10" || body["port"] != float64(443) || body["weight"] != float64(100) {
		t.Errorf("request body = %+v, want address/port/weight sent", body)
	}
	if created.ID != "origin-1" {
		t.Errorf("created.ID = %q, want %q", created.ID, "origin-1")
	}
}

// TestGetArvanCloudLBPoolOrigin pins the request shape and response parsing
// of GET .../origins/{id}.
func TestGetArvanCloudLBPoolOrigin(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"origin-1","address":"203.0.113.10","port":443,"weight":100,"protocol":"https","status":true}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	origin, err := provider.GetArvanCloudLBPoolOrigin(context.Background(), creds(), "example.com", "lb-1", "pool-1", "origin-1")
	if err != nil {
		t.Fatalf("GetArvanCloudLBPoolOrigin() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/load-balancers/lb-1/pools/pool-1/origins/origin-1" {
		t.Fatalf("request = %+v, want a single GET .../origins/origin-1", records)
	}
	if origin.ID != "origin-1" || origin.Port != 443 {
		t.Errorf("origin = %+v, want the parsed origin", origin)
	}
}

// TestUpdateArvanCloudLBPoolOrigin pins the request body of PATCH
// .../origins/{id}.
func TestUpdateArvanCloudLBPoolOrigin(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"origin-1","address":"203.0.113.11","port":8443,"weight":200,"protocol":"http","status":false}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	origin := domain.ArvanCloudLoadBalancerOrigin{
		Address:  "203.0.113.11",
		Port:     8443,
		Weight:   200,
		Protocol: domain.ArvanCloudOriginProtocolHTTP,
		Status:   false,
	}
	updated, err := provider.UpdateArvanCloudLBPoolOrigin(context.Background(), creds(), "example.com", "lb-1", "pool-1", "origin-1", origin)
	if err != nil {
		t.Fatalf("UpdateArvanCloudLBPoolOrigin() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPatch || records[0].path != "/domains/example.com/load-balancers/lb-1/pools/pool-1/origins/origin-1" {
		t.Fatalf("request = %+v, want a single PATCH .../origins/origin-1", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	status, hasStatus := body["status"]
	if !hasStatus || status != false {
		t.Errorf(`request body["status"] = %#v, hasKey=%v, want an explicit false`, status, hasStatus)
	}
	if updated.Weight != 200 {
		t.Errorf("updated.Weight = %d, want %d", updated.Weight, 200)
	}
}

// TestDeleteArvanCloudLBPoolOrigin pins the request shape of DELETE
// .../origins/{id}.
func TestDeleteArvanCloudLBPoolOrigin(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte { return []byte(`{"message":"Deleted successfully"}`) }, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.DeleteArvanCloudLBPoolOrigin(context.Background(), creds(), "example.com", "lb-1", "pool-1", "origin-1"); err != nil {
		t.Fatalf("DeleteArvanCloudLBPoolOrigin() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodDelete || records[0].path != "/domains/example.com/load-balancers/lb-1/pools/pool-1/origins/origin-1" {
		t.Fatalf("request = %+v, want a single DELETE .../origins/origin-1", records)
	}
}

// TestDeleteArvanCloudLBPoolOriginNotFound proves a 404 surfaces as
// domain.ErrNotFound, consistent with the tolerant-delete contract at the
// use-case layer (app.DeleteArvanCloudLBPoolOrigin).
func TestDeleteArvanCloudLBPoolOriginNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not found"}`))
	}))
	defer srv.Close()

	provider := newTestProvider(t, srv)
	err := provider.DeleteArvanCloudLBPoolOrigin(context.Background(), creds(), "example.com", "lb-1", "pool-1", "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("DeleteArvanCloudLBPoolOrigin() error = %v, want domain.ErrNotFound", err)
	}
}
