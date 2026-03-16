package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/meowpayments/meowpayments/internal/domain"
	"github.com/meowpayments/meowpayments/internal/oneclick"
	"github.com/meowpayments/meowpayments/internal/store"
	"github.com/shopspring/decimal"
)

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("not found")

type PaymentStore struct {
	pool *pgxpool.Pool
}

func NewPaymentStore(pool *pgxpool.Pool) store.PaymentStore {
	return &PaymentStore{pool: pool}
}

func (s *PaymentStore) Create(ctx context.Context, p *domain.Payment) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	now := time.Now().UTC()
	p.CreatedAt = now
	p.UpdatedAt = now

	metaJSON, err := marshalMeta(p.Metadata)
	if err != nil {
		return err
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO payments (
			id, dest_asset_id, dest_chain, dest_address,
			amount_usd, amount_asset, amount_symbol,
			deposit_address, deposit_memo, quote_expires_at,
			status, oneclick_status, failure_reason,
			callback_url, metadata, customer_email,
			expires_at, last_polled_at, poll_failures,
			origin_asset_id,
			created_at, updated_at
		) VALUES (
			$1,  $2,  $3,  $4,
			$5,  $6,  $7,
			$8,  $9,  $10,
			$11, $12, $13,
			$14, $15, $16,
			$17, $18, $19,
			$20,
			$21, $22
		)`,
		p.ID, p.DestAssetID, p.DestChain, p.DestAddress,
		p.AmountUSD, p.AmountAsset, p.AmountSymbol,
		p.DepositAddress, p.DepositMemo, p.QuoteExpiresAt,
		string(p.Status), p.OneclickStatus, p.FailureReason,
		p.CallbackURL, metaJSON, p.CustomerEmail,
		p.ExpiresAt, p.LastPolledAt, p.PollFailures,
		p.OriginAssetID,
		p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func (s *PaymentStore) GetByID(ctx context.Context, id uuid.UUID) (*domain.Payment, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+paymentColumns+` FROM payments WHERE id = $1`, id)
	return scanPayment(row)
}

func (s *PaymentStore) GetByDepositAddress(ctx context.Context, addr string) (*domain.Payment, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+paymentColumns+` FROM payments WHERE deposit_address = $1`, addr)
	return scanPayment(row)
}

func (s *PaymentStore) Update(ctx context.Context, p *domain.Payment) error {
	p.UpdatedAt = time.Now().UTC()
	metaJSON, err := marshalMeta(p.Metadata)
	if err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE payments SET
			dest_asset_id    = $2,
			dest_chain       = $3,
			dest_address     = $4,
			amount_usd       = $5,
			amount_asset     = $6,
			amount_symbol    = $7,
			deposit_address  = $8,
			deposit_memo     = $9,
			quote_expires_at = $10,
			status           = $11,
			oneclick_status  = $12,
			failure_reason   = $13,
			callback_url     = $14,
			metadata         = $15,
			customer_email   = $16,
			expires_at       = $17,
			last_polled_at   = $18,
			poll_failures    = $19,
			origin_asset_id  = $20,
			updated_at       = $21
		WHERE id = $1`,
		p.ID, p.DestAssetID, p.DestChain, p.DestAddress,
		p.AmountUSD, p.AmountAsset, p.AmountSymbol,
		p.DepositAddress, p.DepositMemo, p.QuoteExpiresAt,
		string(p.Status), p.OneclickStatus, p.FailureReason,
		p.CallbackURL, metaJSON, p.CustomerEmail,
		p.ExpiresAt, p.LastPolledAt, p.PollFailures,
		p.OriginAssetID,
		p.UpdatedAt,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PaymentStore) List(ctx context.Context, opts store.ListOpts) ([]*domain.Payment, int64, error) {
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PageSize < 1 {
		opts.PageSize = 20
	}
	offset := (opts.Page - 1) * opts.PageSize

	args := []interface{}{}
	where := "WHERE 1=1"
	argN := 1

	if opts.Status != "" {
		where += fmt.Sprintf(" AND status = $%d", argN)
		args = append(args, opts.Status)
		argN++
	}
	if !opts.After.IsZero() {
		where += fmt.Sprintf(" AND created_at >= $%d", argN)
		args = append(args, opts.After)
		argN++
	}
	if !opts.Before.IsZero() {
		where += fmt.Sprintf(" AND created_at <= $%d", argN)
		args = append(args, opts.Before)
		argN++
	}

	countQuery := "SELECT COUNT(*) FROM payments " + where
	var total int64
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, opts.PageSize, offset)
	dataQuery := fmt.Sprintf(
		"SELECT %s FROM payments %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d",
		paymentColumns, where, argN, argN+1,
	)

	rows, err := s.pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var payments []*domain.Payment
	for rows.Next() {
		p, err := scanPayment(rows)
		if err != nil {
			return nil, 0, err
		}
		payments = append(payments, p)
	}
	return payments, total, rows.Err()
}

func (s *PaymentStore) ListActive(ctx context.Context) ([]*domain.Payment, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+paymentColumns+`
		FROM payments
		WHERE status NOT IN ('COMPLETED','EXPIRED','REFUNDED','FAILED')
		  AND deposit_address IS NOT NULL
		ORDER BY last_polled_at NULLS FIRST
		LIMIT 500`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payments []*domain.Payment
	for rows.Next() {
		p, err := scanPayment(rows)
		if err != nil {
			return nil, err
		}
		payments = append(payments, p)
	}
	return payments, rows.Err()
}

func (s *PaymentStore) AppendEvent(ctx context.Context, e *domain.PaymentEvent) error {
	metaJSON, _ := json.Marshal(e.Payload)
	return s.pool.QueryRow(ctx, `
		INSERT INTO payment_events (payment_id, event_type, old_status, new_status, payload)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`,
		e.PaymentID, e.EventType, string(e.OldStatus), string(e.NewStatus), metaJSON,
	).Scan(&e.ID, &e.CreatedAt)
}

func (s *PaymentStore) GetEvents(ctx context.Context, paymentID uuid.UUID) ([]*domain.PaymentEvent, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, payment_id, event_type, old_status, new_status, payload, created_at
		FROM payment_events
		WHERE payment_id = $1
		ORDER BY created_at ASC`, paymentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*domain.PaymentEvent
	for rows.Next() {
		var e domain.PaymentEvent
		var payload []byte
		var oldStatus, newStatus string
		if err := rows.Scan(&e.ID, &e.PaymentID, &e.EventType, &oldStatus, &newStatus, &payload, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.OldStatus = oneclick.PaymentStatus(oldStatus)
		e.NewStatus = oneclick.PaymentStatus(newStatus)
		if len(payload) > 0 {
			_ = json.Unmarshal(payload, &e.Payload)
		}
		events = append(events, &e)
	}
	return events, rows.Err()
}

func (s *PaymentStore) MarkExpired(ctx context.Context, before time.Time) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE payments
		SET status = 'EXPIRED', updated_at = now()
		WHERE expires_at < $1
		  AND status IN ('PENDING', 'AWAITING_DEPOSIT')`,
		before,
	)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// helpers

const paymentColumns = `
	id, dest_asset_id, dest_chain, dest_address,
	amount_usd, amount_asset, amount_symbol,
	deposit_address, deposit_memo, quote_expires_at,
	status, oneclick_status, failure_reason,
	callback_url, metadata, customer_email,
	expires_at, last_polled_at, poll_failures,
	origin_asset_id,
	created_at, updated_at`

type scannable interface {
	Scan(dest ...interface{}) error
}

func scanPayment(row scannable) (*domain.Payment, error) {
	var p domain.Payment
	var status string
	var metaRaw []byte
	var amtUSD, amtAsset *decimal.Decimal
	var originAssetID *string

	err := row.Scan(
		&p.ID, &p.DestAssetID, &p.DestChain, &p.DestAddress,
		&amtUSD, &amtAsset, &p.AmountSymbol,
		&p.DepositAddress, &p.DepositMemo, &p.QuoteExpiresAt,
		&status, &p.OneclickStatus, &p.FailureReason,
		&p.CallbackURL, &metaRaw, &p.CustomerEmail,
		&p.ExpiresAt, &p.LastPolledAt, &p.PollFailures,
		&originAssetID,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan payment: %w", err)
	}
	p.Status = oneclick.PaymentStatus(status)
	p.AmountUSD = amtUSD
	p.AmountAsset = amtAsset
	if originAssetID != nil {
		p.OriginAssetID = *originAssetID
	}

	if len(metaRaw) > 0 {
		_ = json.Unmarshal(metaRaw, &p.Metadata)
	}
	return &p, nil
}

func marshalMeta(m map[string]any) ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}
	return b, nil
}
