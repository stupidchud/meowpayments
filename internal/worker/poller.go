package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/meowpayments/meowpayments/internal/config"
	"github.com/meowpayments/meowpayments/internal/domain"
	"github.com/meowpayments/meowpayments/internal/oneclick"
	"github.com/meowpayments/meowpayments/internal/store"
)

// Poller is the background worker that polls the 1-click API for payment status updates.
type Poller struct {
	store      store.PaymentStore
	oc         oneclick.Client
	dispatcher *Dispatcher
	cfg        config.PollerConfig
	log        *slog.Logger
}

func NewPoller(
	store store.PaymentStore,
	oc oneclick.Client,
	dispatcher *Dispatcher,
	cfg config.PollerConfig,
	log *slog.Logger,
) *Poller {
	return &Poller{
		store:      store,
		oc:         oc,
		dispatcher: dispatcher,
		cfg:        cfg,
		log:        log,
	}
}

// Run starts the polling loop and blocks until ctx is cancelled.
func (p *Poller) Run(ctx context.Context) {
	fastTicker := time.NewTicker(p.cfg.FastInterval)
	expiryTicker := time.NewTicker(60 * time.Second)
	defer fastTicker.Stop()
	defer expiryTicker.Stop()

	// Poll immediately on startup.
	p.pollAll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-fastTicker.C:
			p.pollAll(ctx)
		case <-expiryTicker.C:
			p.reapExpired(ctx)
		}
	}
}

func (p *Poller) pollAll(ctx context.Context) {
	payments, err := p.store.ListActive(ctx)
	if err != nil {
		p.log.Error("poller: list active payments", "err", err)
		return
	}
	for _, payment := range payments {
		if ctx.Err() != nil {
			return
		}
		p.pollOne(ctx, payment)
	}
}

func (p *Poller) pollOne(ctx context.Context, payment *domain.Payment) {
	now := time.Now().UTC()
	payment.LastPolledAt = &now

	status, err := p.oc.Status(ctx, payment.DepositAddress)
	if err != nil {
		payment.PollFailures++
		p.log.Warn("poller: status check failed",
			"payment_id", payment.ID,
			"deposit_address", payment.DepositAddress,
			"failures", payment.PollFailures,
			"err", err,
		)
		if payment.PollFailures >= p.cfg.MaxPollFailures {
			oldStatus := payment.Status
			payment.Status = oneclick.StatusFailed
			payment.FailureReason = "too many poll failures: " + err.Error()
			_ = p.store.Update(ctx, payment)
			_ = p.store.AppendEvent(ctx, &domain.PaymentEvent{
				PaymentID: payment.ID,
				EventType: domain.EventError,
				OldStatus: oldStatus,
				NewStatus: payment.Status,
				Payload:   map[string]any{"reason": payment.FailureReason},
			})
			p.dispatcher.OnStatusChange(payment, oldStatus, payment.Status)
		} else {
			_ = p.store.Update(ctx, payment)
		}
		return
	}

	payment.PollFailures = 0
	payment.OneclickStatus = status.Raw

	if status.Mapped != payment.Status {
		oldStatus := payment.Status
		payment.Status = status.Mapped

		if err := p.store.Update(ctx, payment); err != nil {
			p.log.Error("poller: update payment", "payment_id", payment.ID, "err", err)
			return
		}
		_ = p.store.AppendEvent(ctx, &domain.PaymentEvent{
			PaymentID: payment.ID,
			EventType: domain.EventStatusChange,
			OldStatus: oldStatus,
			NewStatus: payment.Status,
			Payload:   map[string]any{"oneclick_status": status.Raw},
		})
		p.log.Info("payment status changed",
			"payment_id", payment.ID,
			"old", oldStatus,
			"new", payment.Status,
		)
		p.dispatcher.OnStatusChange(payment, oldStatus, payment.Status)
	} else {
		// No status change - just update last_polled_at.
		_ = p.store.Update(ctx, payment)
	}
}

func (p *Poller) reapExpired(ctx context.Context) {
	n, err := p.store.MarkExpired(ctx, time.Now().UTC())
	if err != nil {
		p.log.Error("poller: reap expired", "err", err)
		return
	}
	if n > 0 {
		p.log.Info("poller: expired payments reaped", "count", n)
	}
}
