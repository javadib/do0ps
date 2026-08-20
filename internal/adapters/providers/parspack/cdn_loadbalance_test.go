package parspack_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

func TestListCDNLoadBalancesSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/load-balance" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/load-balance", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[
			{"id":"BMAjyE0v","name":"lb2","enabled":true,"retry_count":1,"enable_cookie_persist":true,
			 "server_fail_count_to_be_down":1,"method":"round_robin","down_servers_recovery_time":1,
			 "cookie_persist_expire_time":1,"servers":[
				{"id":"xOADBZaB","name":"lbs1","ip":"1.1.1.1","http_port":1,"https_port":443,"weight":1,"recovery_time":1,"group":"primary","active":true}
			 ]}
		]}`))
	})

	balances, err := c.ListCDNLoadBalances(context.Background(), creds, "zone-1")
	if err != nil {
		t.Fatalf("ListCDNLoadBalances: %v", err)
	}
	if len(balances) != 1 || balances[0].ID != "BMAjyE0v" || balances[0].Name != "lb2" {
		t.Errorf("balances = %+v, want a single lb2 pool with ID BMAjyE0v", balances)
	}
	if len(balances[0].Servers) != 1 || balances[0].Servers[0].IP != "1.1.1.1" {
		t.Errorf("balances[0].Servers = %+v, want one server at 1.1.1.1", balances[0].Servers)
	}
}

func TestCreateCDNLoadBalanceSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/load-balance" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/load-balance", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["name"] != "lb2" || body["enabled"] != true || body["method"] != "round_robin" {
			t.Errorf("body = %+v, want name/enabled/method lb2/true/round_robin", body)
		}
		servers, ok := body["servers"].([]any)
		if !ok || len(servers) != 1 {
			t.Errorf("body.servers = %+v, want a single server", body["servers"])
		}

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	lb, err := c.CreateCDNLoadBalance(context.Background(), creds, "zone-1", domain.CDNLoadBalance{
		Name: "lb2", Enabled: true, Method: "round_robin",
		Servers: []domain.CDNLoadBalanceServer{{Name: "lbs1", IP: "1.1.1.1", Active: true, Group: "primary"}},
	})
	if err != nil {
		t.Fatalf("CreateCDNLoadBalance: %v", err)
	}
	if lb.Name != "lb2" {
		t.Errorf("lb.Name = %q, want lb2", lb.Name)
	}
	if lb.ID != "" {
		t.Errorf("lb.ID = %q, want empty (provider does not return one on create)", lb.ID)
	}
}

func TestGetCDNLoadBalanceSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/load-balance/BMAjyE0v" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/load-balance/BMAjyE0v", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":
			{"id":"BMAjyE0v","name":"lb2","enabled":true,"retry_count":1,"enable_cookie_persist":true,
			 "server_fail_count_to_be_down":1,"method":"round_robin","down_servers_recovery_time":1,
			 "cookie_persist_expire_time":1,"servers":[]}
		}`))
	})

	lb, err := c.GetCDNLoadBalance(context.Background(), creds, "zone-1", "BMAjyE0v")
	if err != nil {
		t.Fatalf("GetCDNLoadBalance: %v", err)
	}
	if lb.ID != "BMAjyE0v" || lb.Method != "round_robin" {
		t.Errorf("lb = %+v, want ID BMAjyE0v and method round_robin", lb)
	}
}

func TestGetCDNLoadBalanceNotFound(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"message":"Not Found","errors":[]}`))
	})

	_, err := c.GetCDNLoadBalance(context.Background(), creds, "zone-1", "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestUpdateCDNLoadBalanceSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/load-balance/BMAjyE0v" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/load-balance/BMAjyE0v", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if _, hasServers := body["servers"]; hasServers {
			t.Errorf("body = %+v, want no servers field on update", body)
		}

		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	lb, err := c.UpdateCDNLoadBalance(context.Background(), creds, "zone-1", "BMAjyE0v", domain.CDNLoadBalance{
		Name: "lb2-renamed", Enabled: false,
	})
	if err != nil {
		t.Fatalf("UpdateCDNLoadBalance: %v", err)
	}
	if lb.ID != "BMAjyE0v" || lb.Name != "lb2-renamed" {
		t.Errorf("lb = %+v, want ID BMAjyE0v and name lb2-renamed", lb)
	}
}

func TestDeleteCDNLoadBalanceSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	if err := c.DeleteCDNLoadBalance(context.Background(), creds, "zone-1", "BMAjyE0v"); err != nil {
		t.Fatalf("DeleteCDNLoadBalance: %v", err)
	}
}

func TestDeleteCDNLoadBalanceNotFound(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"message":"Not Found","errors":[]}`))
	})

	err := c.DeleteCDNLoadBalance(context.Background(), creds, "zone-1", "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestListCDNLoadBalanceServersSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/load-balance-server" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/load-balance-server", got)
		}
		if got := r.URL.Query().Get("load_balance_id"); got != "BMAjyE0v" {
			t.Errorf("load_balance_id query = %q, want BMAjyE0v", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[
			{"id":"BMAj3Erv","name":"server-1","ip":"1.1.1.1","http_port":1,"https_port":443,"weight":1,"recovery_time":1,"group":"primary","active":true}
		]}`))
	})

	servers, err := c.ListCDNLoadBalanceServers(context.Background(), creds, "zone-1", "BMAjyE0v")
	if err != nil {
		t.Fatalf("ListCDNLoadBalanceServers: %v", err)
	}
	if len(servers) != 1 || servers[0].ID != "BMAj3Erv" || servers[0].Name != "server-1" {
		t.Errorf("servers = %+v, want a single server-1 with ID BMAj3Erv", servers)
	}
}

func TestCreateCDNLoadBalanceServerSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/load-balance-server" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/load-balance-server", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["ip"] != "1.1.1.1" || body["active"] != true {
			t.Errorf("body = %+v, want ip/active 1.1.1.1/true", body)
		}
		if _, hasName := body["name"]; hasName {
			t.Errorf("body = %+v, want no name field on create", body)
		}

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	srv, err := c.CreateCDNLoadBalanceServer(context.Background(), creds, "zone-1", domain.CDNLoadBalanceServer{
		Name: "server-1", IP: "1.1.1.1", Active: true, Group: "primary",
	})
	if err != nil {
		t.Fatalf("CreateCDNLoadBalanceServer: %v", err)
	}
	if srv.IP != "1.1.1.1" {
		t.Errorf("srv.IP = %q, want 1.1.1.1", srv.IP)
	}
	if srv.ID != "" {
		t.Errorf("srv.ID = %q, want empty (provider does not return one on create)", srv.ID)
	}
}

func TestGetCDNLoadBalanceServerSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/load-balance-server/BMAj3Erv" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/load-balance-server/BMAj3Erv", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":
			{"id":"BMAj3Erv","name":"lb1","ip":"1.1.1.1","http_port":1,"https_port":443,"weight":1,"recovery_time":1,"group":"primary","active":true}
		}`))
	})

	srv, err := c.GetCDNLoadBalanceServer(context.Background(), creds, "zone-1", "BMAj3Erv")
	if err != nil {
		t.Fatalf("GetCDNLoadBalanceServer: %v", err)
	}
	if srv.ID != "BMAj3Erv" || srv.Name != "lb1" {
		t.Errorf("srv = %+v, want ID BMAj3Erv and name lb1", srv)
	}
}

func TestGetCDNLoadBalanceServerNotFound(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"message":"Not Found","errors":[]}`))
	})

	_, err := c.GetCDNLoadBalanceServer(context.Background(), creds, "zone-1", "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestUpdateCDNLoadBalanceServerSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["name"] != "server-1-renamed" {
			t.Errorf("body = %+v, want name server-1-renamed", body)
		}

		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	srv, err := c.UpdateCDNLoadBalanceServer(context.Background(), creds, "zone-1", "BMAj3Erv", domain.CDNLoadBalanceServer{
		Name: "server-1-renamed", IP: "1.1.1.1", Group: "primary", Active: true,
	})
	if err != nil {
		t.Fatalf("UpdateCDNLoadBalanceServer: %v", err)
	}
	if srv.ID != "BMAj3Erv" || srv.Name != "server-1-renamed" {
		t.Errorf("srv = %+v, want ID BMAj3Erv and name server-1-renamed", srv)
	}
}

func TestDeleteCDNLoadBalanceServerSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	if err := c.DeleteCDNLoadBalanceServer(context.Background(), creds, "zone-1", "BMAj3Erv"); err != nil {
		t.Fatalf("DeleteCDNLoadBalanceServer: %v", err)
	}
}

func TestDeleteCDNLoadBalanceServerNotFound(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"message":"Not Found","errors":[]}`))
	})

	err := c.DeleteCDNLoadBalanceServer(context.Background(), creds, "zone-1", "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}
