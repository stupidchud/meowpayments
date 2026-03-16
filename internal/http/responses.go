package http

// TokenResponse represents one token from GET /v0/tokens.
type TokenResponse struct {
	AssetID         string  `json:"assetId"`
	Decimals        int     `json:"decimals"`
	Blockchain      string  `json:"blockchain"`
	Symbol          string  `json:"symbol"`
	Price           float64 `json:"price"`
	PriceUpdatedAt  string  `json:"priceUpdatedAt"`
	ContractAddress *string `json:"contractAddress"`
}

// QuoteResponse is the response from POST /v0/quote.
// The API wraps quote fields inside a nested "quote" object.
type QuoteResponse struct {
	Quote QuoteDetails `json:"quote"`
}

// QuoteDetails contains the actual quote fields nested under the "quote" key.
type QuoteDetails struct {
	DepositAddress              string `json:"depositAddress"`
	DepositMemo                 string `json:"depositMemo"`
	AmountIn                    string `json:"amountIn"`
	AmountInFormatted           string `json:"amountInFormatted"`
	AmountInUsd                 string `json:"amountInUsd"`
	MinAmountIn                 string `json:"minAmountIn"`
	AmountOut                   string `json:"amountOut"`
	AmountOutFormatted          string `json:"amountOutFormatted"`
	AmountOutUsd                string `json:"amountOutUsd"`
	MinAmountOut                string `json:"minAmountOut"`
	Deadline                    string `json:"deadline"`
	TimeWhenInactive            string `json:"timeWhenInactive"`
	TimeEstimate                int    `json:"timeEstimate"`
	VirtualChainRecipient       string `json:"virtualChainRecipient"`
	VirtualChainRefundRecipient string `json:"virtualChainRefundRecipient"`
	CustomRecipientMsg          string `json:"customRecipientMsg"`
}

// StatusResponse is the response from GET /v0/status.
type StatusResponse struct {
	// Status values: PENDING_DEPOSIT | KNOWN_DEPOSIT_TX | PROCESSING | SUCCESS
	//                INCOMPLETE_DEPOSIT | REFUNDED | FAILED
	Status         string      `json:"status"`
	DepositAddress string      `json:"depositAddress"`
	SwapDetails    interface{} `json:"swapDetails"`
	TxHashes       interface{} `json:"txHashes"`
}

// WithdrawalResponse represents one entry from GET /v0/any-input/withdrawals.
type WithdrawalResponse struct {
	TxHash    string `json:"txHash"`
	Amount    string `json:"amount"`
	Token     string `json:"token"`
	Timestamp string `json:"timestamp"`
	Status    string `json:"status"`
}

// APIError is the error envelope returned by the 1-click API.
type APIError struct {
	Message    interface{} `json:"message"` // string or []string
	Err        string      `json:"error"`
	StatusCode int         `json:"statusCode"`
	Path       string      `json:"path"`

	// HTTPStatus is the actual HTTP status code from the response (set by transport).
	HTTPStatus int `json:"-"`
}

func (e *APIError) Error() string {
	if e.Err != "" {
		return e.Err
	}
	return "1click api error"
}

func (e *APIError) IsUnauthorized() bool { return e.HTTPStatus == 401 }
func (e *APIError) IsRateLimit() bool    { return e.HTTPStatus == 429 }
func (e *APIError) IsBadRequest() bool   { return e.HTTPStatus == 400 }
