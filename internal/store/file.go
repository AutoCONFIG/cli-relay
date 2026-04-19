package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/AutoCONFIG/cli-relay/internal/provider"
)

// FileStore implements TokenStore using a JSON file per provider.
type FileStore struct {
	baseDir string
	mu      sync.Mutex
}

// NewFileStore creates a FileStore that stores files under baseDir.
func NewFileStore(baseDir string) (*FileStore, error) {
	if err := os.MkdirAll(baseDir, 0700); err != nil {
		return nil, fmt.Errorf("create store dir: %w", err)
	}
	return &FileStore{baseDir: baseDir}, nil
}

func (s *FileStore) path(providerName string) string {
	return filepath.Join(s.baseDir, providerName+".json")
}

func (s *FileStore) Load(_ context.Context, providerName string) (*provider.TokenSet, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path(providerName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read token file: %w", err)
	}

	var ts provider.TokenSet
	if err := json.Unmarshal(data, &ts); err != nil {
		return nil, fmt.Errorf("unmarshal token file: %w", err)
	}
	return &ts, nil
}

func (s *FileStore) Save(_ context.Context, providerName string, tokens *provider.TokenSet) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(tokens, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal tokens: %w", err)
	}

	path := s.path(providerName)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write token file: %w", err)
	}
	return nil
}

func (s *FileStore) Delete(_ context.Context, providerName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.path(providerName)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete token file: %w", err)
	}
	return nil
}
