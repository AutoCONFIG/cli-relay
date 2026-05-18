package relay

import (
	"sync"
	"time"
)

type affinityEntry struct {
	channelID string
	expiresAt time.Time
}

// AffinityCache maps tokenID:model → channelID with TTL support.
type AffinityCache struct {
	mu      sync.RWMutex
	entries map[string]affinityEntry
}

func NewAffinityCache() *AffinityCache {
	ac := &AffinityCache{
		entries: make(map[string]affinityEntry),
	}
	go ac.cleanup()
	return ac
}

func (ac *AffinityCache) key(tokenID, model string) string {
	return tokenID + ":" + model
}

// Get returns the cached channelID for tokenID+model, or empty string on miss.
// Lazy-deletes expired entries on access.
func (ac *AffinityCache) Get(tokenID, model string) string {
	k := ac.key(tokenID, model)
	ac.mu.RLock()
	e, ok := ac.entries[k]
	if !ok {
		ac.mu.RUnlock()
		return ""
	}
	if time.Now().After(e.expiresAt) {
		ac.mu.RUnlock()
		// Lazy delete: upgrade to write lock
		ac.mu.Lock()
		delete(ac.entries, k)
		ac.mu.Unlock()
		return ""
	}
	ac.mu.RUnlock()
	return e.channelID
}

// Set records an affinity mapping with the given TTL in seconds.
func (ac *AffinityCache) Set(tokenID, model, channelID string, ttlSeconds int) {
	if ttlSeconds <= 0 {
		return
	}
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.entries[ac.key(tokenID, model)] = affinityEntry{
		channelID: channelID,
		expiresAt: time.Now().Add(time.Duration(ttlSeconds) * time.Second),
	}
}

// EvictChannel removes all entries pointing to the given channelID.
func (ac *AffinityCache) EvictChannel(channelID string) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	for k, v := range ac.entries {
		if v.channelID == channelID {
			delete(ac.entries, k)
		}
	}
}

// cleanup runs every 60 seconds to purge expired entries.
func (ac *AffinityCache) cleanup() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		ac.mu.Lock()
		now := time.Now()
		for k, v := range ac.entries {
			if now.After(v.expiresAt) {
				delete(ac.entries, k)
			}
		}
		ac.mu.Unlock()
	}
}
