package parspack_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

func TestCreateVMSnapshotSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.URL.Path; got != "/api/public/v1/vms/vm-1/actions" {
			t.Errorf("path = %s, want /api/public/v1/vms/vm-1/actions", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q, want Bearer test-key", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["type"] != "snapshot" {
			t.Errorf("type = %v, want snapshot", body["type"])
		}
		if body["name"] != "pre-upgrade" {
			t.Errorf("name = %v, want pre-upgrade", body["name"])
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"action":{"id":123,"status":"in-progress","type":"snapshot","resource_id":"vm-1","resource_type":"vm","started_at":"2026-08-18T10:00:00Z"}}`))
	})

	action, err := c.CreateVMSnapshot(context.Background(), creds, "vm-1", "pre-upgrade")
	if err != nil {
		t.Fatalf("CreateVMSnapshot: %v", err)
	}
	if action.ID != "123" {
		t.Errorf("action ID = %q, want 123", action.ID)
	}
	if action.Status != domain.VMActionStatusInProgress {
		t.Errorf("status = %q, want in-progress", action.Status)
	}
	if action.ServerID != "vm-1" {
		t.Errorf("ServerID = %q, want vm-1", action.ServerID)
	}
	if action.Type != "snapshot" {
		t.Errorf("type = %q, want snapshot", action.Type)
	}
}

func TestCreateVMSnapshotMapsNotFound(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"vm not found"}`))
	})

	_, err := c.CreateVMSnapshot(context.Background(), creds, "missing", "snap")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestGetVMActionSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if got := r.URL.Path; got != "/api/public/v1/vms/vm-1/actions/123" {
			t.Errorf("path = %s, want /api/public/v1/vms/vm-1/actions/123", got)
		}
		_, _ = w.Write([]byte(`{"action":{"id":123,"status":"completed","type":"snapshot","resource_id":"vm-1","completed_at":"2026-08-18T10:05:00Z"}}`))
	})

	action, err := c.GetVMAction(context.Background(), creds, "vm-1", "123")
	if err != nil {
		t.Fatalf("GetVMAction: %v", err)
	}
	if action.Status != domain.VMActionStatusCompleted {
		t.Errorf("status = %q, want completed", action.Status)
	}
}

func TestListVMSnapshotsSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if got := r.URL.Path; got != "/api/public/v1/snapshots" {
			t.Errorf("path = %s, want /api/public/v1/snapshots", got)
		}
		if got := r.URL.Query().Get("resource_type"); got != "vm" {
			t.Errorf("resource_type = %q, want vm", got)
		}
		_, _ = w.Write([]byte(`{"snapshots":[
			{"id":"5001","name":"pre-upgrade","resource_id":"vm-1","resource_type":"vm","regions":["tehran"],"min_disk_size":25,"size_gigabytes":2.5,"created_at":"2026-08-18T10:05:00Z"},
			{"id":"5002","name":"baseline","resource_id":"vm-2","resource_type":"vm"}]}`))
	})

	snapshots, err := c.ListVMSnapshots(context.Background(), creds)
	if err != nil {
		t.Fatalf("ListVMSnapshots: %v", err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("len(snapshots) = %d, want 2", len(snapshots))
	}
	snap := snapshots[0]
	if snap.ID != "5001" || snap.Name != "pre-upgrade" || snap.ServerID != "vm-1" {
		t.Errorf("snapshot = %+v, want id 5001, name pre-upgrade, server vm-1", snap)
	}
	if len(snap.Regions) != 1 || snap.Regions[0] != "tehran" {
		t.Errorf("Regions = %v, want [tehran]", snap.Regions)
	}
	if snap.MinDiskGB != 25 {
		t.Errorf("MinDiskGB = %d, want 25", snap.MinDiskGB)
	}
	if snap.SizeGB != 2 {
		t.Errorf("SizeGB = %d, want 2", snap.SizeGB)
	}
}

func TestListVMSnapshotsProviderUnavailable(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"boom"}`))
	})

	_, err := c.ListVMSnapshots(context.Background(), creds)
	if !errors.Is(err, domain.ErrProviderUnavailable) {
		t.Fatalf("error = %v, want domain.ErrProviderUnavailable", err)
	}
}

func TestDeleteVMSnapshotSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		if got := r.URL.Path; got != "/api/public/v1/snapshots/5001" {
			t.Errorf("path = %s, want /api/public/v1/snapshots/5001", got)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	if err := c.DeleteVMSnapshot(context.Background(), creds, "5001"); err != nil {
		t.Fatalf("DeleteVMSnapshot: %v", err)
	}
}

func TestDeleteVMSnapshotNotFound(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	err := c.DeleteVMSnapshot(context.Background(), creds, "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestRestoreVMSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.URL.Path; got != "/api/public/v1/vms/vm-1/actions" {
			t.Errorf("path = %s, want /api/public/v1/vms/vm-1/actions", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["type"] != "restore" {
			t.Errorf("type = %v, want restore", body["type"])
		}
		// The restore action takes the snapshot as a JSON number, not a string.
		if got, ok := body["image"].(float64); !ok || got != 5001 {
			t.Errorf("image = %v, want the number 5001", body["image"])
		}

		_, _ = w.Write([]byte(`{"action":{"id":124,"status":"in-progress","type":"restore","resource_id":"vm-1","resource_type":"vm"}}`))
	})

	action, err := c.RestoreVM(context.Background(), creds, "vm-1", "5001")
	if err != nil {
		t.Fatalf("RestoreVM: %v", err)
	}
	if action.Status != domain.VMActionStatusInProgress {
		t.Errorf("status = %q, want in-progress", action.Status)
	}
}

func TestRestoreVMRejectsNonNumericSnapshotID(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("no request should reach the transport when the snapshot ID is not numeric")
	})

	_, err := c.RestoreVM(context.Background(), creds, "vm-1", "snap-not-a-number")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}
