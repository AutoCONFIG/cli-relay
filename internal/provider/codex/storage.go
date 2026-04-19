package codex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/AutoCONFIG/cli-relay/internal/provider"
)

// authDotJson mirrors the Codex CLI's auth.json format for compatibility.
type authDotJson struct {
	AuthMode       string        `json:"auth_mode"`
	OpenAIAPIKey   *string       `json:"OPENAI_API_KEY"`
	Tokens         *tokenData    `json:"tokens"`
	LastRefresh    *string       `json:"last_refresh,omitempty"`
	AgentIdentity  interface{}   `json:"agent_identity,omitempty"`
}

type tokenData struct {
	IDToken      string `json:"id_token"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	AccountID    string `json:"account_id,omitempty"`
}

// defaultStoragePath returns the default path for auth.json.
func defaultStoragePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex", "auth.json")
}

// storagePath returns the configured or default storage path.
func (p *Provider) storagePath() string {
	if p.cfg.StoragePath != "" {
		return p.cfg.StoragePath
	}
	return defaultStoragePath()
}

// loadFromStorage reads tokens from the Codex CLI's auth.json.
func (p *Provider) loadFromStorage() (*provider.TokenSet, error) {
	path := p.storagePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read auth.json: %w", err)
	}

	var auth authDotJson
	if err := json.Unmarshal(data, &auth); err != nil {
		return nil, fmt.Errorf("parse auth.json: %w", err)
	}

	if auth.Tokens == nil {
		return nil, nil
	}

	// If API key auth, return a simple token set
	if auth.AuthMode == string(AuthModeAPIKey) && auth.OpenAIAPIKey != nil {
		ts := &provider.TokenSet{
			APIKey: *auth.OpenAIAPIKey,
		}
		return ts, nil
	}

	// ChatGPT auth
	ts, err := extractTokenSet(
		auth.Tokens.AccessToken,
		auth.Tokens.RefreshToken,
		auth.Tokens.IDToken,
		derefString(auth.OpenAIAPIKey),
	)
	if err != nil {
		return nil, err
	}

	if auth.Tokens.AccountID != "" {
		ts.AccountID = auth.Tokens.AccountID
		ts.ExtraHeaders["ChatGPT-Account-ID"] = auth.Tokens.AccountID
	}

	// Parse last_refresh
	if auth.LastRefresh != nil {
		if t, err := time.Parse(time.RFC3339, *auth.LastRefresh); err == nil {
			ts.LastRefresh = &t
		}
	}

	return ts, nil
}

// saveToStorage writes tokens to the Codex CLI's auth.json format.
func (p *Provider) saveToStorage(ts *provider.TokenSet) error {
	path := p.storagePath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create auth dir: %w", err)
	}

	auth := authDotJson{
		AuthMode: string(AuthModeChatGPT),
	}

	if ts.APIKey != "" {
		auth.AuthMode = string(AuthModeAPIKey)
		auth.OpenAIAPIKey = &ts.APIKey
	}

	if ts.AccessToken != "" || ts.RefreshToken != "" {
		auth.Tokens = &tokenData{
			IDToken:      ts.IDToken,
			AccessToken:  ts.AccessToken,
			RefreshToken: ts.RefreshToken,
			AccountID:    ts.AccountID,
		}
		if ts.APIKey != "" && auth.AuthMode == string(AuthModeChatGPT) {
			auth.OpenAIAPIKey = &ts.APIKey
		}
	}

	if ts.LastRefresh != nil {
		s := ts.LastRefresh.Format(time.RFC3339)
		auth.LastRefresh = &s
	}

	data, err := json.MarshalIndent(auth, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal auth.json: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write auth.json: %w", err)
	}

	return nil
}

// deleteStorage removes the auth.json file.
func (p *Provider) deleteStorage() error {
	path := p.storagePath()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete auth.json: %w", err)
	}
	return nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
