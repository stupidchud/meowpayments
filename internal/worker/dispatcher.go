package worker

import (
	"time"

	"github.com/meowpayments/meowpayments/internal/domain"
	"github.com/meowpayments/meowpayments/internal/hub"
	"github.com/meowpayments/meowpayments/internal/oneclick"
)

// Dispatcher fans out status-change events to the WebSocket hub and the callback sender.
type Dispatcher struct {
	hub      *hub.Hub
	callback *CallbackSender
}

func NewDispatcher(h *hub.Hub, cb *CallbackSender) *Dispatcher {
	return &Dispatcher{hub: h, callback: cb}
}

// OnStatusChange should be called whenever a payment transitions to a new status.
func (d *Dispatcher) OnStatusChange(p *domain.Payment, oldStatus, newStatus oneclick.PaymentStatus) {
	msgType := hub.TypeStatusChanged
	switch newStatus {
	case oneclick.StatusCompleted:
		msgType = hub.TypeCompleted
	case oneclick.StatusRefunded:
		msgType = hub.TypeRefunded
	case oneclick.StatusFailed:
		msgType = hub.TypeFailed
	case oneclick.StatusExpired:
		msgType = hub.TypeExpired
	}

	msg := hub.Message{
		Type:      msgType,
		PaymentID: p.ID.String(),
		Status:    string(newStatus),
		Timestamp: time.Now(),
	}

	// Notify subscribers of this specific payment.
	d.hub.Broadcast(hub.PaymentRoom(p.ID.String()), msg)
	// Notify the global operator room.
	d.hub.Broadcast(hub.GlobalRoom(), msg)

	// Queue callback on every status change.
	if p.CallbackURL != "" {
		d.callback.Enqueue(p)
	}
}
