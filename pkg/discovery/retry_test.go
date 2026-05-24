package discovery

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"golang.org/x/time/rate"
)

var (
	errNonRetryable = errors.New("some other error")
	errNonTransport = errors.New("random error")
)

func rateLimitErr(code int) *transport.Error {
	return &transport.Error{StatusCode: code}
}

// TestWithRetrySuccess verifies a successful first call returns immediately.
func TestWithRetrySuccess(t *testing.T) {
	t.Parallel()

	calls := 0

	err := withRetry(context.Background(), rate.NewLimiter(rate.Inf, 0), 3, func() error {
		calls++

		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

// TestWithRetryOnce verifies a 429 triggers one retry that then succeeds.
func TestWithRetryOnce(t *testing.T) {
	t.Parallel()

	calls := 0

	err := withRetry(context.Background(), rate.NewLimiter(rate.Inf, 0), 3, func() error {
		calls++
		if calls == 1 {
			return rateLimitErr(http.StatusTooManyRequests)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error after retry: %v", err)
	}

	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

// TestWithRetry503 verifies 503 is treated the same as 429.
func TestWithRetry503(t *testing.T) {
	t.Parallel()

	calls := 0

	err := withRetry(context.Background(), rate.NewLimiter(rate.Inf, 0), 3, func() error {
		calls++
		if calls == 1 {
			return rateLimitErr(http.StatusServiceUnavailable)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

// TestWithRetryMaxRetriesExceeded verifies the error is returned after maxRetries attempts.
func TestWithRetryMaxRetriesExceeded(t *testing.T) {
	t.Parallel()

	const maxRetries = 2

	calls := 0

	err := withRetry(context.Background(), rate.NewLimiter(rate.Inf, 0), maxRetries, func() error {
		calls++

		return rateLimitErr(http.StatusTooManyRequests)
	})
	if err == nil {
		t.Fatal("expected error after max retries")
	}

	if calls != maxRetries+1 {
		t.Fatalf("expected %d calls, got %d", maxRetries+1, calls)
	}
}

// TestWithRetryNonRetryable verifies non-429/503 errors are returned without retrying.
func TestWithRetryNonRetryable(t *testing.T) {
	t.Parallel()

	calls := 0
	sentinel := errNonRetryable

	err := withRetry(context.Background(), rate.NewLimiter(rate.Inf, 0), 3, func() error {
		calls++

		return sentinel
	})

	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel, got %v", err)
	}

	if calls != 1 {
		t.Fatalf("expected 1 call (no retry for non-rate-limited error), got %d", calls)
	}
}

// TestWithRetryLimiterError verifies that a limiter failure is returned without calling fn.
func TestWithRetryLimiterError(t *testing.T) {
	t.Parallel()
	// burst=0, limit≠Inf → Wait(n=1) fails immediately with "exceeds burst" error.
	limiter := rate.NewLimiter(1, 0)

	err := withRetry(context.Background(), limiter, 3, func() error {
		t.Fatal("fn should not be called when limiter fails")

		return nil
	})
	if err == nil {
		t.Fatal("expected limiter error")
	}
}

// TestWithRetryContextCancelledDuringBackoff verifies the backoff select respects ctx.Done.
func TestWithRetryContextCancelledDuringBackoff(t *testing.T) {
	t.Parallel()
	// The backoff sleep is ~500 ms; a 50 ms timeout cancels it first.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := withRetry(ctx, rate.NewLimiter(rate.Inf, 0), 10, func() error {
		return rateLimitErr(http.StatusTooManyRequests) // always 429
	})
	if err == nil {
		t.Fatal("expected context error from cancelled backoff")
	}
}

// TestIsRateLimitedTrue verifies 429 is considered rate-limited.
func TestIsRateLimitedTrue(t *testing.T) {
	t.Parallel()

	if !isRateLimited(rateLimitErr(http.StatusTooManyRequests)) {
		t.Fatal("expected true for 429")
	}
}

// TestIsRateLimitedTrueFor503 verifies 503 is considered rate-limited.
func TestIsRateLimitedTrueFor503(t *testing.T) {
	t.Parallel()

	if !isRateLimited(rateLimitErr(http.StatusServiceUnavailable)) {
		t.Fatal("expected true for 503")
	}
}

// TestIsRateLimitedFalseOtherStatus verifies other transport errors are not rate-limited.
func TestIsRateLimitedFalseOtherStatus(t *testing.T) {
	t.Parallel()

	if isRateLimited(rateLimitErr(http.StatusInternalServerError)) {
		t.Fatal("expected false for 500")
	}
}

// TestIsRateLimitedFalseNonTransport verifies plain errors are not rate-limited.
func TestIsRateLimitedFalseNonTransport(t *testing.T) {
	t.Parallel()

	if isRateLimited(errNonTransport) {
		t.Fatal("expected false for non-transport error")
	}
}
