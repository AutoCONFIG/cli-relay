package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/AutoCONFIG/cli-relay/internal/provider"
)

// RefreshError represents a classified token refresh error.
type RefreshError struct {
	IsPermanent bool
	Reason      string // "expired", "reused", "revoked", "other", "transient"
	StatusCode  int
	Message     string
}

func (e *RefreshError) Error() string {
	return fmt.Sprintf("refresh error (%s): %s", e.Reason, e.Message)
}

// Permanent satisfies the permanentError interface in the manager package.
func (e *RefreshError) Permanent() bool { return e.IsPermanent }

// Refresh attempts to refresh the access token using the refresh token.
func (p *Provider) Refresh(ctx context.Context, tokens *provider.TokenSet) (*provider.TokenSet, error) {
	if tokens.RefreshToken == "" {
		return nil, &RefreshError{
			IsPermanent: true,
			Reason:    "expired",
			Message:   "no refresh token available",
		}
	}

	reqBody, _ := json.Marshal(map[string]string{
		"client_id":     ClientID,
		"grant_type":    "refresh_token",
		"refresh_token": tokens.RefreshToken,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.cfg.Issuer+"/oauth/token", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, &RefreshError{
			IsPermanent:  false,
			Reason:     "transient",
			Message:    err.Error(),
			StatusCode: 0,
		}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, classifyRefreshError(body)
	}

	if resp.StatusCode != http.StatusOK {
		reason := "transient"
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			reason = "other"
		}
		return nil, &RefreshError{
			IsPermanent:  resp.StatusCode >= 400 && resp.StatusCode < 500,
			Reason:     reason,
			Message:    fmt.Sprintf("unexpected status %d: %s", resp.StatusCode, body),
			StatusCode: resp.StatusCode,
		}
	}

	var tokenResp struct {
		IDToken      string `json:"id_token"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("decode refresh response: %w", err)
	}

	// Merge: only update non-empty fields (upstream behavior)
	accessToken := tokens.AccessToken
	if tokenResp.AccessToken != "" {
		accessToken = tokenResp.AccessToken
	}
	refreshToken := tokens.RefreshToken
	if tokenResp.RefreshToken != "" {
		refreshToken = tokenResp.RefreshToken
	}
	idToken := tokens.IDToken
	if tokenResp.IDToken != "" {
		idToken = tokenResp.IDToken
	}

	// Re-obtain API key if we got a new id_token
	apiKey := tokens.APIKey
	if tokenResp.IDToken != "" {
		if newKey, err := p.obtainAPIKey(ctx, tokenResp.IDToken); err == nil {
			apiKey = newKey
		}
	}

	ts, err := extractTokenSet(accessToken, refreshToken, idToken, apiKey)
	if err != nil {
		return nil, err
	}

	return ts, nil
}

// Validate checks if the current tokens are still valid.
func (p *Provider) Validate(_ context.Context, tokens *provider.TokenSet) (bool, error) {
	if tokens == nil || tokens.IsEmpty() {
		return false, nil
	}
	// If we have an API key, it doesn't expire
	if tokens.APIKey != "" && tokens.AccessToken == "" {
		return true, nil
	}
	// Check JWT expiration
	return !tokens.IsExpired(), nil
}

func classifyRefreshError(body []byte) *RefreshError {
	var resp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	json.Unmarshal(body, &resp)

	switch resp.Error.Code {
	case "refresh_token_expired":
		return &RefreshError{IsPermanent: true, Reason: "expired", Message: resp.Error.Code}
	case "refresh_token_reused":
		return &RefreshError{IsPermanent: true, Reason: "reused", Message: resp.Error.Code}
	case "refresh_token_invalidated":
		return &RefreshError{IsPermanent: true, Reason: "revoked", Message: resp.Error.Code}
	default:
		return &RefreshError{IsPermanent: true, Reason: "other", Message: string(body)}
	}
}

// IsPermanentRefreshError checks if an error is a permanent refresh failure.
func IsPermanentRefreshError(err error) bool {
	if re, ok := err.(*RefreshError); ok {
		return re.IsPermanent
	}
	return false
}
