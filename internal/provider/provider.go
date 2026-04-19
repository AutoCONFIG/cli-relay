package provider

import (
	"context"
	"time"
)

// AuthMethod enumerates the interactive login mechanisms a provider supports.
type AuthMethod string

const (
	AuthMethodBrowser    AuthMethod = "browser"
	AuthMethodDeviceCode AuthMethod = "device_code"
	AuthMethodAPIKey     AuthMethod = "api_key"
)

// TokenSet is the universal token representation across all providers.
// Not every field is populated by every provider.
type TokenSet struct {
	AccessToken  string            `json:"access_token"`
	RefreshToken string            `json:"refresh_token,omitempty"`
	IDToken      string            `json:"id_token,omitempty"`
	APIKey       string            `json:"api_key,omitempty"`
	AccountID    string            `json:"account_id,omitempty"`
	ExpiresAt    *time.Time        `json:"expires_at,omitempty"`
	Scopes       []string          `json:"scopes,omitempty"`
	LastRefresh  *time.Time        `json:"last_refresh,omitempty"`
	ExtraHeaders map[string]string `json:"extra_headers,omitempty"`
}

// IsExpired returns true if the access token has a known expiration that has passed.
func (t *TokenSet) IsExpired() bool {
	if t.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*t.ExpiresAt)
}

// NeedsRefresh returns true if proactive refresh should be attempted:
// either the token is expired, or last refresh was more than maxAge ago.
func (t *TokenSet) NeedsRefresh(maxAge time.Duration) bool {
	if t.IsExpired() {
		return true
	}
	if t.LastRefresh != nil && time.Since(*t.LastRefresh) > maxAge {
		return true
	}
	return false
}

// IsEmpty returns true if the token set has no usable credentials.
func (t *TokenSet) IsEmpty() bool {
	return t.AccessToken == "" && t.RefreshToken == "" && t.APIKey == ""
}

// Provider is the interface each CLI auth system implements.
type Provider interface {
	// Name returns a unique identifier for this provider (e.g., "codex", "gemini").
	Name() string

	// SupportedMethods returns the interactive auth methods this provider can perform.
	SupportedMethods() []AuthMethod

	// Login initiates an interactive login flow.
	Login(ctx context.Context, method AuthMethod) (*TokenSet, error)

	// Refresh attempts to refresh the current tokens using a refresh token.
	Refresh(ctx context.Context, tokens *TokenSet) (*TokenSet, error)

	// Validate checks whether the given tokens are still valid.
	Validate(ctx context.Context, tokens *TokenSet) (bool, error)

	// Revoke revokes the given tokens at the authorization server.
	Revoke(ctx context.Context, tokens *TokenSet) error
}
