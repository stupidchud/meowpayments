package worker

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/meowpayments/meowpayments/internal/config"
	"github.com/meowpayments/meowpayments/internal/domain"
)

// callbackPayload is the JSON body POSTed to a payment's callback URL.
type callbackPayload struct {
	EventType string         `json:"event_type"`
	PaymentID string         `json:"payment_id"`
	Status    string         `json:"status"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// CallbackSender delivers HTTP callbacks to configured callback URLs.
type CallbackSender struct {
	queue      chan *domain.Payment
	hc         *http.Client
	secret     string
	maxRetries int
	log        *slog.Logger
}

func NewCallbackSender(cfg config.WorkerConfig, log *slog.Logger) *CallbackSender {
	return &CallbackSender{
		queue:      make(chan *domain.Payment, 256),
		hc:         &http.Client{Timeout: cfg.CallbackTimeout},
		secret:     cfg.WebhookSecret,
		maxRetries: cfg.CallbackMaxRetries,
		log:        log,
	}
}

// Enqueue queues a payment for callback delivery.
func (s *CallbackSender) Enqueue(p *domain.Payment) {
	select {
	case s.queue <- p:
	default:
		s.log.Warn("callback queue full, dropping", "payment_id", p.ID)
	}
}

// Run launches a pool of workers and blocks until ctx is cancelled.
func (s *CallbackSender) Run(ctx context.Context, workers int) {
	for i := 0; i < workers; i++ {
		go s.worker(ctx)
	}
	<-ctx.Done()
}

func (s *CallbackSender) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case p, ok := <-s.queue:
			if !ok {
				return
			}
			s.deliver(ctx, p)
		}
	}
}

func (s *CallbackSender) deliver(ctx context.Context, p *domain.Payment) {
	if p.CallbackURL == "" {
		return
	}

	payload := callbackPayload{
		EventType: "payment." + string(p.Status),
		PaymentID: p.ID.String(),
		Status:    string(p.Status),
		Metadata:  p.Metadata,
		Timestamp: time.Now().UTC(),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		s.log.Error("callback: marshal payload", "err", err)
		return
	}

	sig := s.sign(body)
	delay := 500 * time.Millisecond

	for attempt := 0; attempt < s.maxRetries; attempt++ {
		if ctx.Err() != nil {
			return
		}
		err = s.post(ctx, p.CallbackURL, body, sig)
		if err == nil {
			s.log.Info("callback delivered", "payment_id", p.ID, "attempt", attempt+1)
			return
		}
		s.log.Warn("callback failed", "payment_id", p.ID, "attempt", attempt+1, "err", err)
		if attempt < s.maxRetries-1 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
			delay *= 2
		}
	}
	s.log.Error("callback exhausted retries", "payment_id", p.ID, "url", p.CallbackURL)
}

func (s *CallbackSender) post(ctx context.Context, url string, body []byte, sig string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature-SHA256", sig)

	resp, err := s.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("callback: upstream returned %d", resp.StatusCode)
	}
	return nil
}

func (s *CallbackSender) sign(body []byte) string {
	mac := hmac.New(sha256.New, []byte(s.secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
