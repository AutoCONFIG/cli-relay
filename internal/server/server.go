package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/AutoCONFIG/cli-relay/internal/manager"
	"github.com/AutoCONFIG/cli-relay/internal/provider"
)

// Server is the HTTP API server.
type Server struct {
	manager *manager.TokenManager
	logger  *slog.Logger
	mux     *http.ServeMux
}

// New creates a new HTTP server.
func New(m *manager.TokenManager, logger *slog.Logger) *Server {
	s := &Server{
		manager: m,
		logger:  logger,
		mux:     http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/v1/providers", s.handleListProviders)
	s.mux.HandleFunc("GET /api/v1/providers/{name}/token", s.handleGetToken)
	s.mux.HandleFunc("POST /api/v1/providers/{name}/login", s.handleLogin)
	s.mux.HandleFunc("DELETE /api/v1/providers/{name}/token", s.handleLogout)
	s.mux.HandleFunc("POST /api/v1/providers/{name}/refresh", s.handleRefresh)
	s.mux.HandleFunc("POST /api/v1/providers/{name}/recover", s.handleRecover)
}

// Handler returns the HTTP handler.
func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleListProviders(w http.ResponseWriter, _ *http.Request) {
	statuses := s.manager.Status(context.Background())
	writeJSON(w, http.StatusOK, map[string]interface{}{"providers": statuses})
}

func (s *Server) handleGetToken(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing provider name")
		return
	}

	tokens, err := s.manager.GetToken(r.Context(), name)
	if err != nil {
		status := http.StatusNotFound
		if err.Error() != "" {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err.Error())
		return
	}

	resp := tokenResponse{
		AccessToken:  tokens.AccessToken,
		TokenType:    "Bearer",
		AccountID:    tokens.AccountID,
		ExpiresAt:    tokens.ExpiresAt,
		ExtraHeaders: tokens.ExtraHeaders,
	}
	if tokens.APIKey != "" {
		resp.AccessToken = tokens.APIKey
		resp.TokenType = "ApiKey"
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing provider name")
		return
	}

	var req struct {
		Method string `json:"method"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Default to browser if no body
		req.Method = "browser"
	}

	method := provider.AuthMethod(req.Method)
	if method == "" {
		method = provider.AuthMethodBrowser
	}

	tokens, err := s.manager.Login(r.Context(), name, method)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "success",
		"provider":    name,
		"auth_method": method,
		"expires_at":  tokens.ExpiresAt,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing provider name")
		return
	}

	if err := s.manager.Logout(r.Context(), name); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing provider name")
		return
	}

	tokens, err := s.manager.RefreshForce(r.Context(), name)
	if err != nil {
		status := http.StatusInternalServerError
		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "refreshed",
		"expires_at": tokens.ExpiresAt,
	})
}

func (s *Server) handleRecover(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing provider name")
		return
	}

	if err := s.manager.Recover(r.Context(), name); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "recovered"})
}

type tokenResponse struct {
	AccessToken  string            `json:"access_token"`
	TokenType    string            `json:"token_type"`
	AccountID    string            `json:"account_id,omitempty"`
	ExpiresAt    *time.Time        `json:"expires_at,omitempty"`
	ExtraHeaders map[string]string `json:"extra_headers,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
