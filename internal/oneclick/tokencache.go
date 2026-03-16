package oneclick

import (
	"context"
	"sync"
	"time"
)

// TokenCache is an in-memory cache of the supported token list.
// It refreshes automatically in the background via StartRefreshLoop.
type TokenCache struct {
	mu          sync.RWMutex
	tokens      []Token
	byAssetID   map[string]*Token
	refreshedAt time.Time
	ttl         time.Duration
	client      Client
}

// NewTokenCache creates a token cache backed by the given Client.
func NewTokenCache(client Client, ttl time.Duration) *TokenCache {
	return &TokenCache{
		ttl:    ttl,
		client: client,
	}
}

// All returns all cached tokens, refreshing if the cache is stale.
func (c *TokenCache) All(ctx context.Context) ([]Token, error) {
	c.mu.RLock()
	if !c.isStale() {
		defer c.mu.RUnlock()
		return c.tokens, nil
	}
	c.mu.RUnlock()

	return c.refresh(ctx)
}

// ByAssetID returns a single token by its defuse asset identifier.
func (c *TokenCache) ByAssetID(ctx context.Context, assetID string) (*Token, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	t, ok := c.byAssetID[assetID]
	return t, ok
}

// StartRefreshLoop runs a background goroutine that refreshes the cache on the given TTL.
// It blocks until ctx is cancelled.
func (c *TokenCache) StartRefreshLoop(ctx context.Context) {
	// Do an immediate refresh.
	_, _ = c.refresh(ctx)

	ticker := time.NewTicker(c.ttl)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = c.refresh(ctx)
		}
	}
}

func (c *TokenCache) isStale() bool {
	return c.tokens == nil || time.Since(c.refreshedAt) > c.ttl
}

func (c *TokenCache) refresh(ctx context.Context) ([]Token, error) {
	tokens, err := c.client.Tokens(ctx)
	if err != nil {
		return nil, err
	}

	index := make(map[string]*Token, len(tokens))
	for i := range tokens {
		t := tokens[i]
		index[t.AssetID] = &t
	}

	c.mu.Lock()
	c.tokens = tokens
	c.byAssetID = index
	c.refreshedAt = time.Now()
	c.mu.Unlock()

	return tokens, nil
}
