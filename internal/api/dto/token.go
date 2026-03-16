package dto

// TokenResponse is the public representation of a supported asset.
type TokenResponse struct {
	AssetID         string  `json:"asset_id"`
	Symbol          string  `json:"symbol"`
	Blockchain      string  `json:"blockchain"`
	Decimals        int     `json:"decimals"`
	PriceUSD        float64 `json:"price_usd"`
	ContractAddress string  `json:"contract_address,omitempty"`
}

// TokenListResponse wraps a list of tokens.
type TokenListResponse struct {
	Tokens []TokenResponse `json:"tokens"`
	Count  int             `json:"count"`
}
