package dto

import "time"

// CreatePaymentRequest is the body for POST /v1/payments.
type CreatePaymentRequest struct {
	// DestAssetID is the defuse asset identifier the operator wants to receive.
	// Example: "nep141:usdc.near", "nep141:eth-0xa0b86991c...omft.near"
	// Get valid values from GET /v1/tokens.
	DestAssetID string `json:"dest_asset_id" validate:"required"`

	// OriginAssetID is the defuse asset identifier the customer will send.
	// When set, the deposit address is generated specifically for that token/chain,
	// e.g. "nep141:sol-0x...omft.near" for USDC on Solana.
	// If omitted, ANY_INPUT mode is used and any supported token is accepted.
	OriginAssetID string `json:"origin_asset_id"`

	// OriginRefundAddress is the customer's address on the origin chain.
	// Required when origin_asset_id is set - used by NEAR Intents to refund
	// the customer if the swap fails. Must be a valid address on the origin chain
	// (e.g. 0x... for Ethereum, base58 for Solana).
	// If omitted, falls back to the operator's configured refund address.
	OriginRefundAddress string `json:"origin_refund_address"`

	// DestChain is the blockchain name, e.g. "near", "eth", "sol".
	DestChain string `json:"dest_chain" validate:"required"`

	// DestAddress is the on-chain address to deliver the output tokens to.
	DestAddress string `json:"dest_address" validate:"required"`

	// AmountUSD is an informational expected USD amount (guidance for the customer).
	// Since we use ANY_INPUT the actual amount may differ. Optional.
	AmountUSD *string `json:"amount_usd"`

	// ExpiresInSeconds is how long until this payment expires. Default: 3600 (1 hour).
	ExpiresInSeconds int `json:"expires_in_seconds"`

	// CallbackURL is an HTTPS URL to POST when the payment reaches a terminal state.
	CallbackURL string `json:"callback_url"`

	// CustomerEmail is optional and purely informational.
	CustomerEmail string `json:"customer_email"`

	// Metadata is an arbitrary JSON object stored with the payment.
	Metadata map[string]any `json:"metadata"`

	// SlippageBPS overrides the default slippage tolerance (basis points, e.g. 100 = 1%).
	SlippageBPS *int `json:"slippage_bps"`
}

// PaymentResponse is the API representation of a payment.
type PaymentResponse struct {
	ID             string         `json:"id"`
	Status         string         `json:"status"`
	DepositAddress string         `json:"deposit_address,omitempty"`
	DepositMemo    string         `json:"deposit_memo,omitempty"`
	DestAssetID    string         `json:"dest_asset_id"`
	DestChain      string         `json:"dest_chain"`
	DestAddress    string         `json:"dest_address"`
	OriginAssetID  string         `json:"origin_asset_id,omitempty"`
	AmountUSD      *string        `json:"amount_usd,omitempty"`
	PaymentURL     string         `json:"payment_url"`
	ExpiresAt      time.Time      `json:"expires_at"`
	CallbackURL    string         `json:"callback_url,omitempty"`
	CustomerEmail  string         `json:"customer_email,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// PaymentListResponse wraps paginated payment results.
type PaymentListResponse struct {
	Payments []*PaymentResponse `json:"payments"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
}

// SubmitDepositRequest is the body for POST /v1/pay/:id/submit.
// Customers call this after sending their transaction to speed up processing.
type SubmitDepositRequest struct {
	TxHash string `json:"tx_hash" validate:"required"`
}
