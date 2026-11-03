package app

import (
	"context"

	"github.com/code-corhuila/drp-reservation-api/internal/domain"
)

type Principal struct {
	Subject string
	Role    string
}

func (p Principal) Admin() bool { return p.Role == "ADMIN" }

type ReservationRepository interface {
	List(ctx context.Context) ([]domain.Reservation, error)
	FindByID(ctx context.Context, id string) (domain.Reservation, error)
	Save(ctx context.Context, r domain.Reservation) error
}

type SpaceCatalog interface {
	Lookup(ctx context.Context, spaceID string) (active bool, err error)
}

type IdempotencyStore interface {
	Get(ctx context.Context, actor, key string) (body string, r domain.Reservation, ok bool)
	Put(ctx context.Context, actor, key, body string, r domain.Reservation)
}

type TokenVerifier interface {
	Parse(ctx context.Context, raw string) (Principal, error)
}

// Outbox records domain events in the same use-case as the aggregate write.
// DATABASE_URL uses Flyway reservation.outbox (V003); empty DSN keeps memory.
type Outbox interface {
	Append(ctx context.Context, e domain.OutboxEvent) error
	Unpublished(ctx context.Context) ([]domain.OutboxEvent, error)
	MarkPublished(ctx context.Context, eventID string) error
}

// UnitOfWork is instructor-norm: outbox in the same DB transaction as the aggregate.
type UnitOfWork interface {
	Within(ctx context.Context, fn func(ctx context.Context) error) error
}

// EventPublisher is the walking HTTP sink. AMQP replaces it later.
type EventPublisher interface {
	Publish(ctx context.Context, e domain.OutboxEvent) error
}

// Inbox de-duplicates consumed events by eventId (BR-006 / E-23).
type Inbox interface {
	Seen(ctx context.Context, eventID string) bool
	Mark(ctx context.Context, eventID string)
}
