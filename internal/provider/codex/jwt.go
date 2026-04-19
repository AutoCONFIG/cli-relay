package codex

import (
	"fmt"
	"time"

	"github.com/AutoCONFIG/cli-relay/internal/provider"
)

// extractTokenSet builds a provider.TokenSet from parsed JWT claims and raw tokens.
func extractTokenSet(accessToken, refreshToken, idToken, apiKey string) (*provider.TokenSet, error) {
	ts := &provider.TokenSet{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		IDToken:      idToken,
		APIKey:       apiKey,
		ExtraHeaders: make(map[string]string),
	}

	now := time.Now()
	ts.LastRefresh = &now

	// Parse account_id and exp from the id_token (preferred) or access_token
	tokenStr := idToken
	if tokenStr == "" {
		tokenStr = accessToken
	}
	if tokenStr != "" {
		claims, err := parseJWTClaims(tokenStr)
		if err != nil {
			return nil, fmt.Errorf("parse token claims: %w", err)
		}

		// Extract exp
		if exp, ok := claims["exp"]; ok {
			switch v := exp.(type) {
			case float64:
				t := time.Unix(int64(v), 0)
				ts.ExpiresAt = &t
			}
		}

		// Extract OpenAI-specific claims from https://api.openai.com/auth namespace
		authNS, _ := claims["https://api.openai.com/auth"].(map[string]interface{})
		if authNS != nil {
			if acctID, ok := authNS["chatgpt_account_id"].(string); ok {
				ts.AccountID = acctID
				ts.ExtraHeaders["ChatGPT-Account-ID"] = acctID
			}
			if fedramp, ok := authNS["chatgpt_account_is_fedramp"].(bool); ok && fedramp {
				ts.ExtraHeaders["X-OpenAI-Fedramp"] = "true"
			}
		}
	}

	return ts, nil
}
