// Copyright 2014 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package ratelimit

import "io"

// reader implements a rate-limited io.Reader.
type reader struct {
	r      io.Reader
	bucket *Bucket
}

// Reader returns a reader that is rate limited by
// the given token bucket. Each token in the bucket
// represents one byte.
func Reader(r io.Reader, bucket *Bucket) io.Reader {
	return &reader{
		r:      r,
		bucket: bucket,
	}
}

// Read implements the io.Reader interface.
func (r *reader) Read(buf []byte) (int, error) {
	if len(buf) == 0 {
		return 0, nil
	}
	// Wait for the first byte to become available.
	// This ensures that we don't read from the underlying
	// reader until we have capacity.
	r.bucket.Wait(1)

	// Take up to the number of available tokens, but not more
	// than the buffer size.
	n := r.bucket.TakeAvailable(int64(len(buf)))
	if n == 0 {
		// Should not happen as we've just waited for one.
		n = 1
	}

	nRead, err := r.r.Read(buf[0:n])
	if nRead > 0 {
		// If we read less than we took tokens for, return the
		// unused tokens to the bucket.
		if n > int64(nRead) {
			r.bucket.Take(-(n - int64(nRead)))
		}
	}
	return nRead, err
}

// writer implements a rate-limited io.Writer.
type writer struct {
	w      io.Writer
	bucket *Bucket
}

// Writer returns a writer that is rate limited by
// the given token bucket. Each token in the bucket
// represents one byte.
func Writer(w io.Writer, bucket *Bucket) io.Writer {
	return &writer{
		w:      w,
		bucket: bucket,
	}
}

// Write implements the io.Writer interface.
func (w *writer) Write(buf []byte) (int, error) {
	if len(buf) == 0 {
		return 0, nil
	}
	w.bucket.Wait(int64(len(buf)))
	return w.w.Write(buf)
}
