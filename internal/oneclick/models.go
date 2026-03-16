package oneclick

import "time"

// Token is the domain representation of a supported asset.
type Token struct {
	AssetID         string
	Symbol          string
	Blockchain      string
	Decimals        int
	PriceUSD        float64
	ContractAddress string
}

// QuoteParams are the inputs to the domain client's Quote method.
type QuoteParams struct {
	// DestinationAsset is the defuse asset identifier the operator wants to receive.
	// e.g. "nep141:usdc.near"
	DestinationAsset string
	// OriginAsset is the defuse asset identifier the customer will send.
	// When empty, ANY_INPUT mode is used (any token accepted).
	OriginAsset string
	// OriginRefundAddress is the customer's address on the origin chain for refunds.
	// Only used when OriginAsset is set. Falls back to RefundAddress if empty.
	OriginRefundAddress string
	// OriginAmountUSD is the payment amount in USD, used to compute the EXACT_INPUT
	// token amount when OriginAsset is set. If zero, falls back to ANY_INPUT mode.
	OriginAmountUSD float64
	// RecipientAddress is the on-chain address the output tokens are sent to.
	RecipientAddress string
	// RecipientType is "DESTINATION_CHAIN" (default) or "INTENTS".
	RecipientType string
	// RefundAddress is where tokens go if the swap fails.
	RefundAddress string
	// Deadline is when the quote and deposit address expire.
	Deadline time.Time
	// SlippageBPS is slippage tolerance in basis points (100 = 1%).
	SlippageBPS int
	// DryRun true means preview only - no deposit address is generated.
	DryRun bool
}

// Quote is the result of a successful quote call.
type Quote struct {
	DepositAddress string
	DepositMemo    string
	Deadline       time.Time
	TimeEstimate   int // seconds
}

// DepositStatus is the mapped result of a status poll.
type DepositStatus struct {
	// Raw is the verbatim status string from the 1-click API.
	Raw string
	// Mapped is our canonical PaymentStatus.
	Mapped PaymentStatus
}

// PaymentStatus is the canonical status used throughout meowpayments.
type PaymentStatus string

const (
	StatusPending         PaymentStatus = "PENDING"
	StatusAwaitingDeposit PaymentStatus = "AWAITING_DEPOSIT"
	StatusDetected        PaymentStatus = "DETECTED"
	StatusProcessing      PaymentStatus = "PROCESSING"
	StatusCompleted       PaymentStatus = "COMPLETED"
	StatusExpired         PaymentStatus = "EXPIRED"
	StatusRefunded        PaymentStatus = "REFUNDED"
	StatusFailed          PaymentStatus = "FAILED"
	StatusIncomplete      PaymentStatus = "INCOMPLETE"
)

// MapOneclickStatus converts a raw 1-click API status string to our domain status.
func MapOneclickStatus(raw string) PaymentStatus {
	switch raw {
	case "PENDING_DEPOSIT":
		return StatusAwaitingDeposit
	case "KNOWN_DEPOSIT_TX":
		return StatusDetected
	case "PROCESSING":
		return StatusProcessing
	case "SUCCESS":
		return StatusCompleted
	case "INCOMPLETE_DEPOSIT":
		return StatusIncomplete
	case "REFUNDED":
		return StatusRefunded
	case "FAILED":
		return StatusFailed
	default:
		return StatusAwaitingDeposit
	}
}

// IsTerminal returns true if no further status transitions are expected.
func (s PaymentStatus) IsTerminal() bool {
	switch s {
	case StatusCompleted, StatusExpired, StatusRefunded, StatusFailed:
		return true
	default:
		return false
	}
}
