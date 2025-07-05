// Copyright 2014 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package ratelimit

import (
	"math"
	"testing"
	"time"
)

// mockClock is a fake clock for testing.
type mockClock struct {
	now time.Time
}

func (c *mockClock) Now() time.Time {
	return c.now
}

func (c *mockClock) Sleep(d time.Duration) {
	c.now = c.now.Add(d)
}

func (c *mockClock) advance(d time.Duration) {
	c.now = c.now.Add(d)
}

func newTestBucket(fillInterval time.Duration, capacity int64, quantum int64) (*Bucket, *mockClock) {
	clock := &mockClock{now: time.Now()}
	return NewBucketWithQuantumAndClock(fillInterval, capacity, quantum, clock), clock
}

func TestTake(t *testing.T) {
	tests := []struct {
		about        string
		fillInterval time.Duration
		capacity     int64
		reqs         []takeReq
	}{{
		about:        "serial requests",
		fillInterval: 250 * time.Millisecond,
		capacity:     10,
		reqs: []takeReq{
			{time: 0, count: 0, expectWait: 0},
			{time: 0, count: 10, expectWait: 0},
			{time: 0, count: 1, expectWait: 250 * time.Millisecond},
			{time: 250 * time.Millisecond, count: 1, expectWait: 250 * time.Millisecond},
		},
	}, {
		about:        "concurrent requests",
		fillInterval: 250 * time.Millisecond,
		capacity:     10,
		reqs: []takeReq{
			{time: 0, count: 10, expectWait: 0},
			{time: 0, count: 2, expectWait: 500 * time.Millisecond},
			{time: 0, count: 2, expectWait: 1000 * time.Millisecond},
			{time: 0, count: 1, expectWait: 1250 * time.Millisecond},
		},
	}, {
		about:        "more than capacity",
		fillInterval: 1 * time.Millisecond,
		capacity:     10,
		reqs: []takeReq{
			{time: 0, count: 10, expectWait: 0},
			{time: 20 * time.Millisecond, count: 15, expectWait: 5 * time.Millisecond},
		},
	}, {
		about:        "sub-quantum time",
		fillInterval: 10 * time.Millisecond,
		capacity:     10,
		reqs: []takeReq{
			{time: 0, count: 10, expectWait: 0},
			{time: 7 * time.Millisecond, count: 1, expectWait: 3 * time.Millisecond},
			{time: 8 * time.Millisecond, count: 1, expectWait: 12 * time.Millisecond},
		},
	}, {
		about:        "within capacity",
		fillInterval: 10 * time.Millisecond,
		capacity:     5,
		reqs: []takeReq{
			{time: 0, count: 5, expectWait: 0},
			{time: 60 * time.Millisecond, count: 5, expectWait: 0},
			{time: 60 * time.Millisecond, count: 1, expectWait: 10 * time.Millisecond},
			{time: 80 * time.Millisecond, count: 2, expectWait: 10 * time.Millisecond},
		},
	}}

	for _, test := range tests {
		t.Run(test.about, func(t *testing.T) {
			tb, clock := newTestBucket(test.fillInterval, test.capacity, 1)
			startTime := clock.Now()
			for i, req := range test.reqs {
				d, ok := tb.take(startTime.Add(req.time), req.count, infinityDuration)
				if !ok {
					t.Errorf("test %d: take failed unexpectedly", i)
				}
				if d != req.expectWait {
					t.Errorf("test %d: got %v want %v", i, d, req.expectWait)
				}
			}
		})
	}
}

type takeReq struct {
	time       time.Duration
	count      int64
	expectWait time.Duration
}

func TestTakeMaxDuration(t *testing.T) {
	tests := []struct {
		about        string
		fillInterval time.Duration
		capacity     int64
		reqs         []takeReq
	}{
		{
			about:        "wait time exceeds max",
			fillInterval: 100 * time.Millisecond,
			capacity:     1,
			reqs: []takeReq{
				{time: 0, count: 1, expectWait: 0},
				{time: 0, count: 1, expectWait: 100 * time.Millisecond},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.about, func(t *testing.T) {
			tb, clock := newTestBucket(test.fillInterval, test.capacity, 1)
			startTime := clock.Now()
			for i, req := range test.reqs {
				if req.expectWait > 0 {
					d, ok := tb.take(startTime.Add(req.time), req.count, req.expectWait-1)
					if ok {
						t.Errorf("test %d: expected take to fail but it succeeded", i)
					}
					if d != 0 {
						t.Errorf("test %d: got %v want 0", i, d)
					}
				}
				d, ok := tb.take(startTime.Add(req.time), req.count, req.expectWait)
				if !ok {
					t.Errorf("test %d: expected take to succeed but it failed", i)
				}
				if d != req.expectWait {
					t.Errorf("test %d: got %v want %v", i, d, req.expectWait)
				}
			}
		})
	}
}

func TestTakeAvailable(t *testing.T) {
	type takeAvailableReq struct {
		time   time.Duration
		count  int64
		expect int64
	}
	tests := []struct {
		about        string
		fillInterval time.Duration
		capacity     int64
		reqs         []takeAvailableReq
	}{{
		about:        "serial requests",
		fillInterval: 250 * time.Millisecond,
		capacity:     10,
		reqs: []takeAvailableReq{
			{time: 0, count: 0, expect: 0},
			{time: 0, count: 10, expect: 10},
			{time: 0, count: 1, expect: 0},
			{time: 250 * time.Millisecond, count: 1, expect: 1},
		},
	}, {
		about:        "concurrent requests",
		fillInterval: 250 * time.Millisecond,
		capacity:     10,
		reqs: []takeAvailableReq{
			{time: 0, count: 5, expect: 5},
			{time: 0, count: 2, expect: 2},
			{time: 0, count: 5, expect: 3},
			{time: 0, count: 1, expect: 0},
		},
	}, {
		about:        "more than capacity",
		fillInterval: 1 * time.Millisecond,
		capacity:     10,
		reqs: []takeAvailableReq{
			{time: 0, count: 10, expect: 10},
			{time: 20 * time.Millisecond, count: 15, expect: 10},
		},
	}, {
		about:        "within capacity",
		fillInterval: 10 * time.Millisecond,
		capacity:     5,
		reqs: []takeAvailableReq{
			{time: 0, count: 5, expect: 5},
			{time: 60 * time.Millisecond, count: 5, expect: 5},
			{time: 70 * time.Millisecond, count: 1, expect: 1},
		},
	}}

	for _, test := range tests {
		t.Run(test.about, func(t *testing.T) {
			tb, clock := newTestBucket(test.fillInterval, test.capacity, 1)
			startTime := clock.Now()
			for i, req := range test.reqs {
				d := tb.takeAvailable(startTime.Add(req.time), req.count)
				if d != req.expect {
					t.Errorf("test %d: got %v want %v", i, d, req.expect)
				}
			}
		})
	}
}

func TestPanics(t *testing.T) {
	assertPanic := func(t *testing.T, name string, f func(), expected string) {
		t.Helper()
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("%s: expected panic, but got none", name)
				return
			}
			if msg, ok := r.(string); !ok || msg != expected {
				t.Errorf("%s: unexpected panic message: got %q, want %q", name, r, expected)
			}
		}()
		f()
	}

	assertPanic(t, "zero fill interval", func() { NewBucket(0, 1) }, "token bucket fill interval must be positive")
	assertPanic(t, "negative fill interval", func() { NewBucket(-1, 1) }, "token bucket fill interval must be positive")
	assertPanic(t, "zero capacity", func() { NewBucket(1, 0) }, "token bucket capacity must be positive")
	assertPanic(t, "negative capacity", func() { NewBucket(1, -1) }, "token bucket capacity must be positive")
	assertPanic(t, "zero quantum", func() { NewBucketWithQuantum(1, 1, 0) }, "token bucket quantum must be positive")
	assertPanic(t, "negative quantum", func() { NewBucketWithQuantum(1, 1, -1) }, "token bucket quantum must be positive")
	assertPanic(t, "negative rate", func() { NewBucketWithRate(-1.0, 1) }, "rate must be positive")
}

func isCloseTo(x, y, tolerance float64) bool {
	if y == 0 {
		return math.Abs(x) < tolerance
	}
	return math.Abs(x-y)/y < tolerance
}

func TestRate(t *testing.T) {
	tb := NewBucket(1, 1)
	if !isCloseTo(tb.Rate(), 1e9, 0.00001) {
		t.Errorf("got %v want 1e9", tb.Rate())
	}
	tb = NewBucket(2*time.Second, 1)
	if !isCloseTo(tb.Rate(), 0.5, 0.00001) {
		t.Errorf("got %v want 0.5", tb.Rate())
	}
	tb = NewBucketWithQuantum(100*time.Millisecond, 1, 5)
	if !isCloseTo(tb.Rate(), 50, 0.00001) {
		t.Errorf("got %v want 50", tb.Rate())
	}
}

func TestNewBucketWithRate(t *testing.T) {
	checkRate := func(t *testing.T, rate float64) {
		t.Helper()
		// Directly create the mock clock instead of an unused bucket.
		clock := &mockClock{now: time.Now()}
		tb := NewBucketWithRateAndClock(rate, 1<<62, clock)

		if !isCloseTo(tb.Rate(), rate, rateMargin) {
			t.Errorf("rate %g: got rate %g want %g", rate, tb.Rate(), rate)
		}

		// Drain the bucket
		d, ok := tb.take(clock.Now(), 1<<62, infinityDuration)
		if !ok || d != 0 {
			t.Errorf("rate %g: initial take failed", rate)
		}

		// Check that the actual rate is as expected by
		// asking for a not-quite multiple of the bucket's
		// quantum and checking that the wait time is correct.
		d, ok = tb.take(clock.Now(), tb.quantum*2-tb.quantum/2, infinityDuration)
		if !ok {
			t.Errorf("rate %g: second take failed", rate)
		}
		expectWait := time.Duration(1e9 * float64(tb.quantum) * 2 / rate)
		if !isCloseTo(float64(d), float64(expectWait), rateMargin) {
			t.Errorf("rate %g: got wait time %v want %v", rate, d, expectWait)
		}
	}

	for rate := float64(1); rate < 1e4; rate += 7 {
		checkRate(t, rate)
	}
	for _, rate := range []float64{
		1024 * 1024 * 1024,
		1e-5,
		0.9e-5,
		0.5,
		0.9,
		0.9e8,
		3e12,
		// Rates higher than this may not be representable.
		// float64(1<<63 - 1),
	} {
		checkRate(t, rate)
		checkRate(t, rate/3)
		checkRate(t, rate*1.3)
	}
}

func TestAvailable(t *testing.T) {
	tests := []struct {
		about                 string
		capacity              int64
		fillInterval          time.Duration
		take                  int64
		sleep                 time.Duration
		expectCountAfterTake  int64
		expectCountAfterSleep int64
	}{{
		about:                 "should fill tokens after interval",
		capacity:              5,
		fillInterval:          time.Second,
		take:                  5,
		sleep:                 time.Second,
		expectCountAfterTake:  0,
		expectCountAfterSleep: 1,
	}, {
		about:                 "should fill tokens plus existing count",
		capacity:              2,
		fillInterval:          time.Second,
		take:                  1,
		sleep:                 time.Second,
		expectCountAfterTake:  1,
		expectCountAfterSleep: 2,
	}, {
		about:                 "shouldn't fill before interval",
		capacity:              2,
		fillInterval:          2 * time.Second,
		take:                  1,
		sleep:                 time.Second,
		expectCountAfterTake:  1,
		expectCountAfterSleep: 1,
	}, {
		about:                 "should fill only once after 1*interval before 2*interval",
		capacity:              2,
		fillInterval:          2 * time.Second,
		take:                  1,
		sleep:                 3 * time.Second,
		expectCountAfterTake:  1,
		expectCountAfterSleep: 2,
	}}

	for _, tt := range tests {
		t.Run(tt.about, func(t *testing.T) {
			tb, clock := newTestBucket(tt.fillInterval, tt.capacity, 1)
			startTime := clock.Now()

			if c := tb.takeAvailable(startTime, tt.take); c != tt.take {
				t.Errorf("take = %d, want = %d", c, tt.take)
			}
			tb.mu.Lock()
			tb.adjustAvailableTokens(tb.currentTick(startTime))
			c := tb.availableTokens
			tb.mu.Unlock()
			if c != tt.expectCountAfterTake {
				t.Errorf("after take, available = %d, want = %d", c, tt.expectCountAfterTake)
			}

			tb.mu.Lock()
			tb.adjustAvailableTokens(tb.currentTick(startTime.Add(tt.sleep)))
			c = tb.availableTokens
			tb.mu.Unlock()
			if c != tt.expectCountAfterSleep {
				t.Errorf("after sleep, available = %d, want = %d", c, tt.expectCountAfterSleep)
			}
		})
	}
}

func TestNoBonusTokenAfterBucketIsFull(t *testing.T) {
	tb, clock := newTestBucket(time.Second, 100, 20)

	if curAvail := tb.Available(); curAvail != 100 {
		t.Fatalf("initially: actual available = %d, expected = %d", curAvail, 100)
	}

	clock.advance(time.Second * 5)

	if curAvail := tb.Available(); curAvail != 100 {
		t.Fatalf("after pause: actual available = %d, expected = %d", curAvail, 100)
	}

	if cnt := tb.TakeAvailable(100); cnt != 100 {
		t.Fatalf("taking: actual taken count = %d, expected = %d", cnt, 100)
	}

	if curAvail := tb.Available(); curAvail != 0 {
		t.Fatalf("after taken: actual available = %d, expected = %d", curAvail, 0)
	}
}

func BenchmarkWait(b *testing.B) {
	tb := NewBucket(1, 16*1024)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		tb.Wait(1)
	}
}

func BenchmarkNewBucketWithRate(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		NewBucketWithRate(4e12, 1<<62)
	}
}
