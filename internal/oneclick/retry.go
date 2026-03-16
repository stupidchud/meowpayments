package oneclick

import (
	"context"
	"time"

	transport "github.com/meowpayments/meowpayments/internal/http"
)

type retrier struct {
	maxAttempts int
	baseDelay   time.Duration
}

func newRetrier(maxAttempts int, baseDelay time.Duration) *retrier {
	return &retrier{maxAttempts: maxAttempts, baseDelay: baseDelay}
}

// run executes fn with exponential backoff, skipping retries for client errors.
func (r *retrier) run(ctx context.Context, fn func() error) error {
	var lastErr error
	delay := r.baseDelay
	for attempt := 0; attempt < r.maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		// Don't retry on 4xx client errors (except 429 rate limit).
		if apiErr, ok := lastErr.(*transport.APIError); ok {
			if apiErr.IsBadRequest() || apiErr.IsUnauthorized() {
				return lastErr
			}
		}
		if attempt < r.maxAttempts-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			delay *= 2
		}
	}
	return lastErr
}
