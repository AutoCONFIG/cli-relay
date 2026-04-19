package store

import (
	"context"

	"github.com/AutoCONFIG/cli-relay/internal/provider"
)

// TokenStore persists and retrieves token sets keyed by provider name.
type TokenStore interface {
	// Load reads the stored tokens for a provider.
	// Returns (nil, nil) if no tokens are stored.
	Load(ctx context.Context, providerName string) (*provider.TokenSet, error)

	// Save persists tokens for a provider.
	Save(ctx context.Context, providerName string, tokens *provider.TokenSet) error

	// Delete removes stored tokens for a provider.
	Delete(ctx context.Context, providerName string) error
}
