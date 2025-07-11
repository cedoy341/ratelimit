# ratelimit

The ratelimit package provides an efficient token bucket implementation. See
http://en.wikipedia.org/wiki/Token_bucket.

## Usage

#### func  Limiter

Limiter for Automatic Bucket Management
The Limiter type manages a collection of token buckets. It's useful when you need to rate-limit multiple independent entities (e.g., users, IP addresses, API keys) and want to automatically clean up the resources for entities that are no longer active.
#### func NewLimiter(ttl time.Duration, newBucket func() *Bucket) *Limiter

A Limiter creates, manages, and automatically cleans up token buckets that have not been used for a specified Time-To-Live (TTL) duration.

#### func NewLimiter
```go

func NewLimiter(ttl time.Duration, newBucket func() *Bucket) *Limiter
NewLimiter creates a new Limiter. It takes a ttl for expiring unused buckets and a factory function (newBucket) that defines how to create new buckets (e.g., their rate and capacity). A background goroutine runs periodically to clean up expired buckets.
```
#### func (*Limiter) Get
```go
func (l *Limiter) Get(key string) *Bucket
Get returns the token bucket for the given key. If a bucket for the key does not exist, a new one is created using the factory function provided to NewLimiter. Each call to Get updates the bucket's last access time, preventing it from expiring.
```
#### func (*Limiter) Stop
```go
func (l *Limiter) Stop()
Stop terminates the background cleanup goroutine. It is called automatically when the Limiter is garbage collected, but can be invoked manually to stop it sooner.
```
#### func  Reader

```go
func Reader(r io.Reader, bucket *Bucket) io.Reader
```
Reader returns a reader that is rate limited by the given token bucket. Each
token in the bucket represents one byte.

#### func  Writer

```go
func Writer(w io.Writer, bucket *Bucket) io.Writer
```
Writer returns a writer that is rate limited by the given token bucket. Each
token in the bucket represents one byte.

#### type Bucket

```go
type Bucket struct {
}
```

Bucket represents a token bucket that fills at a predetermined rate. Methods on
Bucket may be called concurrently.

#### func  NewBucket

```go
func NewBucket(fillInterval time.Duration, capacity int64) *Bucket
```
NewBucket returns a new token bucket that fills at the rate of one token every
fillInterval, up to the given maximum capacity. Both arguments must be positive.
The bucket is initially full.

#### func  NewBucketWithQuantum

```go
func NewBucketWithQuantum(fillInterval time.Duration, capacity, quantum int64) *Bucket
```
NewBucketWithQuantum is similar to NewBucket, but allows the specification of
the quantum size - quantum tokens are added every fillInterval.

#### func  NewBucketWithRate

```go
func NewBucketWithRate(rate float64, capacity int64) *Bucket
```
NewBucketWithRate returns a token bucket that fills the bucket at the rate of
rate tokens per second up to the given maximum capacity. Because of limited
clock resolution, at high rates, the actual rate may be up to 1% different from
the specified rate.

#### func (*Bucket) Available

```go
func (tb *Bucket) Available() int64
```
Available returns the number of available tokens. It will be negative
when there are consumers waiting for tokens. Note that if this
returns greater than zero, it does not guarantee that calls that take
tokens from the buffer will succeed, as the number of available
tokens could have changed in the meantime. This method is intended
primarily for metrics reporting and debugging.

#### func (*Bucket) Rate

```go
func (tb *Bucket) Rate() float64
```
Rate returns the fill rate of the bucket, in tokens per second.

#### func (*Bucket) Take

```go
func (tb *Bucket) Take(count int64) time.Duration
```
Take takes count tokens from the bucket without blocking. It returns the time
that the caller should wait until the tokens are actually available.

Note that if the request is irrevocable - there is no way to return tokens to
the bucket once this method commits us to taking them.

#### func (*Bucket) TakeAvailable

```go
func (tb *Bucket) TakeAvailable(count int64) int64
```
TakeAvailable takes up to count immediately available tokens from the bucket. It
returns the number of tokens removed, or zero if there are no available tokens.
It does not block.

#### func (*Bucket) TakeMaxDuration

```go
func (tb *Bucket) TakeMaxDuration(count int64, maxWait time.Duration) (time.Duration, bool)
```
TakeMaxDuration is like Take, except that it will only take tokens from the
bucket if the wait time for the tokens is no greater than maxWait.

If it would take longer than maxWait for the tokens to become available, it does
nothing and reports false, otherwise it returns the time that the caller should
wait until the tokens are actually available, and reports true.

#### func (*Bucket) Wait

```go
func (tb *Bucket) Wait(count int64)
```
Wait takes count tokens from the bucket, waiting until they are available.

#### func (*Bucket) WaitMaxDuration

```go
func (tb *Bucket) WaitMaxDuration(count int64, maxWait time.Duration) bool
```
WaitMaxDuration is like Wait except that it will only take tokens from the
bucket if it needs to wait for no greater than maxWait. It reports whether any
tokens have been removed from the bucket If no tokens have been removed, it
returns immediately.
