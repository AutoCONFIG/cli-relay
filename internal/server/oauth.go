package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/AutoCONFIG/cli-relay/internal/manager"
	"github.com/AutoCONFIG/cli-relay/internal/provider"
)

// OAuthProxy handles browser-based OAuth flows that can be initiated remotely
// and have their callbacks proxied through the server.
type OAuthProxy struct {
	manager *manager.TokenManager
	db      hasAdminChecker
	logger  *slog.Logger

	mu      sync.Mutex
	pending map[string]*oauthSession

	cleanupOnce sync.Once
}

type hasAdminChecker interface {
	HasAdmin() (bool, error)
}

type oauthSession struct {
	ProviderName string
	State        string
	CodeVerifier string
	RedirectURL  string
	CreatedAt    time.Time
	Method       provider.AuthMethod
}

// NewOAuthProxy creates a new OAuth proxy handler.
func NewOAuthProxy(m *manager.TokenManager, db hasAdminChecker, logger *slog.Logger) *OAuthProxy {
	return &OAuthProxy{
		manager: m,
		db:      db,
		logger:  logger,
		pending: make(map[string]*oauthSession),
	}
}

// RegisterRoutes adds OAuth proxy routes to the given mux.
func (p *OAuthProxy) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/oauth/init", p.requireAuth(p.handleInit))
	mux.HandleFunc("GET /api/v1/oauth/callback", p.handleCallback)
}

// requireAuth checks admin authentication — reuses the same logic as Server.requireAuth.
func (p *OAuthProxy) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hasAdmin, err := p.db.HasAdmin()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if !hasAdmin {
			// No admin set yet, allow access for first-time setup
			next(w, r)
			return
		}

		token := ""
		if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			token = strings.TrimPrefix(auth, "Bearer ")
		} else {
			if c, err := r.Cookie("session"); err == nil {
				token = c.Value
			}
		}

		// Validate session via the parent server's session store.
		// We need access to the server's validSession method.
		if srv, ok := r.Context().Value(serverContextKey("server")).(*Server); ok {
			if token == "" || !srv.validSession(token) {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
		} else if token == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		next(w, r)
	}
}

// handleInit starts an OAuth flow for a provider and returns the auth URL
// for the user's browser to navigate to.
func (p *OAuthProxy) handleInit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Provider string `json:"provider"`
		Method   string `json:"method"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Provider == "" {
		writeError(w, http.StatusBadRequest, "provider is required")
		return
	}

	method := provider.AuthMethod(req.Method)
	if method == "" {
		method = provider.AuthMethodBrowser
	}

	// Generate state for CSRF protection
	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate state")
		return
	}
	state := hex.EncodeToString(stateBytes)

	// Generate PKCE verifier
	verifierBytes := make([]byte, 64)
	if _, err := rand.Read(verifierBytes); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate verifier")
		return
	}
	codeVerifier := hex.EncodeToString(verifierBytes)

	// Build the callback URL that points back to this server
	scheme := "http"
	if r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}

	// Validate Host header: only allow alphanumeric + dots + hyphens + port
	host := r.Host
	if !isSafeHost(host) {
		writeError(w, http.StatusBadRequest, "invalid host header")
		return
	}

	callbackURL := fmt.Sprintf("%s://%s/api/v1/oauth/callback", scheme, host)

	// Store the session
	session := &oauthSession{
		ProviderName: req.Provider,
		State:        state,
		CodeVerifier: codeVerifier,
		RedirectURL:  callbackURL,
		CreatedAt:    time.Now(),
		Method:       method,
	}

	p.mu.Lock()
	p.pending[state] = session
	p.mu.Unlock()

	// Start periodic cleanup if not already running
	p.cleanupOnce.Do(func() {
		go p.periodicCleanup()
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"state":        state,
		"callback_url": callbackURL,
		"provider":     req.Provider,
		"method":       string(method),
		"expires_in":   300,
	})
}

// handleCallback receives the OAuth callback from the browser.
func (p *OAuthProxy) handleCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	errorParam := r.URL.Query().Get("error")

	if errorParam != "" {
		p.logger.Warn("OAuth callback received error", "error", errorParam)
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `<html><body><h2>Authentication Failed</h2><p>You can close this window.</p></body></html>`)
		return
	}

	if state == "" || code == "" {
		writeError(w, http.StatusBadRequest, "missing state or code")
		return
	}

	p.mu.Lock()
	session, ok := p.pending[state]
	if ok {
		delete(p.pending, state)
	}
	p.mu.Unlock()

	if !ok {
		p.logger.Warn("OAuth callback with unknown state", "state", state)
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `<html><body><h2>Invalid or Expired Session</h2><p>You can close this window.</p></body></html>`)
		return
	}

	ctx := context.WithValue(r.Context(), oauthContextKey("callback"), &oauthCallback{
		Code:         code,
		State:        state,
		CodeVerifier: session.CodeVerifier,
		RedirectURL:  session.RedirectURL,
	})

	tokens, err := p.manager.Login(ctx, session.ProviderName, session.Method)
	if err != nil {
		p.logger.Error("OAuth token exchange failed", "provider", session.ProviderName, "error", err)
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, `<html><body><h2>Authentication Failed</h2><p>Token exchange error. Please try again.</p></body></html>`)
		return
	}

	p.logger.Info("OAuth login successful", "provider", session.ProviderName, "expires_at", tokens.ExpiresAt)
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, `<html><body><h2>Authentication Successful</h2><p>You can close this window and return to CLI Relay.</p><script>window.close()</script></body></html>`)
}

func (p *OAuthProxy) periodicCleanup() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		p.mu.Lock()
		for state, session := range p.pending {
			if time.Since(session.CreatedAt) > 5*time.Minute {
				delete(p.pending, state)
			}
		}
		p.mu.Unlock()
	}
}

// isSafeHost validates that a Host header value is safe to use in a URL.
// Prevents host header injection attacks.
func isSafeHost(host string) bool {
	// Basic validation: host should contain only alphanumeric, dots, hyphens, colons (port)
	for _, c := range host {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '.' || c == '-' || c == ':' || c == '[' || c == ']') {
			return false
		}
	}
	return host != ""
}

// oauthContextKey is used to pass callback data through context.
type oauthContextKey string

// serverContextKey is used to pass server reference through context.
type serverContextKey string

// OAuthCallbackFromContext extracts OAuth callback data from a context.
func OAuthCallbackFromContext(ctx context.Context) *oauthCallback {
	if cb, ok := ctx.Value(oauthContextKey("callback")).(*oauthCallback); ok {
		return cb
	}
	return nil
}

type oauthCallback struct {
	Code         string
	State        string
	CodeVerifier string
	RedirectURL  string
}

// IsProxiedCallback returns true when the login flow is being driven by
// the OAuth proxy (i.e., the authorization code came from a proxied callback).
func IsProxiedCallback(ctx context.Context) bool {
	return OAuthCallbackFromContext(ctx) != nil
}

// BuildProxiedAuthURL constructs an authorization URL with the proxy's callback URL
// and PKCE parameters. Providers should use this when running in proxy mode.
func BuildProxiedAuthURL(baseAuthURL, clientID, callbackURL, state, codeChallenge, scopes string) string {
	v := url.Values{}
	v.Set("response_type", "code")
	v.Set("client_id", clientID)
	v.Set("redirect_uri", callbackURL)
	v.Set("scope", scopes)
	v.Set("state", state)
	if codeChallenge != "" {
		v.Set("code_challenge_method", "S256")
		v.Set("code_challenge", codeChallenge)
	}

	if strings.Contains(baseAuthURL, "?") {
		return baseAuthURL + "&" + v.Encode()
	}
	return baseAuthURL + "?" + v.Encode()
}
