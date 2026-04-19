package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/AutoCONFIG/cli-relay/internal/provider"
)

// deviceCodeResponse is the response from the usercode request.
type deviceCodeResponse struct {
	DeviceAuthID string `json:"device_auth_id"`
	UserCode     string `json:"user_code"`
	Interval     string `json:"interval"` // string per upstream, parsed to duration
}

// deviceTokenResponse is the response from the device token poll.
type deviceTokenResponse struct {
	AuthorizationCode string `json:"authorization_code"`
	CodeChallenge     string `json:"code_challenge"`
	CodeVerifier      string `json:"code_verifier"`
}

// LoginDeviceCode performs the device code authorization flow.
func (p *Provider) LoginDeviceCode(ctx context.Context) (*provider.TokenSet, error) {
	// Step 1: Request user code
	reqBody, _ := json.Marshal(map[string]string{"client_id": ClientID})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.cfg.Issuer+"/api/accounts/deviceauth/usercode", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create usercode request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request usercode: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("usercode request failed (%d): %s", resp.StatusCode, body)
	}

	var dcResp deviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&dcResp); err != nil {
		return nil, fmt.Errorf("decode usercode response: %w", err)
	}

	fmt.Printf("\nTo authenticate, visit: %s/codex/device\n", p.cfg.Issuer)
	fmt.Printf("Enter code: %s\n\n", dcResp.UserCode)

	// Step 2: Poll for authorization
	pollInterval := 5 * time.Second
	if dcResp.Interval != "" {
		if d, err := time.ParseDuration(dcResp.Interval + "s"); err == nil {
			pollInterval = d
		}
	}

	timeout := time.After(DeviceCodeTimeout)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// First poll immediately
	for {
		select {
		case <-timeout:
			return nil, fmt.Errorf("device code authorization timed out")
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			result, err := p.pollDeviceToken(ctx, dcResp.DeviceAuthID, dcResp.UserCode)
			if err != nil {
				continue // transient error, keep polling
			}
			if result == nil {
				continue // still pending
			}

			// Step 3: Exchange authorization code for tokens
			redirectURI := p.cfg.Issuer + "/deviceauth/callback"
			return p.exchangeCodeForTokensRaw(ctx, result.AuthorizationCode,
				redirectURI, result.CodeVerifier)
		}
	}
}

func (p *Provider) pollDeviceToken(ctx context.Context, deviceAuthID, userCode string) (*deviceTokenResponse, error) {
	reqBody, _ := json.Marshal(map[string]string{
		"device_auth_id": deviceAuthID,
		"user_code":      userCode,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.cfg.Issuer+"/api/accounts/deviceauth/token", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var result deviceTokenResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, err
		}
		return &result, nil
	case http.StatusForbidden, http.StatusNotFound:
		return nil, nil // still pending
	default:
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
}

// exchangeCodeForTokens exchanges an authorization code for tokens using PKCE.
func (p *Provider) exchangeCodeForTokens(ctx context.Context, code, redirectURI, codeVerifier string) (*provider.TokenSet, error) {
	return p.exchangeCodeForTokensRaw(ctx, code, redirectURI, codeVerifier)
}

func (p *Provider) exchangeCodeForTokensRaw(ctx context.Context, code, redirectURI, codeVerifier string) (*provider.TokenSet, error) {
	// Step 1: Exchange code for tokens
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {ClientID},
		"code_verifier": {codeVerifier},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.cfg.Issuer+"/oauth/token", bytes.NewBufferString(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create token exchange request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token exchange request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, body)
	}

	var tokenResp struct {
		IDToken      string `json:"id_token"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}

	// Step 2: Token-exchange grant to get API-key-style token
	apiKey, err := p.obtainAPIKey(ctx, tokenResp.IDToken)
	if err != nil {
		// Non-fatal: the API key is optional, access_token still works
		fmt.Printf("Warning: token-exchange grant failed: %v\n", err)
	}

	// Step 3: Build TokenSet
	ts, err := extractTokenSet(tokenResp.AccessToken, tokenResp.RefreshToken,
		tokenResp.IDToken, apiKey)
	if err != nil {
		return nil, err
	}

	return ts, nil
}

// obtainAPIKey performs the token-exchange grant to get an API-key-style access token.
func (p *Provider) obtainAPIKey(ctx context.Context, idToken string) (string, error) {
	form := url.Values{
		"grant_type":           {"urn:ietf:params:oauth:grant-type:token-exchange"},
		"client_id":            {ClientID},
		"requested_token":      {"openai-api-key"},
		"subject_token":        {idToken},
		"subject_token_type":   {"urn:ietf:params:oauth:token-type:id_token"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.cfg.Issuer+"/oauth/token", bytes.NewBufferString(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("create token-exchange request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token-exchange request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token-exchange failed (%d): %s", resp.StatusCode, body)
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode token-exchange response: %w", err)
	}

	return result.AccessToken, nil
}
