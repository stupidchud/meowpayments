package handlers

import (
	"github.com/go-fuego/fuego"
	"github.com/meowpayments/meowpayments/internal/api/dto"
	"github.com/meowpayments/meowpayments/internal/oneclick"
)

type TokenHandlers struct {
	cache *oneclick.TokenCache
}

func NewTokenHandlers(cache *oneclick.TokenCache) *TokenHandlers {
	return &TokenHandlers{cache: cache}
}

// List handles GET /v1/tokens - returns all supported assets.
func (h *TokenHandlers) List(c fuego.ContextNoBody) (dto.TokenListResponse, error) {
	tokens, err := h.cache.All(c.Context())
	if err != nil {
		return dto.TokenListResponse{}, fuego.HTTPError{Err: err, Status: 502, Title: "could not fetch token list"}
	}

	items := make([]dto.TokenResponse, len(tokens))
	for i, t := range tokens {
		items[i] = dto.TokenResponse{
			AssetID:         t.AssetID,
			Symbol:          t.Symbol,
			Blockchain:      t.Blockchain,
			Decimals:        t.Decimals,
			PriceUSD:        t.PriceUSD,
			ContractAddress: t.ContractAddress,
		}
	}
	return dto.TokenListResponse{Tokens: items, Count: len(items)}, nil
}
