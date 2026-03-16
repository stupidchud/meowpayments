package handlers

import (
	"net/http"

	"github.com/go-fuego/fuego"
	"github.com/google/uuid"
	"github.com/meowpayments/meowpayments/internal/hub"
	"github.com/meowpayments/meowpayments/internal/store"
	"github.com/meowpayments/meowpayments/internal/store/postgres"
	"nhooyr.io/websocket"
)

type WSHandlers struct {
	hub          *hub.Hub
	paymentStore store.PaymentStore
}

func NewWSHandlers(h *hub.Hub, ps store.PaymentStore) *WSHandlers {
	return &WSHandlers{hub: h, paymentStore: ps}
}

// Global handles GET /v1/ws - operator WebSocket.
// Authenticated via X-API-Key or ?token= query param (WS browsers can't set headers).
// Receives all payment events.
func (h *WSHandlers) Global(c fuego.ContextNoBody) (any, error) {
	ws, err := websocket.Accept(c.Response(), c.Request(), &websocket.AcceptOptions{
		InsecureSkipVerify: false,
		CompressionMode:    websocket.CompressionContextTakeover,
	})
	if err != nil {
		return nil, fuego.HTTPError{Err: err, Status: http.StatusBadRequest}
	}
	defer ws.CloseNow() //nolint:errcheck

	conn := h.hub.Register(ws, hub.GlobalRoom())
	defer h.hub.Unregister(conn)

	// Block until the client disconnects or context is done.
	<-c.Context().Done()
	_ = ws.Close(websocket.StatusNormalClosure, "server shutting down")
	return nil, nil
}

// Payment handles GET /v1/pay/:id/ws - customer WebSocket for a specific payment.
// No authentication required - keyed by the opaque payment ID.
func (h *WSHandlers) Payment(c fuego.ContextNoBody) (any, error) {
	id := c.PathParam("id")
	if id == "" {
		return nil, fuego.HTTPError{Status: http.StatusBadRequest, Title: "missing payment id"}
	}

	// Verify the payment exists.
	parsedID, parseErr := uuid.Parse(id)
	if parseErr != nil {
		return nil, fuego.HTTPError{Status: http.StatusBadRequest, Title: "invalid payment id"}
	}
	_, err := h.paymentStore.GetByID(c.Context(), parsedID)
	if err != nil {
		if err == postgres.ErrNotFound {
			return nil, fuego.HTTPError{Status: http.StatusNotFound, Title: "payment not found"}
		}
		return nil, fuego.HTTPError{Err: err, Status: http.StatusInternalServerError}
	}

	ws, err := websocket.Accept(c.Response(), c.Request(), &websocket.AcceptOptions{
		CompressionMode: websocket.CompressionContextTakeover,
	})
	if err != nil {
		return nil, fuego.HTTPError{Err: err, Status: http.StatusBadRequest}
	}
	defer ws.CloseNow() //nolint:errcheck

	conn := h.hub.Register(ws, hub.PaymentRoom(id))
	defer h.hub.Unregister(conn)

	<-c.Context().Done()
	_ = ws.Close(websocket.StatusNormalClosure, "")
	return nil, nil
}
