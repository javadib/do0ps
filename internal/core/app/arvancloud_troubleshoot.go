package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// --- ListArvanCloudTroubleshoots ------------------------------------------

// ListArvanCloudTroubleshootsInput identifies the domain.
type ListArvanCloudTroubleshootsInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
}

// ListArvanCloudTroubleshoots is a fast operation (AGENTS.md 4.3): returns
// past troubleshoot runs for a domain.
type ListArvanCloudTroubleshoots struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewListArvanCloudTroubleshoots builds the use case from its ports.
func NewListArvanCloudTroubleshoots(queue ports.Queue, provider ports.ArvanCloudProvider) *ListArvanCloudTroubleshoots {
	return &ListArvanCloudTroubleshoots{queue: queue, provider: provider}
}

func (uc *ListArvanCloudTroubleshoots) Execute(ctx context.Context, in ListArvanCloudTroubleshootsInput) ([]domain.ArvanCloudTroubleshoot, error) {
	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		list, err := uc.provider.ListArvanCloudTroubleshoots(ctx, in.Credentials, in.Domain)
		if err != nil {
			return nil, fmt.Errorf("listing arvancloud troubleshoots for %q: %w", in.Domain, err)
		}
		b, err := json.Marshal(list)
		if err != nil {
			return nil, fmt.Errorf("marshaling arvancloud troubleshoots: %w", err)
		}
		return b, nil
	})
	if err != nil {
		return nil, err
	}
	var list []domain.ArvanCloudTroubleshoot
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("decoding arvancloud troubleshoots: %w", err)
	}
	return list, nil
}

// --- RunArvanCloudTroubleshoot -------------------------------------------

// RunArvanCloudTroubleshootInput identifies the domain.
type RunArvanCloudTroubleshootInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
}

// RunArvanCloudTroubleshoot is a fast operation (AGENTS.md 4.3): the POST
// endpoint returns the completed troubleshoot result synchronously (HTTP 201
// with the full Troubleshoot object), so no job/polling is needed.
type RunArvanCloudTroubleshoot struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewRunArvanCloudTroubleshoot builds the use case from its ports.
func NewRunArvanCloudTroubleshoot(queue ports.Queue, provider ports.ArvanCloudProvider) *RunArvanCloudTroubleshoot {
	return &RunArvanCloudTroubleshoot{queue: queue, provider: provider}
}

func (uc *RunArvanCloudTroubleshoot) Execute(ctx context.Context, in RunArvanCloudTroubleshootInput) (*domain.ArvanCloudTroubleshoot, error) {
	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		result, err := uc.provider.RunArvanCloudTroubleshoot(ctx, in.Credentials, in.Domain)
		if err != nil {
			return nil, fmt.Errorf("running arvancloud troubleshoot for %q: %w", in.Domain, err)
		}
		b, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("marshaling arvancloud troubleshoot result: %w", err)
		}
		return b, nil
	})
	if err != nil {
		return nil, err
	}
	var result domain.ArvanCloudTroubleshoot
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud troubleshoot result: %w", err)
	}
	return &result, nil
}

// --- GetLatestArvanCloudTroubleshoot ------------------------------------

// GetLatestArvanCloudTroubleshootInput identifies the domain.
type GetLatestArvanCloudTroubleshootInput struct {
	Credentials domain.ProviderCredentials
	Domain      string
}

// GetLatestArvanCloudTroubleshoot is a fast operation (AGENTS.md 4.3):
// returns the most recent troubleshoot run for a domain.
type GetLatestArvanCloudTroubleshoot struct {
	queue    ports.Queue
	provider ports.ArvanCloudProvider
}

// NewGetLatestArvanCloudTroubleshoot builds the use case from its ports.
func NewGetLatestArvanCloudTroubleshoot(queue ports.Queue, provider ports.ArvanCloudProvider) *GetLatestArvanCloudTroubleshoot {
	return &GetLatestArvanCloudTroubleshoot{queue: queue, provider: provider}
}

func (uc *GetLatestArvanCloudTroubleshoot) Execute(ctx context.Context, in GetLatestArvanCloudTroubleshootInput) (*domain.ArvanCloudTroubleshoot, error) {
	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		result, err := uc.provider.GetLatestArvanCloudTroubleshoot(ctx, in.Credentials, in.Domain)
		if err != nil {
			return nil, fmt.Errorf("getting latest arvancloud troubleshoot for %q: %w", in.Domain, err)
		}
		b, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("marshaling arvancloud troubleshoot result: %w", err)
		}
		return b, nil
	})
	if err != nil {
		return nil, err
	}
	var result domain.ArvanCloudTroubleshoot
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding arvancloud troubleshoot result: %w", err)
	}
	return &result, nil
}
