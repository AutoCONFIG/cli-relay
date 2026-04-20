package codex

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/AutoCONFIG/cli-relay/internal/provider"
)

const (
	// ClientID is the OpenAI Codex OAuth client identifier.
	ClientID = "app_EMoamEEZ73f0CkXaXp7hrann"

	// DefaultIssuer is the default OpenAI auth issuer URL.
	DefaultIssuer = "https://auth.openai.com"

	// DefaultCallbackPort is the default local OAuth callback port.
	DefaultCallbackPort = 1455

	// OAuthScopes is the space-separated list of OAuth scopes.
	OAuthScopes = "openid profile email offline_access api.connectors.read api.connectors.invoke"

	// ProactiveRefreshAge matches upstream Codex: refresh if last refresh > 8 days.
	ProactiveRefreshAge = 192 * time.Hour

	// RefreshBuffer refreshes this long before actual expiration.
	RefreshBuffer = 5 * time.Minute

	// DeviceCodeTimeout is the max wait for device code authorization.
	DeviceCodeTimeout = 15 * time.Minute
)

// AuthMode represents the stored auth_mode in auth.json.
type AuthMode string

const (
	AuthModeChatGPT       AuthMode = "chatgpt"
	AuthModeAPIKey        AuthMode = "api_key"
	AuthModeChatGPTTokens AuthMode = "chatgpt_auth_tokens"
)

// Config holds Codex-specific configuration.
type Config struct {
	Issuer      string
	StoragePath string // path to auth.json; defaults to ~/.codex/auth.json
}

// Provider implements provider.Provider for OpenAI Codex.
type Provider struct {
	cfg    Config
	client *http.Client
}

// New creates a new Codex provider.
func New(cfg Config) *Provider {
	if cfg.Issuer == "" {
		cfg.Issuer = DefaultIssuer
	}
	return &Provider{
		cfg: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (p *Provider) Name() string {
	return "codex"
}

func (p *Provider) SupportedMethods() []provider.AuthMethod {
	return []provider.AuthMethod{
		provider.AuthMethodBrowser,
		provider.AuthMethodDeviceCode,
		provider.AuthMethodAPIKey,
	}
}

// parseJWTClaims extracts claims from a JWT without verifying the signature.
// Token validation is handled by the upstream auth server.
func parseJWTClaims(tokenStr string) (map[string]interface{}, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}
	payload, err := base64urlDecode(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode JWT payload: %w", err)
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("parse JWT claims: %w", err)
	}
	return claims, nil
}

func base64urlDecode(s string) ([]byte, error) {
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	return base64.URLEncoding.DecodeString(s)
}
