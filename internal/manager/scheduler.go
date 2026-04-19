package manager

import (
	"context"
	"log/slog"
	"time"

	"github.com/AutoCONFIG/cli-relay/internal/config"
)

// Scheduler runs background token refresh checks.
type Scheduler struct {
	manager       *TokenManager
	checkInterval time.Duration
	maxRetries    int
	retryBackoff  time.Duration
	logger        *slog.Logger

	stopCh chan struct{}
}

// NewScheduler creates a new scheduler.
func NewScheduler(manager *TokenManager, cfg config.RefreshConfig, logger *slog.Logger) *Scheduler {
	interval := cfg.CheckInterval
	if interval == 0 {
		interval = 1 * time.Minute
	}
	retries := cfg.MaxRetries
	if retries == 0 {
		retries = 3
	}
	backoff := cfg.RetryBackoff
	if backoff == 0 {
		backoff = 30 * time.Second
	}

	return &Scheduler{
		manager:       manager,
		checkInterval: interval,
		maxRetries:    retries,
		retryBackoff:  backoff,
		logger:        logger,
		stopCh:        make(chan struct{}),
	}
}

// Start begins the background refresh loop.
func (s *Scheduler) Start(ctx context.Context) {
	s.logger.Info("starting token refresh scheduler", "interval", s.checkInterval)

	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	// Initial check on start
	s.checkAndRefresh(ctx)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("scheduler stopped by context")
			return
		case <-s.stopCh:
			s.logger.Info("scheduler stopped")
			return
		case <-ticker.C:
			s.checkAndRefresh(ctx)
		}
	}
}

// Stop gracefully shuts down the scheduler.
func (s *Scheduler) Stop() {
	close(s.stopCh)
}

func (s *Scheduler) checkAndRefresh(ctx context.Context) {
	for name := range s.manager.providers {
		if ctx.Err() != nil {
			return
		}

		tokens, err := s.manager.store.Load(ctx, name)
		if err != nil || tokens == nil || tokens.IsEmpty() {
			continue
		}

		if tokens.NeedsRefresh(s.manager.proactiveRefreshAge(name)) {
			s.logger.Info("proactive refresh needed", "provider", name)

			_, err := s.manager.RefreshForce(ctx, name)
			if err != nil {
				s.logger.Error("proactive refresh failed",
					"provider", name, "error", err)
			} else {
				s.logger.Info("proactive refresh succeeded", "provider", name)
			}
		}
	}
}
