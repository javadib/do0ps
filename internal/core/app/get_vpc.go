package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// GetVPCInput identifies the VPC to inspect.
type GetVPCInput struct {
	Credentials domain.ProviderCredentials
	VPCID       string
}

// GetVPC is a fast operation: it runs on a worker but the caller waits for
// the result inside the same tool call.
type GetVPC struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewGetVPC builds the use case from its ports.
func NewGetVPC(queue ports.Queue, provider ports.ParspackProvider) *GetVPC {
	return &GetVPC{queue: queue, provider: provider}
}

// Execute returns the current state of one VPC.
func (uc *GetVPC) Execute(ctx context.Context, in GetVPCInput) (*domain.VPC, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.VPCID == "" {
		return nil, fmt.Errorf("vpc_id is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		vpc, err := uc.provider.GetVPC(ctx, in.Credentials, in.VPCID)
		if err != nil {
			return nil, fmt.Errorf("getting VPC %s: %w", in.VPCID, err)
		}
		return json.Marshal(vpc)
	})
	if err != nil {
		return nil, err
	}

	var vpc domain.VPC
	if err := json.Unmarshal(raw, &vpc); err != nil {
		return nil, fmt.Errorf("decoding VPC: %w", err)
	}
	return &vpc, nil
}
