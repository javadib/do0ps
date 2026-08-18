package parspack

import (
	"context"
	"fmt"
	"strconv"

	"github.com/javadib/do0ps/internal/core/domain"
)

// sshKeysBasePath is confirmed against github.com/abrhacom/go-api-abrha's
// keys.go (AGENTS.md 4.5: cross-reference that client when the docs site
// cannot be scraped). Relative to Client.baseURL, i.e.
// https://my.parspack.com/cserver/api/public/v1/account/keys.
const sshKeysBasePath = "api/public/v1/account/keys"

// The wire types below mirror github.com/abrhacom/go-api-abrha's keys.go
// exactly (field names and JSON tags) so this adapter decodes real Parspack
// responses correctly. Nothing above the adapter boundary ever sees these —
// CreateSSHKey/ListSSHKeys/DeleteSSHKey translate them into
// internal/core/domain types.

type sshKeyWire struct {
	ID          int    `json:"id,float64,omitempty"`
	Name        string `json:"name,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	PublicKey   string `json:"public_key,omitempty"`
}

// sshKeyCreateRequest mirrors go-api-abrha's KeyCreateRequest.
type sshKeyCreateRequest struct {
	Name      string `json:"name"`
	PublicKey string `json:"public_key"`
}

type sshKeyRoot struct {
	SSHKey *sshKeyWire `json:"ssh_key"`
}

type sshKeysRoot struct {
	SSHKeys []sshKeyWire `json:"ssh_keys"`
}

// CreateSSHKey registers a public key with the account. Returns the
// provider-assigned copy, which carries the ID and fingerprint.
func (c *Client) CreateSSHKey(ctx context.Context, creds domain.ProviderCredentials, key domain.SSHKey) (*domain.SSHKey, error) {
	var root sshKeyRoot
	if err := c.doJSON(ctx, creds, "POST", sshKeysBasePath, sshKeyCreateRequest{
		Name:      key.Name,
		PublicKey: key.PublicKey,
	}, &root); err != nil {
		return nil, fmt.Errorf("creating SSH key %q: %w", key.Name, err)
	}
	if root.SSHKey == nil {
		return nil, fmt.Errorf("creating SSH key %q: %w", key.Name, errEmptyResponse)
	}
	return toDomainSSHKey(root.SSHKey), nil
}

// ListSSHKeys returns every key registered with the credentials.
func (c *Client) ListSSHKeys(ctx context.Context, creds domain.ProviderCredentials) ([]domain.SSHKey, error) {
	var root sshKeysRoot
	if err := c.doJSON(ctx, creds, "GET", sshKeysBasePath, nil, &root); err != nil {
		return nil, fmt.Errorf("listing SSH keys: %w", err)
	}

	keys := make([]domain.SSHKey, len(root.SSHKeys))
	for i := range root.SSHKeys {
		keys[i] = *toDomainSSHKey(&root.SSHKeys[i])
	}
	return keys, nil
}

// DeleteSSHKey removes a registered key by provider ID (or fingerprint — the
// provider accepts either).
func (c *Client) DeleteSSHKey(ctx context.Context, creds domain.ProviderCredentials, id string) error {
	if err := c.doJSON(ctx, creds, "DELETE", sshKeysBasePath+"/"+id, nil, nil); err != nil {
		return fmt.Errorf("deleting SSH key %s: %w", id, err)
	}
	return nil
}

// toDomainSSHKey translates a wire key into the shared domain.SSHKey shape.
// The provider's numeric ID becomes the domain string ID.
func toDomainSSHKey(k *sshKeyWire) *domain.SSHKey {
	return &domain.SSHKey{
		ID:          strconv.Itoa(k.ID),
		Name:        k.Name,
		Fingerprint: k.Fingerprint,
		PublicKey:   k.PublicKey,
	}
}
