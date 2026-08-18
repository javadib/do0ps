package mcp

import (
	"context"
	"encoding/json"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// snapshotToMap renders a domain.VMSnapshot the way every snapshot-returning
// tool reports it back to the caller.
func snapshotToMap(snap domain.VMSnapshot) map[string]any {
	return map[string]any{
		"id":          snap.ID,
		"name":        snap.Name,
		"server_id":   snap.ServerID,
		"regions":     snap.Regions,
		"min_disk_gb": snap.MinDiskGB,
		"size_gb":     snap.SizeGB,
		"created_at":  snap.CreatedAt,
	}
}

type createSnapshotArgs struct {
	credentialArgs
	ServerID string `json:"server_id"`
	Name     string `json:"name"`
}

func createSnapshotTool(uc *app.CreateSnapshot) Tool {
	props := credentialProperties()
	props["server_id"] = map[string]any{
		"type":        "string",
		"description": "The provider ID of the server to snapshot, as returned by create_server or list_servers.",
	}
	props["name"] = map[string]any{
		"type":        "string",
		"description": "A name for the snapshot, e.g. \"web-01-before-upgrade\". It is also how a retry recognizes an already-created snapshot.",
	}

	return Tool{
		Name: "create_snapshot",
		Description: "Take a point-in-time snapshot of a server's disk at Parspack. This is a long operation: it returns " +
			"immediately with an operation_id and status \"pending\". Poll get_operation_status with that id to learn when " +
			"the snapshot is ready; its id can then be used as the image for a new server.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "server_id", "name"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args createSnapshotArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			out, err := uc.Execute(ctx, app.CreateSnapshotInput{
				Credentials: args.domain(),
				ServerID:    args.ServerID,
				Name:        args.Name,
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"operation_id": out.OperationID,
				"status":       out.Status.String(),
				"note":         "Snapshotting runs in the background. Call get_operation_status with this operation_id to check progress.",
			}, nil
		},
	}
}

func listSnapshotsTool(uc *app.ListSnapshots) Tool {
	props := credentialProperties()

	return Tool{
		Name: "list_snapshots",
		Description: "List every VM snapshot at Parspack visible to the given credentials. This is a fast operation: " +
			"the list is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args credentialArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			snapshots, err := uc.Execute(ctx, app.ListSnapshotsInput{Credentials: args.domain()})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(snapshots))
			for i, snap := range snapshots {
				out[i] = snapshotToMap(snap)
			}
			return map[string]any{"snapshots": out}, nil
		},
	}
}

type snapshotIDArgs struct {
	credentialArgs
	SnapshotID string `json:"snapshot_id"`
}

func deleteSnapshotTool(uc *app.DeleteSnapshot) Tool {
	props := credentialProperties()
	props["snapshot_id"] = map[string]any{
		"type":        "string",
		"description": "The provider ID of the snapshot to delete, as returned by create_snapshot or list_snapshots.",
	}

	return Tool{
		Name: "delete_snapshot",
		Description: "Permanently delete a VM snapshot at Parspack by its provider ID. This is a fast operation and " +
			"cannot be undone. Deleting a snapshot that no longer exists is treated as already done rather than an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "snapshot_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args snapshotIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteSnapshotInput{
				Credentials: args.domain(),
				SnapshotID:  args.SnapshotID,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "snapshot_id": args.SnapshotID}, nil
		},
	}
}

type restoreVMArgs struct {
	credentialArgs
	ServerID   string `json:"server_id"`
	SnapshotID string `json:"snapshot_id"`
}

func restoreVMTool(uc *app.RestoreVM) Tool {
	props := credentialProperties()
	props["server_id"] = map[string]any{
		"type":        "string",
		"description": "The provider ID of the server to restore, as returned by create_server or list_servers.",
	}
	props["snapshot_id"] = map[string]any{
		"type":        "string",
		"description": "The provider ID of the snapshot to restore, as returned by create_snapshot or list_snapshots.",
	}

	return Tool{
		Name: "restore_vm",
		Description: "Wipe a server's disk at Parspack and replace it with the contents of a snapshot. This is a long " +
			"operation: it returns immediately with an operation_id and status \"pending\", and it is destructive and " +
			"cannot be undone. Poll get_operation_status with that id to learn when the restore is complete.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "server_id", "snapshot_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args restoreVMArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			out, err := uc.Execute(ctx, app.RestoreVMInput{
				Credentials: args.domain(),
				ServerID:    args.ServerID,
				SnapshotID:  args.SnapshotID,
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"operation_id": out.OperationID,
				"status":       out.Status.String(),
				"note":         "Restoring runs in the background. Call get_operation_status with this operation_id to check progress.",
			}, nil
		},
	}
}
