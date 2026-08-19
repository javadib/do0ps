package parspack_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

func TestListCDNBulklistsSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/user/bulklists" {
			t.Errorf("path = %s, want /external/api/v1/user/bulklists", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q, want Bearer test-key", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[
			{"id":"RD0kerP6","name":"list1","type":"country","items":[{"value":"1","value_detail":"Afghanistan"}]},
			{"id":"vpaO2roj","name":"list2","type":"ip","items":[{"value":"1.1.1.1","value_detail":null},{"value":"2.2.2.2","value_detail":null}]}
		]}`))
	})

	lists, err := c.ListCDNBulklists(context.Background(), creds)
	if err != nil {
		t.Fatalf("ListCDNBulklists: %v", err)
	}
	if len(lists) != 2 {
		t.Fatalf("len(lists) = %d, want 2", len(lists))
	}
	if lists[0].ID != "RD0kerP6" || lists[0].Type != "country" || lists[0].Items[0].ValueDetail != "Afghanistan" {
		t.Errorf("lists[0] = %+v, want id RD0kerP6, type country, item detail Afghanistan", lists[0])
	}
	if lists[1].Type != "ip" || len(lists[1].Items) != 2 {
		t.Errorf("lists[1] = %+v, want type ip with 2 items", lists[1])
	}
}

func TestCreateCDNBulklistSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.URL.Path; got != "/external/api/v1/user/bulklists" {
			t.Errorf("path = %s, want /external/api/v1/user/bulklists", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["name"] != "Blocked IPs" || body["type"] != "ip" {
			t.Errorf("body = %+v, want name/type Blocked IPs/ip", body)
		}
		items, ok := body["items"].([]any)
		if !ok || len(items) != 1 || items[0] != "192.168.0.1" {
			t.Errorf("body.items = %+v, want [192.168.0.1]", body["items"])
		}

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	list, err := c.CreateCDNBulklist(context.Background(), creds, domain.CDNBulklistSpec{
		Name: "Blocked IPs", Type: "ip", Items: []string{"192.168.0.1"},
	})
	if err != nil {
		t.Fatalf("CreateCDNBulklist: %v", err)
	}
	if list.Name != "Blocked IPs" || list.Type != "ip" || len(list.Items) != 1 {
		t.Errorf("list = %+v, want name Blocked IPs, type ip, 1 item", list)
	}
}

func TestGetCDNBulklistSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/user/bulklists/RD0kerP6" {
			t.Errorf("path = %s, want /external/api/v1/user/bulklists/RD0kerP6", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":{
			"id":"RD0kerP6","name":"list1","type":"country","items":[{"value":"1","value_detail":"Afghanistan"}]
		}}`))
	})

	list, err := c.GetCDNBulklist(context.Background(), creds, "RD0kerP6")
	if err != nil {
		t.Fatalf("GetCDNBulklist: %v", err)
	}
	if list.ID != "RD0kerP6" || list.Name != "list1" {
		t.Errorf("list = %+v, want id RD0kerP6, name list1", list)
	}
}

func TestGetCDNBulklistNotFound(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"message":"Not Found","errors":[]}`))
	})

	_, err := c.GetCDNBulklist(context.Background(), creds, "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestUpdateCDNBulklistSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if got := r.URL.Path; got != "/external/api/v1/user/bulklists/RD0kerP6" {
			t.Errorf("path = %s, want /external/api/v1/user/bulklists/RD0kerP6", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	list, err := c.UpdateCDNBulklist(context.Background(), creds, "RD0kerP6", domain.CDNBulklistSpec{
		Name: "Blocked IPs v2", Type: "ip", Items: []string{"192.168.0.2"},
	})
	if err != nil {
		t.Fatalf("UpdateCDNBulklist: %v", err)
	}
	if list.ID != "RD0kerP6" || list.Name != "Blocked IPs v2" {
		t.Errorf("list = %+v, want id RD0kerP6, name Blocked IPs v2", list)
	}
}

func TestDeleteCDNBulklistSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		if got := r.URL.Path; got != "/external/api/v1/user/bulklists/RD0kerP6" {
			t.Errorf("path = %s, want /external/api/v1/user/bulklists/RD0kerP6", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	if err := c.DeleteCDNBulklist(context.Background(), creds, "RD0kerP6"); err != nil {
		t.Fatalf("DeleteCDNBulklist: %v", err)
	}
}

func TestDeleteCDNBulklistNotFound(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"message":"Not Found","errors":[]}`))
	})

	err := c.DeleteCDNBulklist(context.Background(), creds, "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestListCDNFirewallCountriesSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1/firewalls/countries" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1/firewalls/countries", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[
			{"id":"1","name":"Afghanistan"},
			{"id":"2","name":"Aland Islands"}
		]}`))
	})

	countries, err := c.ListCDNFirewallCountries(context.Background(), creds, "zone-1")
	if err != nil {
		t.Fatalf("ListCDNFirewallCountries: %v", err)
	}
	if len(countries) != 2 || countries[0].Code != "1" || countries[0].Name != "Afghanistan" {
		t.Errorf("countries = %+v, want first entry code 1 named Afghanistan", countries)
	}
}

func TestListCDNFirewallCountriesUnauthorized(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Unauthenticated."}`))
	})

	_, err := c.ListCDNFirewallCountries(context.Background(), creds, "zone-1")
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("error = %v, want domain.ErrInvalidCredentials", err)
	}
}
