package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/code-corhuila/drp-reservation-api/internal/domain"
)

// Outbox maps Flyway reservation.outbox (V003). No DDL here (Anexo J). No AMQP.
type Outbox struct {
	DB *sql.DB
}

func NewOutbox(db *sql.DB) *Outbox {
	return &Outbox{DB: db}
}

func (o *Outbox) Append(ctx context.Context, e domain.OutboxEvent) error {
	_, err := o.DB.ExecContext(ctx, `
INSERT INTO outbox (
  event_id, event_type, aggregate_id, aggregate_type, topic, payload, occurred_at, published
) VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7, FALSE)`,
		e.EventID, e.EventType, e.AggregateID, e.AggregateType, e.Topic, e.PayloadJSON, e.OccurredAt.UTC())
	return err
}

func (o *Outbox) Unpublished(ctx context.Context) ([]domain.OutboxEvent, error) {
	rows, err := o.DB.QueryContext(ctx, `
SELECT event_id::text, event_type, aggregate_id::text, aggregate_type, topic, payload::text, occurred_at
FROM outbox
WHERE published = FALSE
ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.OutboxEvent{}
	for rows.Next() {
		var e domain.OutboxEvent
		var at time.Time
		if err := rows.Scan(&e.EventID, &e.EventType, &e.AggregateID, &e.AggregateType, &e.Topic, &e.PayloadJSON, &at); err != nil {
			return nil, err
		}
		e.OccurredAt = at.UTC()
		e.Version = 1
		out = append(out, e)
	}
	return out, rows.Err()
}

func (o *Outbox) MarkPublished(ctx context.Context, eventID string) error {
	_, err := o.DB.ExecContext(ctx, `
UPDATE outbox SET published = TRUE, published_at = NOW() WHERE event_id = $1`, eventID)
	return err
}
