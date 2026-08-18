package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// GetSSLChallengeInput identifies the order to show challenges for.
type GetSSLChallengeInput struct {
	Credentials domain.ProviderCredentials
	OrderID     string
}

// GetSSLChallenge is a fast operation: it re-shows the challenges of an
// already-processed order, e.g. to display verification instructions again.
type GetSSLChallenge struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewGetSSLChallenge builds the use case from its ports.
func NewGetSSLChallenge(queue ports.Queue, provider ports.ParspackProvider) *GetSSLChallenge {
	return &GetSSLChallenge{queue: queue, provider: provider}
}

// Execute returns the order's current challenges.
func (uc *GetSSLChallenge) Execute(ctx context.Context, in GetSSLChallengeInput) (*domain.SSLChallengeSet, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.OrderID == "" {
		return nil, fmt.Errorf("order_id is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		set, err := uc.provider.GetSSLChallenge(ctx, in.Credentials, in.OrderID)
		if err != nil {
			return nil, fmt.Errorf("getting challenges for SSL order %s: %w", in.OrderID, err)
		}
		return json.Marshal(set)
	})
	if err != nil {
		return nil, err
	}

	var set domain.SSLChallengeSet
	if err := json.Unmarshal(raw, &set); err != nil {
		return nil, fmt.Errorf("decoding SSL order challenges: %w", err)
	}
	return &set, nil
}

// ReloadSSLChallengeInput identifies the order and the new verification
// method to switch to.
type ReloadSSLChallengeInput struct {
	Credentials domain.ProviderCredentials
	OrderID     string
	Method      string // DNS_TXT, FILE, ADMIN, DNS_CNAME, ...
	EmailPrefix string // only meaningful when Method is "ADMIN"
}

// ReloadSSLChallenge is a fast operation: it switches the verification
// method, invalidating any previously shown challenge tokens.
type ReloadSSLChallenge struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewReloadSSLChallenge builds the use case from its ports.
func NewReloadSSLChallenge(queue ports.Queue, provider ports.ParspackProvider) *ReloadSSLChallenge {
	return &ReloadSSLChallenge{queue: queue, provider: provider}
}

// Execute switches the order's verification method.
func (uc *ReloadSSLChallenge) Execute(ctx context.Context, in ReloadSSLChallengeInput) (*domain.SSLChallengeSet, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.OrderID == "" {
		return nil, fmt.Errorf("order_id is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		set, err := uc.provider.ReloadSSLChallenge(ctx, in.Credentials, in.OrderID, in.Method, in.EmailPrefix)
		if err != nil {
			return nil, fmt.Errorf("reloading challenge for SSL order %s: %w", in.OrderID, err)
		}
		return json.Marshal(set)
	})
	if err != nil {
		return nil, err
	}

	var set domain.SSLChallengeSet
	if err := json.Unmarshal(raw, &set); err != nil {
		return nil, fmt.Errorf("decoding reloaded SSL order challenges: %w", err)
	}
	return &set, nil
}

// VerifySSLChallengeInput identifies the order and the method whose
// challenge has been completed (DNS record published, file uploaded, or
// email link clicked).
type VerifySSLChallengeInput struct {
	Credentials domain.ProviderCredentials
	OrderID     string
	Method      string
}

// VerifySSLChallenge is a fast operation: it checks the completed challenge
// and, on success, returns the certificate immediately if it is ready.
type VerifySSLChallenge struct {
	queue    ports.Queue
	provider ports.ParspackProvider
}

// NewVerifySSLChallenge builds the use case from its ports.
func NewVerifySSLChallenge(queue ports.Queue, provider ports.ParspackProvider) *VerifySSLChallenge {
	return &VerifySSLChallenge{queue: queue, provider: provider}
}

// Execute verifies the challenge for in.Method.
func (uc *VerifySSLChallenge) Execute(ctx context.Context, in VerifySSLChallengeInput) (*domain.SSLVerifyResult, error) {
	if err := in.Credentials.Validate(); err != nil {
		return nil, err
	}
	if in.OrderID == "" {
		return nil, fmt.Errorf("order_id is required: %w", domain.ErrInvalidInput)
	}
	if in.Method == "" {
		return nil, fmt.Errorf("method is required: %w", domain.ErrInvalidInput)
	}

	raw, err := uc.queue.Dispatch(ctx, func(ctx context.Context) (json.RawMessage, error) {
		result, err := uc.provider.VerifySSLChallenge(ctx, in.Credentials, in.OrderID, in.Method)
		if err != nil {
			return nil, fmt.Errorf("verifying challenge for SSL order %s: %w", in.OrderID, err)
		}
		return json.Marshal(result)
	})
	if err != nil {
		return nil, err
	}

	var result domain.SSLVerifyResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding SSL challenge verification result: %w", err)
	}
	return &result, nil
}
