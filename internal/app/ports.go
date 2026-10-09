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
// Flyway table reservation.outbox is the durable form; memory is the Corte 3 walk.
type Outbox interface {
	Append(ctx context.Context, e domain.OutboxEvent) error
	Unpublished(ctx context.Context) ([]domain.OutboxEvent, error)
}
