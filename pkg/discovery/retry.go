package discovery

import (
	"context"
	"errors"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"golang.org/x/time/rate"
)

const (
	initialBackoff = 500 * time.Millisecond
	jitterLow      = 0.8
	jitterRange    = 0.4
)

// withRetry executes fn, retrying on 429/503 with exponential backoff and jitter.
// The rate limiter is waited before every attempt.
func withRetry(ctx context.Context, limiter *rate.Limiter, maxRetries int, fn func() error) error {
	backoff := initialBackoff

	for attempt := 0; ; attempt++ {
		err := limiter.Wait(ctx)
		if err != nil {
			return err
		}

		err = fn()
		if err == nil {
			return nil
		}

		if !isRateLimited(err) || attempt >= maxRetries {
			return err
		}

		// jitter: ±20% of backoff to spread retry storms across workers
		//nolint:gosec // jitter does not require cryptographic randomness
		jitter := time.Duration(float64(backoff) * (jitterLow + jitterRange*rand.Float64()))

		select {
		case <-time.After(jitter):
		case <-ctx.Done():
			return ctx.Err()
		}

		backoff *= 2
	}
}

func isRateLimited(err error) bool {
	if te, ok := errors.AsType[*transport.Error](err); ok {
		return te.StatusCode == http.StatusTooManyRequests ||
			te.StatusCode == http.StatusServiceUnavailable
	}

	return false
}
