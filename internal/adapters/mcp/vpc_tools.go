package mcp

import (
	"context"
	"encoding/json"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

type createVPCArgs struct {
	credentialArgs
	Name        string `json:"name"`
	Region      string `json:"region"`
	Description string `json:"description"`
	IPRange     string `json:"ip_range"`
}

func createVPCTool(uc *app.CreateVPC) Tool {
	props := credentialProperties()
	props["name"] = map[string]any{
		"type":        "string",
		"description": "A name for the VPC, e.g. \"web-net\". Must be unique within the account and contain alphanumeric characters only.",
	}
	props["region"] = map[string]any{
		"type":        "string",
		"description": "Provider datacenter region, e.g. \"tehran\".",
	}
	props["description"] = map[string]any{
		"type":        "string",
		"description": "Free-form text up to 255 characters describing the VPC, e.g. \"network for the web tier\".",
	}
	props["ip_range"] = map[string]any{
		"type":        "string",
		"description": "Private IP range for the VPC, e.g. \"10.10.10.0/24\". Must be RFC1918, no larger than /16 and no smaller than /24. Omit to use the provider default.",
	}

	return Tool{
		Name: "create_vpc",
		Description: "Create an isolated private network (VPC) at Parspack. Servers can later be placed into it via " +
			"create_server's vpc_uuid parameter. This is a fast operation: the created VPC (with its provider id and " +
			"default flag) is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "name", "region"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args createVPCArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			vpc, err := uc.Execute(ctx, app.CreateVPCInput{
				Credentials: args.domain(),
				VPC: domain.VPC{
					Name:        args.Name,
					Region:      args.Region,
					Description: args.Description,
					IPRange:     args.IPRange,
				},
			})
			if err != nil {
				return nil, err
			}
			return vpcToMap(*vpc), nil
		},
	}
}

func listVPCsTool(uc *app.ListVPCs) Tool {
	props := credentialProperties()

	return Tool{
		Name: "list_vpcs",
		Description: "List every VPC (private network) at Parspack visible to the given credentials. This is a fast " +
			"operation: the list is returned within this call.",
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

			vpcs, err := uc.Execute(ctx, app.ListVPCsInput{Credentials: args.domain()})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(vpcs))
			for i, vpc := range vpcs {
				out[i] = vpcToMap(vpc)
			}
			return map[string]any{"vpcs": out}, nil
		},
	}
}

type vpcIDArgs struct {
	credentialArgs
	VPCID string `json:"vpc_id"`
}

func getVPCTool(uc *app.GetVPC) Tool {
	props := credentialProperties()
	props["vpc_id"] = map[string]any{
		"type":        "string",
		"description": "The provider ID of the VPC to look up, as returned by create_vpc or list_vpcs.",
	}

	return Tool{
		Name: "get_vpc",
		Description: "Get the current state of one VPC (private network) at Parspack by its provider ID. This is a fast " +
			"operation: the result is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "vpc_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args vpcIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			vpc, err := uc.Execute(ctx, app.GetVPCInput{
				Credentials: args.domain(),
				VPCID:       args.VPCID,
			})
			if err != nil {
				return nil, err
			}
			return vpcToMap(*vpc), nil
		},
	}
}

func deleteVPCTool(uc *app.DeleteVPC) Tool {
	props := credentialProperties()
	props["vpc_id"] = map[string]any{
		"type":        "string",
		"description": "The provider ID of the VPC to delete, as returned by create_vpc or list_vpcs.",
	}

	return Tool{
		Name: "delete_vpc",
		Description: "Permanently delete a VPC (private network) at Parspack by its provider ID. This is a fast " +
			"operation and cannot be undone. Deleting a VPC that no longer exists is treated as already done rather " +
			"than an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "vpc_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args vpcIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteVPCInput{
				Credentials: args.domain(),
				VPCID:       args.VPCID,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "vpc_id": args.VPCID}, nil
		},
	}
}

// vpcToMap renders a domain.VPC the way every VPC-returning tool reports it
// back to the caller.
func vpcToMap(vpc domain.VPC) map[string]any {
	return map[string]any{
		"id":          vpc.ID,
		"name":        vpc.Name,
		"region":      vpc.Region,
		"description": vpc.Description,
		"ip_range":    vpc.IPRange,
		"default":     vpc.Default,
		"created_at":  vpc.CreatedAt,
	}
}
