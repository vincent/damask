package demo

import (
	"context"
	"log"
	"math/rand"
	"time"
)

// resetting is a package-level flag used to signal that a reset is in progress.
// It is set by StartResetLoop and read by IsResetting.
var resettingCh = make(chan struct{}, 1)

// IsResetting reports whether a demo reset is currently in progress.
// Handlers use this to return 503 during the reset window.
func (s *Seeder) IsResetting() bool {
	select {
	case <-resettingCh:
		resettingCh <- struct{}{} // put it back
		return true
	default:
		return false
	}
}

// StartResetLoop runs the demo reset on a ticker in a background goroutine.
// Call this once from main.go when DEMO_MODE=true.
// A random jitter of 0–15 minutes is applied to the first tick to avoid
// top-of-hour resets.
func (s *Seeder) StartResetLoop(ctx context.Context) {
	interval := time.Duration(s.cfg.ResetIntervalHours) * time.Hour

	// Apply jitter: wait 0–15 min before the first scheduled reset
	jitter := time.Duration(rand.Intn(15)) * time.Minute //nolint:gosec

	go func() {
		log.Printf("demo: reset loop started interval=%v jitter=%v", interval, jitter)

		select {
		case <-time.After(jitter):
		case <-ctx.Done():
			return
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.runReset(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

// runReset wipes and reseeds the demo workspace, with retry on failure.
func (s *Seeder) runReset(ctx context.Context) {
	log.Printf("demo: reset started")
	start := time.Now()

	// Signal that a reset is in progress
	select {
	case resettingCh <- struct{}{}:
		defer func() { <-resettingCh }()
	default:
		// Already resetting
		return
	}

	if err := s.Wipe(ctx); err != nil {
		log.Printf("demo: reset failed step=wipe error=%v", err)
		s.scheduleRetry(ctx, 30*time.Minute)
		return
	}

	if err := s.Seed(ctx); err != nil {
		log.Printf("demo: reset failed step=seed error=%v", err)
		s.scheduleRetry(ctx, 30*time.Minute)
		return
	}

	log.Printf("demo: reset complete total_duration_ms=%d", time.Since(start).Milliseconds())
}

// scheduleRetry schedules a single retry after the given delay.
func (s *Seeder) scheduleRetry(ctx context.Context, delay time.Duration) {
	log.Printf("demo: retry scheduled in=%v", delay)
	go func() {
		select {
		case <-time.After(delay):
			log.Printf("demo: reset started (retry)")
			start := time.Now()

			// Check if reset is already in progress
			select {
			case resettingCh <- struct{}{}:
				defer func() { <-resettingCh }()
			default:
				return
			}

			if err := s.Wipe(ctx); err != nil {
				log.Printf("demo: retry failed step=wipe error=%v", err)
				return
			}
			if err := s.Seed(ctx); err != nil {
				log.Printf("demo: retry failed step=seed error=%v", err)
				return
			}
			log.Printf("demo: retry complete total_duration_ms=%d", time.Since(start).Milliseconds())
		case <-ctx.Done():
		}
	}()
}
