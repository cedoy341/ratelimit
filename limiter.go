// Copyright 2024 Google LLC
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package ratelimit

import (
	"runtime"
	"sync"
	"time"
)

// Limiter manages a collection of token buckets, automatically cleaning up
// buckets that have not been used for a specified Time-To-Live (TTL) duration.
type Limiter struct {
	buckets map[string]*managedBucket
	mu      sync.Mutex
	clock   Clock
	ttl     time.Duration

	// newBucket is a factory function that creates a new bucket when a key is first seen.
	newBucket func() *Bucket

	// stop is used to signal the cleanup goroutine to terminate.
	stop chan struct{}
}

// managedBucket wraps a Bucket with its last access time to track its usage for TTL.
type managedBucket struct {
	bucket     *Bucket
	lastAccess time.Time
}

// NewLimiter creates a new Limiter that manages token buckets.
// It takes a ttl for expiring unused buckets and a factory function (`newBucket`)
// to create new buckets with a specific configuration (e.g., rate and capacity).
// The cleanup process runs in a background goroutine, checking for expired
// buckets every ttl/2.
func NewLimiter(ttl time.Duration, newBucket func() *Bucket) *Limiter {
	return NewLimiterWithClock(ttl, newBucket, nil)
}

// NewLimiterWithClock is like NewLimiter but allows providing a custom clock,
// which is useful for testing. If the provided clock is nil, it defaults to the real system clock.
func NewLimiterWithClock(ttl time.Duration, newBucket func() *Bucket, clock Clock) *Limiter {
	if clock == nil {
		clock = &realClock{}
	}
	l := &Limiter{
		buckets:   make(map[string]*managedBucket),
		clock:     clock,
		ttl:       ttl,
		newBucket: newBucket,
		stop:      make(chan struct{}),
	}

	// Start the background goroutine to periodically clean up expired buckets.
	// The cleanup interval is set to half of the TTL for timely removal.
	go l.cleanupLoop(ttl / 2)

	// A finalizer is set to ensure the cleanup goroutine is stopped when the
	// Limiter is garbage collected, preventing goroutine leaks.
	runtime.SetFinalizer(l, (*Limiter).Stop)
	return l
}

// Get returns the token bucket for the given key.
// If a bucket for the key does not exist, a new one is created using the
// factory function provided to NewLimiter. Each call to Get updates the
// bucket's last access time, keeping it alive.
func (l *Limiter) Get(key string) *Bucket {
	l.mu.Lock()
	defer l.mu.Unlock()

	mb, ok := l.buckets[key]
	if !ok {
		mb = &managedBucket{
			bucket: l.newBucket(),
		}
		l.buckets[key] = mb
	}
	// Update the last access time on every retrieval.
	mb.lastAccess = l.clock.Now()
	return mb.bucket
}

// Stop terminates the background cleanup goroutine. It is called automatically
// when the Limiter is garbage collected but can be invoked manually to stop it
// sooner. It is safe to call Stop multiple times.
func (l *Limiter) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()
	// Ensure stop is only closed once.
	if l.stop != nil {
		close(l.stop)
		l.stop = nil
	}
}

// cleanupLoop runs the cleanup cycle at the given interval until Stop is called.
func (l *Limiter) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			l.cleanup()
		case <-l.stop:
			return
		}
	}
}

// cleanup iterates through the managed buckets and removes any that have
// expired (i.e., not been accessed for the TTL duration).
func (l *Limiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.clock.Now()
	for key, mb := range l.buckets {
		if now.Sub(mb.lastAccess) > l.ttl {
			delete(l.buckets, key)
		}
	}
}
