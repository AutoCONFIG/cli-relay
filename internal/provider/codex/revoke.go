package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/AutoCONFIG/cli-relay/internal/provider"
)

// Revoke revokes the tokens at the OpenAI auth server.
// Best-effort: errors are returned but should not block logout.
func (p *Provider) Revoke(ctx context.Context, tokens *provider.TokenSet) error {
	if tokens == nil {
		return nil
	}

	// Prefer to revoke refresh token
	token := tokens.RefreshToken
	tokenTypeHint := "refresh_token"

	// Fall back to access token if no refresh token
	if token == "" {
		token = tokens.AccessToken
		tokenTypeHint = "access_token"
	}

	if token == "" {
		return nil
	}

	reqBody := map[string]string{
		"token":           token,
		"token_type_hint": tokenTypeHint,
	}

	// Include client_id only when revoking refresh token (upstream behavior)
	if tokenTypeHint == "refresh_token" {
		reqBody["client_id"] = ClientID
	}

	body, _ := json.Marshal(reqBody)

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.cfg.Issuer+"/oauth/revoke", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create revoke request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("revoke request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("revoke failed (%d)", resp.StatusCode)
	}

	return nil
}
