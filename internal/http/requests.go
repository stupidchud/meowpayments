package http

// QuoteRequest is the body sent to POST /v0/quote.
type QuoteRequest struct {
	Dry                bool      `json:"dry"`
	SwapType           string    `json:"swapType"`
	SlippageTolerance  int       `json:"slippageTolerance"`
	OriginAsset        string    `json:"originAsset,omitempty"`
	DepositType        string    `json:"depositType"`
	DestinationAsset   string    `json:"destinationAsset"`
	Amount             string    `json:"amount"`
	RefundTo           string    `json:"refundTo"`
	RefundType         string    `json:"refundType"`
	Recipient          string    `json:"recipient"`
	RecipientType      string    `json:"recipientType"`
	Deadline           string    `json:"deadline"` // RFC3339
	QuoteWaitingTimeMs int       `json:"quoteWaitingTimeMs,omitempty"`
	AppFees            []AppFee  `json:"appFees,omitempty"`
}

// AppFee is an optional fee entry in a QuoteRequest.
type AppFee struct {
	Recipient string `json:"recipient"`
	FeesBPS   int    `json:"feesBPS"`
}

// DepositSubmitRequest is the body sent to POST /v0/deposit/submit.
type DepositSubmitRequest struct {
	DepositAddress string `json:"depositAddress"`
	TxHash         string `json:"txHash"`
}
