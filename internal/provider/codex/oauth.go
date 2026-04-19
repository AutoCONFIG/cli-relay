package codex

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"time"

	"github.com/AutoCONFIG/cli-relay/internal/provider"
)

// pkceCodes holds PKCE code verifier and challenge.
type pkceCodes struct {
	Verifier  string
	Challenge string
}

func generatePKCE() (*pkceCodes, error) {
	verifierBytes := make([]byte, 64)
	if _, err := rand.Read(verifierBytes); err != nil {
		return nil, fmt.Errorf("generate PKCE verifier: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(verifierBytes)

	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])

	return &pkceCodes{Verifier: verifier, Challenge: challenge}, nil
}

func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// LoginBrowser performs the browser-based OAuth2+PKCE flow.
func (p *Provider) LoginBrowser(ctx context.Context) (*provider.TokenSet, error) {
	pkce, err := generatePKCE()
	if err != nil {
		return nil, err
	}

	state, err := generateState()
	if err != nil {
		return nil, err
	}

	// Start local callback server
	callbackPath := "/auth/callback"
	mux := http.NewServeMux()
	resultCh := make(chan *callbackResult, 1)

	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}
		// Redirect user to success page
		w.Header().Set("Location", "https://auth.openai.com/success")
		w.WriteHeader(http.StatusFound)
		resultCh <- &callbackResult{code: code}
	})

	server := &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", DefaultCallbackPort), Handler: mux}
	go server.ListenAndServe()
	defer server.Close()

	redirectURI := fmt.Sprintf("http://localhost:%d%s", DefaultCallbackPort, callbackPath)

	// Build authorization URL
	authURL, err := url.Parse(p.cfg.Issuer + "/oauth/authorize")
	if err != nil {
		return nil, fmt.Errorf("parse auth URL: %w", err)
	}
	q := authURL.Query()
	q.Set("response_type", "code")
	q.Set("client_id", ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", OAuthScopes)
	q.Set("code_challenge", pkce.Challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("state", state)
	q.Set("id_token_add_organizations", "true")
	q.Set("codex_cli_simplified_flow", "true")
	q.Set("originator", "codex_cli_rs")
	authURL.RawQuery = q.Encode()

	// Open browser
	if err := openBrowser(authURL.String()); err != nil {
		fmt.Printf("Please open this URL in your browser:\n%s\n", authURL.String())
	}

	// Wait for callback with timeout
	select {
	case result := <-resultCh:
		// Exchange code for tokens
		return p.exchangeCodeForTokens(ctx, result.code, redirectURI, pkce.Verifier)
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("browser login timed out after 5 minutes")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type callbackResult struct {
	code string
}

func (p *Provider) Login(ctx context.Context, method provider.AuthMethod) (*provider.TokenSet, error) {
	switch method {
	case provider.AuthMethodBrowser:
		return p.LoginBrowser(ctx)
	case provider.AuthMethodDeviceCode:
		return p.LoginDeviceCode(ctx)
	case provider.AuthMethodAPIKey:
		return p.LoginAPIKey(ctx)
	default:
		return nil, fmt.Errorf("unsupported auth method: %s", method)
	}
}

func (p *Provider) LoginAPIKey(_ context.Context) (*provider.TokenSet, error) {
	// API key login is handled by the manager layer reading from config/env
	return nil, fmt.Errorf("API key login requires setting the key via config or OPENAI_API_KEY env var")
}

func openBrowser(url string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	default:
		cmd = "xdg-open"
	}
	return exec.Command(cmd, append(args, url)...).Start()
}
