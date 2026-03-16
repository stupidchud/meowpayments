package hub

import "time"

// Message is the envelope sent to WebSocket subscribers.
type Message struct {
	// Type is the event name, e.g. "payment.status_changed", "payment.completed".
	Type      string    `json:"type"`
	PaymentID string    `json:"payment_id"`
	Status    string    `json:"status"`
	Data      any       `json:"data,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// Room name helpers - callers use these to avoid typos.
func PaymentRoom(paymentID string) string { return "payment:" + paymentID }
func GlobalRoom() string                  { return "global" }

const (
	TypeStatusChanged = "payment.status_changed"
	TypeCompleted     = "payment.completed"
	TypeRefunded      = "payment.refunded"
	TypeFailed        = "payment.failed"
	TypeExpired       = "payment.expired"
	TypePing          = "ping"
)
