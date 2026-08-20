package parspack

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/javadib/do0ps/internal/core/domain"
)

// snapshotBasePath is confirmed against github.com/abrhacom/go-api-abrha's
// snapshots.go, relative to Client.baseURL, i.e.
// https://my.parspack.com/cserver/api/public/v1/snapshots.
const snapshotBasePath = "api/public/v1/snapshots"

// The wire types below mirror github.com/abrhacom/go-api-abrha's snapshots.go,
// vm_actions.go and action.go exactly (field names and JSON tags) so this
// adapter decodes real Parspack responses correctly. Nothing above the adapter
// boundary ever sees these — the methods below translate them into
// internal/core/domain types.

type snapshotWire struct {
	ID            string   `json:"id,omitempty"`
	Name          string   `json:"name,omitempty"`
	ResourceID    string   `json:"resource_id,omitempty"`
	ResourceType  string   `json:"resource_type,omitempty"`
	Regions       []string `json:"regions,omitempty"`
	MinDiskSize   int      `json:"min_disk_size,omitempty"`
	SizeGigaBytes float64  `json:"size_gigabytes,omitempty"`
	Created       string   `json:"created_at,omitempty"`
}

type snapshotsRoot struct {
	Snapshots []snapshotWire `json:"snapshots"`
}

// actionWire mirrors go-api-abrha's Action. VM actions are started with a POST
// to /vms/{id}/actions and polled through /vms/{id}/actions/{action_id}; both
// answer with the action wrapped under the "action" key.
type actionWire struct {
	ID           int    `json:"id"`
	Status       string `json:"status"`
	Type         string `json:"type"`
	ResourceID   string `json:"resource_id"`
	ResourceType string `json:"resource_type"`
	StartedAt    string `json:"started_at"`
	CompletedAt  string `json:"completed_at"`
}

type actionRoot struct {
	Action *actionWire `json:"action"`
}

// actionRequest is the body sent to start a VM action. It mirrors
// go-api-abrha's ActionRequest (a free-form map); only the "type" plus the one
// extra field each verb takes are ever sent here.
type actionRequest struct {
	Type  string `json:"type"`
	Name  string `json:"name,omitempty"`
	Image *int   `json:"image,omitempty"`
}

// vmActionPath is the "start an action" path for one server.
func vmActionPath(serverID string) string {
	return vmBasePath + "/" + serverID + "/actions"
}

// CreateVMSnapshot starts the async snapshot action for a server. The
// returned VMAction is "in-progress"; the snapshot itself only appears in
// ListVMSnapshots once the action completes.
func (c *Client) CreateVMSnapshot(ctx context.Context, creds domain.ProviderCredentials, serverID, name string) (*domain.VMAction, error) {
	reqBody := actionRequest{Type: "snapshot", Name: name}

	var root actionRoot
	if err := c.doJSON(ctx, creds, "POST", vmActionPath(serverID), reqBody, &root); err != nil {
		return nil, fmt.Errorf("starting snapshot %q of server %s: %w", name, serverID, err)
	}
	if root.Action == nil {
		return nil, fmt.Errorf("starting snapshot %q of server %s: %w", name, serverID, errEmptyResponse)
	}
	return toDomainAction(root.Action), nil
}

// GetVMAction returns the current state of an action running on a server.
func (c *Client) GetVMAction(ctx context.Context, creds domain.ProviderCredentials, serverID, actionID string) (*domain.VMAction, error) {
	var root actionRoot
	if err := c.doJSON(ctx, creds, "GET", vmActionPath(serverID)+"/"+actionID, nil, &root); err != nil {
		return nil, fmt.Errorf("get action %s of server %s: %w", actionID, serverID, err)
	}
	if root.Action == nil {
		return nil, fmt.Errorf("get action %s of server %s: %w", actionID, serverID, errEmptyResponse)
	}
	return toDomainAction(root.Action), nil
}

// ListVMSnapshots returns every VM snapshot visible to the credentials. Only
// VM snapshots are returned — the API serves volume snapshots from the same
// endpoint, filtered by resource_type.
func (c *Client) ListVMSnapshots(ctx context.Context, creds domain.ProviderCredentials) ([]domain.VMSnapshot, error) {
	var root snapshotsRoot
	if err := c.doJSON(ctx, creds, "GET", snapshotBasePath+"?resource_type=vm", nil, &root); err != nil {
		return nil, fmt.Errorf("list VM snapshots: %w", err)
	}

	snapshots := make([]domain.VMSnapshot, len(root.Snapshots))
	for i := range root.Snapshots {
		snapshots[i] = *toDomainSnapshot(&root.Snapshots[i])
	}
	return snapshots, nil
}

// DeleteVMSnapshot removes a snapshot by provider ID.
func (c *Client) DeleteVMSnapshot(ctx context.Context, creds domain.ProviderCredentials, id string) error {
	if err := c.doJSON(ctx, creds, "DELETE", snapshotBasePath+"/"+id, nil, nil); err != nil {
		return fmt.Errorf("delete VM snapshot %s: %w", id, err)
	}
	return nil
}

// RestoreVM wipes the disk of serverID and replaces it with the contents of
// snapshotID. The action endpoint takes the image as a JSON number, so the
// snapshot ID must be numeric.
func (c *Client) RestoreVM(ctx context.Context, creds domain.ProviderCredentials, serverID, snapshotID string) (*domain.VMAction, error) {
	imageID, err := strconv.Atoi(snapshotID)
	if err != nil {
		return nil, fmt.Errorf("restoring snapshot %s to server %s: snapshot ID must be numeric: %w", snapshotID, serverID, domain.ErrInvalidInput)
	}

	reqBody := actionRequest{Type: "restore", Image: &imageID}

	var root actionRoot
	if err := c.doJSON(ctx, creds, "POST", vmActionPath(serverID), reqBody, &root); err != nil {
		return nil, fmt.Errorf("restoring snapshot %s to server %s: %w", snapshotID, serverID, err)
	}
	if root.Action == nil {
		return nil, fmt.Errorf("restoring snapshot %s to server %s: %w", snapshotID, serverID, errEmptyResponse)
	}
	return toDomainAction(root.Action), nil
}

// toDomainAction translates a wire action into the shared domain.VMAction
// shape.
func toDomainAction(a *actionWire) *domain.VMAction {
	action := &domain.VMAction{
		ID:       strconv.Itoa(a.ID),
		ServerID: a.ResourceID,
		Type:     a.Type,
		Status:   a.Status,
	}
	if a.StartedAt != "" {
		if t, err := time.Parse(time.RFC3339, a.StartedAt); err == nil {
			action.StartedAt = t
		}
	}
	if a.CompletedAt != "" {
		if t, err := time.Parse(time.RFC3339, a.CompletedAt); err == nil {
			action.CompletedAt = t
		}
	}
	return action
}

// toDomainSnapshot translates a wire snapshot into the shared domain.VMSnapshot
// shape. The billable size comes down as gigabytes (a float on the wire).
func toDomainSnapshot(s *snapshotWire) *domain.VMSnapshot {
	snap := &domain.VMSnapshot{
		ID:        s.ID,
		Name:      s.Name,
		ServerID:  s.ResourceID,
		Regions:   s.Regions,
		MinDiskGB: s.MinDiskSize,
		SizeGB:    int(s.SizeGigaBytes),
	}
	if s.Created != "" {
		if t, err := time.Parse(time.RFC3339, s.Created); err == nil {
			snap.CreatedAt = t
		}
	}
	return snap
}
