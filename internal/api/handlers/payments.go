package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-fuego/fuego"
	"github.com/google/uuid"
	"github.com/meowpayments/meowpayments/internal/api/dto"
	"github.com/meowpayments/meowpayments/internal/config"
	"github.com/meowpayments/meowpayments/internal/domain"
	"github.com/meowpayments/meowpayments/internal/oneclick"
	"github.com/meowpayments/meowpayments/internal/store"
	"github.com/meowpayments/meowpayments/internal/store/postgres"
	"github.com/shopspring/decimal"
)

// PaymentHandlers groups all payment-related Fuego handlers.
type PaymentHandlers struct {
	store      store.PaymentStore
	oc         oneclick.Client
	tokenCache *oneclick.TokenCache
	cfg        *config.Config
}

func NewPaymentHandlers(
	store store.PaymentStore,
	oc oneclick.Client,
	tokenCache *oneclick.TokenCache,
	cfg *config.Config,
) *PaymentHandlers {
	return &PaymentHandlers{store: store, oc: oc, tokenCache: tokenCache, cfg: cfg}
}

// Create handles POST /v1/payments.
// It calls the 1-click API to generate a deposit address and persists the payment.
func (h *PaymentHandlers) Create(c fuego.ContextWithBody[dto.CreatePaymentRequest]) (dto.PaymentResponse, error) {
	req, err := c.Body()
	if err != nil {
		return dto.PaymentResponse{}, fuego.HTTPError{Err: err, Status: http.StatusBadRequest}
	}

	expiresIn := time.Duration(req.ExpiresInSeconds) * time.Second
	if expiresIn <= 0 {
		expiresIn = time.Hour
	}
	deadline := time.Now().UTC().Add(expiresIn)

	slippage := h.cfg.OneClick.DefaultSlippageBPS
	if req.SlippageBPS != nil {
		slippage = *req.SlippageBPS
	}

	var originAmountUSD float64
	if req.AmountUSD != nil {
		originAmountUSD, _ = strconv.ParseFloat(*req.AmountUSD, 64)
	}

	quote, err := h.oc.Quote(c.Context(), oneclick.QuoteParams{
		DestinationAsset:    req.DestAssetID,
		OriginAsset:         req.OriginAssetID,
		OriginRefundAddress: req.OriginRefundAddress,
		OriginAmountUSD:     originAmountUSD,
		RecipientAddress:    req.DestAddress,
		RecipientType:       "DESTINATION_CHAIN",
		RefundAddress:       h.cfg.OneClick.RefundAddress,
		Deadline:            deadline,
		SlippageBPS:         slippage,
		DryRun:              false,
	})
	if err != nil {
		return dto.PaymentResponse{}, fuego.HTTPError{
			Err:    err,
			Status: http.StatusBadGateway,
			Title:  "failed to obtain quote from NEAR Intents: " + err.Error(),
		}
	}

	payment := &domain.Payment{
		DestAssetID:    req.DestAssetID,
		DestChain:      req.DestChain,
		DestAddress:    req.DestAddress,
		OriginAssetID:  req.OriginAssetID,
		DepositAddress: quote.DepositAddress,
		DepositMemo:    quote.DepositMemo,
		Status:         oneclick.StatusAwaitingDeposit,
		CallbackURL:    req.CallbackURL,
		CustomerEmail:  req.CustomerEmail,
		Metadata:       req.Metadata,
		ExpiresAt:      deadline,
	}
	if t := quote.Deadline; !t.IsZero() {
		payment.QuoteExpiresAt = &t
	}
	if req.AmountUSD != nil {
		d, err := decimal.NewFromString(*req.AmountUSD)
		if err == nil {
			payment.AmountUSD = &d
		}
	}

	if err := h.store.Create(c.Context(), payment); err != nil {
		return dto.PaymentResponse{}, fuego.HTTPError{Err: err, Status: http.StatusInternalServerError}
	}
	_ = h.store.AppendEvent(c.Context(), &domain.PaymentEvent{
		PaymentID: payment.ID,
		EventType: domain.EventStatusChange,
		NewStatus: payment.Status,
	})

	return toPaymentResponse(payment, h.cfg.HTTP.BaseURL), nil
}

// Get handles GET /v1/payments/:id.
func (h *PaymentHandlers) Get(c fuego.ContextNoBody) (dto.PaymentResponse, error) {
	id, err := parseUUID(c, "id")
	if err != nil {
		return dto.PaymentResponse{}, err
	}
	p, err := h.store.GetByID(c.Context(), id)
	if err != nil {
		return dto.PaymentResponse{}, notFound(err)
	}
	return toPaymentResponse(p, h.cfg.HTTP.BaseURL), nil
}

// GetPublic handles GET /v1/pay/:id - unauthenticated, for customers.
// Returns a subset of fields suitable for a payment page.
func (h *PaymentHandlers) GetPublic(c fuego.ContextNoBody) (dto.PaymentResponse, error) {
	id, err := parseUUID(c, "id")
	if err != nil {
		return dto.PaymentResponse{}, err
	}
	p, err := h.store.GetByID(c.Context(), id)
	if err != nil {
		return dto.PaymentResponse{}, notFound(err)
	}
	resp := toPaymentResponse(p, h.cfg.HTTP.BaseURL)
	// Strip internal fields from public response.
	resp.CallbackURL = ""
	resp.Metadata = nil
	return resp, nil
}

// List handles GET /v1/payments.
func (h *PaymentHandlers) List(c fuego.ContextNoBody) (dto.PaymentListResponse, error) {
	opts := store.ListOpts{
		Page:     queryInt(c.Request(), "page", 1),
		PageSize: queryInt(c.Request(), "page_size", 20),
		Status:   c.Request().URL.Query().Get("status"),
	}
	payments, total, err := h.store.List(c.Context(), opts)
	if err != nil {
		return dto.PaymentListResponse{}, fuego.HTTPError{Err: err, Status: http.StatusInternalServerError}
	}

	items := make([]*dto.PaymentResponse, len(payments))
	for i, p := range payments {
		resp := toPaymentResponse(p, h.cfg.HTTP.BaseURL)
		items[i] = &resp
	}
	return dto.PaymentListResponse{
		Payments: items,
		Total:    total,
		Page:     opts.Page,
		PageSize: opts.PageSize,
	}, nil
}

// Cancel handles DELETE /v1/payments/:id.
func (h *PaymentHandlers) Cancel(c fuego.ContextNoBody) (dto.PaymentResponse, error) {
	id, err := parseUUID(c, "id")
	if err != nil {
		return dto.PaymentResponse{}, err
	}
	p, err := h.store.GetByID(c.Context(), id)
	if err != nil {
		return dto.PaymentResponse{}, notFound(err)
	}
	if p.IsTerminal() {
		return dto.PaymentResponse{}, fuego.HTTPError{
			Status: http.StatusConflict,
			Title:  "payment is already in a terminal state",
		}
	}
	oldStatus := p.Status
	p.Status = oneclick.StatusFailed
	p.FailureReason = "cancelled by operator"
	if err := h.store.Update(c.Context(), p); err != nil {
		return dto.PaymentResponse{}, fuego.HTTPError{Err: err, Status: http.StatusInternalServerError}
	}
	_ = h.store.AppendEvent(c.Context(), &domain.PaymentEvent{
		PaymentID: p.ID,
		EventType: domain.EventStatusChange,
		OldStatus: oldStatus,
		NewStatus: p.Status,
		Payload:   map[string]any{"reason": "cancelled by operator"},
	})
	return toPaymentResponse(p, h.cfg.HTTP.BaseURL), nil
}

// GetEvents handles GET /v1/payments/:id/events.
func (h *PaymentHandlers) GetEvents(c fuego.ContextNoBody) (any, error) {
	id, err := parseUUID(c, "id")
	if err != nil {
		return nil, err
	}
	events, err := h.store.GetEvents(c.Context(), id)
	if err != nil {
		return nil, fuego.HTTPError{Err: err, Status: http.StatusInternalServerError}
	}
	return map[string]any{"events": events}, nil
}

// SubmitDeposit handles POST /v1/pay/:id/submit - customer submits tx hash.
func (h *PaymentHandlers) SubmitDeposit(c fuego.ContextWithBody[dto.SubmitDepositRequest]) (any, error) {
	id, err := parseUUID(c, "id")
	if err != nil {
		return nil, err
	}
	req, err := c.Body()
	if err != nil {
		return nil, fuego.HTTPError{Err: err, Status: http.StatusBadRequest}
	}

	p, err := h.store.GetByID(c.Context(), id)
	if err != nil {
		return nil, notFound(err)
	}
	if p.DepositAddress == "" {
		return nil, fuego.HTTPError{Status: http.StatusBadRequest, Title: "payment has no deposit address"}
	}

	if err := h.oc.SubmitDeposit(c.Context(), p.DepositAddress, req.TxHash); err != nil {
		// Non-fatal - log and continue.
		_ = err
	}
	return map[string]any{"ok": true}, nil
}

// helpers

func toPaymentResponse(p *domain.Payment, baseURL string) dto.PaymentResponse {
	resp := dto.PaymentResponse{
		ID:             p.ID.String(),
		Status:         string(p.Status),
		DepositAddress: p.DepositAddress,
		DepositMemo:    p.DepositMemo,
		DestAssetID:    p.DestAssetID,
		DestChain:      p.DestChain,
		DestAddress:    p.DestAddress,
		OriginAssetID:  p.OriginAssetID,
		PaymentURL:     p.PaymentURL(baseURL),
		ExpiresAt:      p.ExpiresAt,
		CallbackURL:    p.CallbackURL,
		CustomerEmail:  p.CustomerEmail,
		Metadata:       p.Metadata,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
	if p.AmountUSD != nil {
		s := p.AmountUSD.StringFixed(8)
		resp.AmountUSD = &s
	}
	return resp
}

func parseUUID(c interface{ PathParam(string) string }, key string) (uuid.UUID, error) {
	raw := c.PathParam(key)
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fuego.HTTPError{
			Status: http.StatusBadRequest,
			Title:  "invalid " + key + " - must be a UUID",
		}
	}
	return id, nil
}

func notFound(err error) fuego.HTTPError {
	if errors.Is(err, postgres.ErrNotFound) {
		return fuego.HTTPError{Status: http.StatusNotFound, Title: "payment not found"}
	}
	return fuego.HTTPError{Err: err, Status: http.StatusInternalServerError}
}

func queryInt(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return def
	}
	return n
}
