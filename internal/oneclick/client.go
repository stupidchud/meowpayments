// Package oneclick is the domain-level client for the NEAR Intents 1-click API.
// It wraps the raw HTTP transport, maps types, and adds retry logic.
package oneclick

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	transport "github.com/meowpayments/meowpayments/internal/http"
)

// Client is the interface for interacting with the 1-click API at a domain level.
type Client interface {
	Tokens(ctx context.Context) ([]Token, error)
	Quote(ctx context.Context, params QuoteParams) (*Quote, error)
	SubmitDeposit(ctx context.Context, depositAddress, txHash string) error
	Status(ctx context.Context, depositAddress string) (*DepositStatus, error)
	AnyInputWithdrawals(ctx context.Context, depositAddress string) ([]transport.WithdrawalResponse, error)
}

type client struct {
	t    *transport.Transport
	retr *retrier
}

// NewClient creates a new domain-level 1-click client.
func NewClient(t *transport.Transport) Client {
	return &client{
		t:    t,
		retr: newRetrier(3, 500*time.Millisecond),
	}
}

func (c *client) Tokens(ctx context.Context) ([]Token, error) {
	var tokens []transport.TokenResponse
	err := c.retr.run(ctx, func() error {
		var e error
		tokens, e = c.t.GetTokens(ctx)
		return e
	})
	if err != nil {
		return nil, fmt.Errorf("oneclick: tokens: %w", err)
	}
	return mapTokens(tokens), nil
}

func (c *client) Quote(ctx context.Context, params QuoteParams) (*Quote, error) {
	if params.RecipientType == "" {
		params.RecipientType = "DESTINATION_CHAIN"
	}

	refundTo := params.RefundAddress
	originAsset := params.OriginAsset
	swapType := "ANY_INPUT"
	depositType := "INTENTS"
	amount := "0"

	if originAsset != "" && params.OriginAmountUSD > 0 {
		// Specific origin asset with a known USD amount: use EXACT_INPUT + ORIGIN_CHAIN
		// so the 1-click API returns a real native chain address (0x..., base58, etc.).
		// We need to look up the token to get price and decimals for the amount calculation.
		tokens, err := c.Tokens(ctx)
		if err != nil {
			return nil, fmt.Errorf("oneclick: quote: fetch tokens for amount calc: %w", err)
		}
		var originToken *Token
		for i := range tokens {
			if tokens[i].AssetID == originAsset {
				originToken = &tokens[i]
				break
			}
		}
		if originToken != nil && originToken.PriceUSD > 0 {
			// amount = ceil((usd / price) * 1.005 * 10^decimals) as integer string.
			// The 0.5% buffer guards against minor price drift between quote time
			// and when the user actually sends — without it, a stale price_usd can
			// produce an amount that falls just short of the required deposit,
			// causing INCOMPLETE_DEPOSIT. Any excess is refunded to refundTo by
			// NEAR Intents after the swap completes.
			units := (params.OriginAmountUSD / originToken.PriceUSD) * 1.005 * math.Pow(10, float64(originToken.Decimals))
			amount = strconv.FormatInt(int64(math.Ceil(units)), 10)
			swapType = "EXACT_INPUT"
			depositType = "ORIGIN_CHAIN"
			if params.OriginRefundAddress != "" {
				refundTo = params.OriginRefundAddress
			}
		}
		// If token not found or no price, fall through to ANY_INPUT mode silently.
	}

	if swapType == "ANY_INPUT" {
		// ANY_INPUT requires originAsset = destinationAsset.
		originAsset = params.DestinationAsset
	}

	req := transport.QuoteRequest{
		Dry:               params.DryRun,
		SwapType:          swapType,
		SlippageTolerance: params.SlippageBPS,
		OriginAsset:       originAsset,
		DepositType:       depositType,
		DestinationAsset:  params.DestinationAsset,
		Amount:            amount,
		RefundTo:          refundTo,
		RefundType:        "ORIGIN_CHAIN",
		Recipient:         params.RecipientAddress,
		RecipientType:     params.RecipientType,
		Deadline:          params.Deadline.UTC().Format(time.RFC3339),
	}

	var resp *transport.QuoteResponse
	err := c.retr.run(ctx, func() error {
		var e error
		resp, e = c.t.PostQuote(ctx, req)
		return e
	})
	if err != nil {
		return nil, fmt.Errorf("oneclick: quote: %w", err)
	}

	deadline, _ := time.Parse(time.RFC3339, resp.Quote.Deadline)
	return &Quote{
		DepositAddress: resp.Quote.DepositAddress,
		DepositMemo:    resp.Quote.DepositMemo,
		Deadline:       deadline,
		TimeEstimate:   resp.Quote.TimeEstimate,
	}, nil
}

func (c *client) SubmitDeposit(ctx context.Context, depositAddress, txHash string) error {
	return c.t.PostDepositSubmit(ctx, transport.DepositSubmitRequest{
		DepositAddress: depositAddress,
		TxHash:         txHash,
	})
}

func (c *client) Status(ctx context.Context, depositAddress string) (*DepositStatus, error) {
	var resp *transport.StatusResponse
	err := c.retr.run(ctx, func() error {
		var e error
		resp, e = c.t.GetStatus(ctx, depositAddress)
		return e
	})
	if err != nil {
		return nil, fmt.Errorf("oneclick: status: %w", err)
	}
	return &DepositStatus{
		Raw:    resp.Status,
		Mapped: MapOneclickStatus(resp.Status),
	}, nil
}

func (c *client) AnyInputWithdrawals(ctx context.Context, depositAddress string) ([]transport.WithdrawalResponse, error) {
	var out []transport.WithdrawalResponse
	err := c.retr.run(ctx, func() error {
		var e error
		out, e = c.t.GetAnyInputWithdrawals(ctx, depositAddress)
		return e
	})
	if err != nil {
		return nil, fmt.Errorf("oneclick: withdrawals: %w", err)
	}
	return out, nil
}

// mappers

func mapTokens(in []transport.TokenResponse) []Token {
	out := make([]Token, len(in))
	for i, t := range in {
		addr := ""
		if t.ContractAddress != nil {
			addr = *t.ContractAddress
		}
		out[i] = Token{
			AssetID:         t.AssetID,
			Symbol:          t.Symbol,
			Blockchain:      t.Blockchain,
			Decimals:        t.Decimals,
			PriceUSD:        t.Price,
			ContractAddress: addr,
		}
	}
	return out
}
