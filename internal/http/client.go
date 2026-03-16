// Package http provides the raw HTTP transport layer for the NEAR Intents 1-click API.
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const defaultBaseURL = "https://1click.chaindefuser.com"

// Transport is a low-level HTTP client for the NEAR Intents 1-click API.
// It handles authentication, JSON encoding/decoding, and error mapping.
type Transport struct {
	baseURL string
	apiKey  string
	hc      *http.Client
}

// NewTransport creates a new Transport.
func NewTransport(baseURL, apiKey string, timeout time.Duration) *Transport {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Transport{
		baseURL: baseURL,
		apiKey:  apiKey,
		hc:      &http.Client{Timeout: timeout},
	}
}

// GetTokens calls GET /v0/tokens.
func (t *Transport) GetTokens(ctx context.Context) ([]TokenResponse, error) {
	var out []TokenResponse
	if err := t.get(ctx, "/v0/tokens", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// PostQuote calls POST /v0/quote.
func (t *Transport) PostQuote(ctx context.Context, req QuoteRequest) (*QuoteResponse, error) {
	var out QuoteResponse
	if err := t.post(ctx, "/v0/quote", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PostDepositSubmit calls POST /v0/deposit/submit.
func (t *Transport) PostDepositSubmit(ctx context.Context, req DepositSubmitRequest) error {
	return t.post(ctx, "/v0/deposit/submit", req, nil)
}

// GetStatus calls GET /v0/status?depositAddress=...
func (t *Transport) GetStatus(ctx context.Context, depositAddress string) (*StatusResponse, error) {
	var out StatusResponse
	params := url.Values{"depositAddress": {depositAddress}}
	if err := t.get(ctx, "/v0/status", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetAnyInputWithdrawals calls GET /v0/any-input/withdrawals?depositAddress=...
func (t *Transport) GetAnyInputWithdrawals(ctx context.Context, depositAddress string) ([]WithdrawalResponse, error) {
	var out []WithdrawalResponse
	params := url.Values{"depositAddress": {depositAddress}}
	if err := t.get(ctx, "/v0/any-input/withdrawals", params, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// internal helpers

func (t *Transport) get(ctx context.Context, path string, params url.Values, out interface{}) error {
	u := t.baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("http: new request: %w", err)
	}
	t.setHeaders(req)
	return t.do(req, out)
}

func (t *Transport) post(ctx context.Context, path string, body interface{}, out interface{}) error {
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("http: marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL+path, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("http: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	t.setHeaders(req)
	return t.do(req, out)
}

func (t *Transport) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	if t.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+t.apiKey)
	}
}

func (t *Transport) do(req *http.Request, out interface{}) error {
	resp, err := t.hc.Do(req)
	if err != nil {
		return fmt.Errorf("http: do request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("http: read body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var apiErr APIError
		_ = json.Unmarshal(data, &apiErr)
		apiErr.HTTPStatus = resp.StatusCode
		if apiErr.Err == "" {
			apiErr.Err = fmt.Sprintf("upstream returned %d: %s", resp.StatusCode, string(data))
		}
		return &apiErr
	}

	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("http: decode response: %w", err)
		}
	}
	return nil
}
