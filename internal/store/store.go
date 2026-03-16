// Package store defines repository interfaces for meowpayments persistence.
package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/meowpayments/meowpayments/internal/domain"
)

// PaymentStore is the persistence interface for payments.
type PaymentStore interface {
	// Create inserts a new payment. Sets p.ID if empty.
	Create(ctx context.Context, p *domain.Payment) error
	// GetByID retrieves a payment by its ID.
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Payment, error)
	// GetByDepositAddress retrieves a payment by its 1-click deposit address.
	GetByDepositAddress(ctx context.Context, addr string) (*domain.Payment, error)
	// Update persists changes to an existing payment.
	Update(ctx context.Context, p *domain.Payment) error
	// List returns payments with optional filters and pagination.
	List(ctx context.Context, opts ListOpts) ([]*domain.Payment, int64, error)
	// ListActive returns all non-terminal payments that have a deposit address.
	// Used by the background poller.
	ListActive(ctx context.Context) ([]*domain.Payment, error)
	// AppendEvent appends an audit event for a payment.
	AppendEvent(ctx context.Context, e *domain.PaymentEvent) error
	// GetEvents returns all events for a payment in chronological order.
	GetEvents(ctx context.Context, paymentID uuid.UUID) ([]*domain.PaymentEvent, error)
	// MarkExpired bulk-transitions AWAITING_DEPOSIT payments past their expiry
	// to EXPIRED. Returns the number of rows updated.
	MarkExpired(ctx context.Context, before time.Time) (int64, error)
}

// ListOpts controls pagination and filtering for List queries.
type ListOpts struct {
	Page     int
	PageSize int
	Status   string    // optional; empty = all
	Before   time.Time // optional; zero = no upper bound
	After    time.Time // optional; zero = no lower bound
}
