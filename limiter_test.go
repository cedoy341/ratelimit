// Copyright 2024 Google LLC
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package ratelimit

import (
	"testing"
	"time"
)

// TestLimiter verifies the core functionality of the Limiter, including
// bucket creation, TTL-based cleanup, and access time updates.
func TestLimiter(t *testing.T) {
	clock := &mockClock{now: time.Now()}
	ttl := 10 * time.Second

	// Factory for creating new buckets for the test.
	newBucket := func() *Bucket {
		// Using NewBucketWithClock to inject the mock clock into the buckets as well.
		return NewBucketWithClock(time.Second, 10, clock)
	}

	limiter := NewLimiterWithClock(ttl, newBucket, clock)
	defer limiter.Stop()

	// 1. Get a bucket for key "a"
	b1 := limiter.Get("a")
	if b1 == nil {
		t.Fatal("Expected a bucket for key 'a', got nil")
	}

	// Check that it's in the map
	limiter.mu.Lock()
	if _, ok := limiter.buckets["a"]; !ok {
		limiter.mu.Unlock()
		t.Fatal(`bucket "a" not found in limiter's map after Get()`)
	}
	limiter.mu.Unlock()

	// 2. Advance time by less than TTL
	clock.advance(ttl / 2)

	// Get another bucket for key "b"
	_ = limiter.Get("b")

	// Run cleanup manually to test its logic
	limiter.cleanup()

	// Both buckets should still be there
	limiter.mu.Lock()
	if _, ok := limiter.buckets["a"]; !ok {
		limiter.mu.Unlock()
		t.Fatal(`bucket "a" should not have been cleaned up yet`)
	}
	if _, ok := limiter.buckets["b"]; !ok {
		limiter.mu.Unlock()
		t.Fatal(`bucket "b" should not have been cleaned up yet`)
	}
	limiter.mu.Unlock()

	// 3. Advance time so that "a" expires but "b" does not
	clock.advance(ttl/2 + time.Second) // Total time elapsed since "a" was accessed: ttl + 1s

	// Access bucket "b" again to update its lastAccess time
	_ = limiter.Get("b")

	// Run cleanup
	limiter.cleanup()

	// Bucket "a" should be gone, "b" should remain
	limiter.mu.Lock()
	if _, ok := limiter.buckets["a"]; ok {
		limiter.mu.Unlock()
		t.Fatal(`bucket "a" should have been cleaned up`)
	}
	if _, ok := limiter.buckets["b"]; !ok {
		limiter.mu.Unlock()
		t.Fatal(`bucket "b" should not have been cleaned up`)
	}
	limiter.mu.Unlock()

	// 4. Advance time so that "b" also expires
	clock.advance(ttl + time.Second)

	// Run cleanup
	limiter.cleanup()

	// Bucket "b" should now be gone
	limiter.mu.Lock()
	if len(limiter.buckets) != 0 {
		limiter.mu.Unlock()
		t.Fatalf("Expected all buckets to be cleaned up, but %d remain", len(limiter.buckets))
	}
	limiter.mu.Unlock()
}

// TestLimiterStop verifies that the cleanup goroutine can be stopped correctly.
func TestLimiterStop(t *testing.T) {
	clock := &mockClock{now: time.Now()}
	ttl := 10 * time.Second
	newBucket := func() *Bucket {
		return NewBucketWithClock(time.Second, 10, clock)
	}

	limiter := NewLimiterWithClock(ttl, newBucket, clock)
	// Stop it immediately
	limiter.Stop()

	// Check if the stop channel is closed and nilled out
	limiter.mu.Lock()
	if limiter.stop != nil {
		limiter.mu.Unlock()
		t.Error("limiter.stop should be nil after Stop() is called")
	}
	limiter.mu.Unlock()

	// A second stop should not cause a panic (e.g., closing a closed channel)
	limiter.Stop()
}
