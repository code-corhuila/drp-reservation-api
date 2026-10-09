package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"

	"github.com/code-corhuila/drp-reservation-api/internal/domain"
)

type ListFilter struct {
	Actor     Principal
	State     *domain.State
	SpaceID   string
	StartFrom *time.Time
	StartTo   *time.Time
	Page      int
	Limit     int
}

type ReservationPage struct {
	Items      []domain.Reservation
	Page       int
	Limit      int
	Total      int
	TotalPages int
}

type CreateCommand struct {
	Actor          Principal
	SpaceID        string
	StartAt        time.Time
	EndAt          time.Time
	IdempotencyKey string
	RawBody        string
}

type Bookings struct {
	Reservations ReservationRepository
	Spaces       SpaceCatalog
	Idempotency  IdempotencyStore
	Outbox       Outbox
	Now          func() time.Time
	NewID        func() string
}

func (b Bookings) now() time.Time {
	if b.Now == nil {
		return time.Now().UTC()
	}
	return b.Now().UTC()
}

func (b Bookings) newID() string {
	if b.NewID != nil {
		return b.NewID()
	}
	var buf [16]byte
	_, _ = rand.Read(buf[:])
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	h := hex.EncodeToString(buf[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

func (b Bookings) List(ctx context.Context, f ListFilter) (ReservationPage, error) {
	if f.Page < 1 || f.Limit < 1 || f.Limit > 100 {
		return ReservationPage{}, ErrValidation
	}
	all, err := b.Reservations.List(ctx)
	if err != nil {
		return ReservationPage{}, err
	}
	filtered := make([]domain.Reservation, 0, len(all))
	for _, r := range all {
		if !f.Actor.Admin() && r.UserID != f.Actor.Subject {
			continue
		}
		if f.State != nil && r.State != *f.State {
			continue
		}
		if f.SpaceID != "" && r.SpaceID != f.SpaceID {
			continue
		}
		if f.StartFrom != nil && r.Period.Start.Before(*f.StartFrom) {
			continue
		}
		if f.StartTo != nil && !r.Period.Start.Before(*f.StartTo) {
			continue
		}
		filtered = append(filtered, r)
	}
	sort.Slice(filtered, func(i, j int) bool {
		if !filtered[i].Period.Start.Equal(filtered[j].Period.Start) {
			return filtered[i].Period.Start.After(filtered[j].Period.Start)
		}
		return filtered[i].ID > filtered[j].ID
	})
	total := len(filtered)
	totalPages := 0
	if total > 0 {
		totalPages = (total + f.Limit - 1) / f.Limit
	}
	start := (f.Page - 1) * f.Limit
	if start >= total {
		return ReservationPage{Items: []domain.Reservation{}, Page: f.Page, Limit: f.Limit, Total: total, TotalPages: totalPages}, nil
	}
	end := start + f.Limit
	if end > total {
		end = total
	}
	return ReservationPage{Items: filtered[start:end], Page: f.Page, Limit: f.Limit, Total: total, TotalPages: totalPages}, nil
}

func (b Bookings) Get(ctx context.Context, actor Principal, id string) (domain.Reservation, error) {
	r, err := b.Reservations.FindByID(ctx, id)
	if err != nil {
		return domain.Reservation{}, err
	}
	if !actor.Admin() && r.UserID != actor.Subject {
		return domain.Reservation{}, ErrNotFound
	}
	return r, nil
}

func (b Bookings) Create(ctx context.Context, cmd CreateCommand) (domain.Reservation, bool, error) {
	if len(cmd.IdempotencyKey) < 8 || len(cmd.IdempotencyKey) > 128 {
		return domain.Reservation{}, false, ErrValidation
	}
	if existing, r, ok := b.Idempotency.Get(ctx, cmd.Actor.Subject, cmd.IdempotencyKey); ok {
		if existing != cmd.RawBody {
			return domain.Reservation{}, false, ErrConflict
		}
		return r, true, nil
	}
	now := b.now()
	if !cmd.StartAt.After(now) {
		return domain.Reservation{}, false, ErrInPast
	}
	if cmd.EndAt.Sub(cmd.StartAt) > 12*time.Hour {
		return domain.Reservation{}, false, ErrTooLong
	}
	active, err := b.Spaces.Lookup(ctx, cmd.SpaceID)
	if err != nil {
		return domain.Reservation{}, false, err
	}
	if !active {
		return domain.Reservation{}, false, ErrInactiveSpace
	}
	all, err := b.Reservations.List(ctx)
	if err != nil {
		return domain.Reservation{}, false, err
	}
	candidate, err := domain.NewReservation(b.newID(), cmd.Actor.Subject, cmd.SpaceID, cmd.StartAt, cmd.EndAt, now)
	if err != nil {
		return domain.Reservation{}, false, ErrValidation
	}
	for _, other := range all {
		if other.State == domain.StateConfirmed && other.Overlaps(candidate) {
			return domain.Reservation{}, false, ErrOverlap
		}
	}
	if err := b.Reservations.Save(ctx, candidate); err != nil {
		return domain.Reservation{}, false, err
	}
	if err := b.appendEvent(ctx, candidate, domain.EventReservationCreated, domain.TopicReservationCreated); err != nil {
		return domain.Reservation{}, false, err
	}
	b.Idempotency.Put(ctx, cmd.Actor.Subject, cmd.IdempotencyKey, cmd.RawBody, candidate)
	return candidate, false, nil
}

func (b Bookings) Cancel(ctx context.Context, actor Principal, id string) (domain.Reservation, error) {
	r, err := b.Get(ctx, actor, id)
	if err != nil {
		return domain.Reservation{}, err
	}
	now := b.now()
	if !r.Period.Start.After(now.Add(2 * time.Hour)) {
		return domain.Reservation{}, ErrTransition
	}
	got, err := r.Cancel(now)
	if err != nil {
		return domain.Reservation{}, ErrTransition
	}
	if err := b.Reservations.Save(ctx, got); err != nil {
		return domain.Reservation{}, err
	}
	if err := b.appendEvent(ctx, got, domain.EventReservationCancelled, domain.TopicReservationCancelled); err != nil {
		return domain.Reservation{}, err
	}
	return got, nil
}

func (b Bookings) appendEvent(ctx context.Context, r domain.Reservation, eventType, topic string) error {
	if b.Outbox == nil {
		return nil
	}
	payload, err := json.Marshal(map[string]string{
		"reservationId": r.ID,
		"userId":        r.UserID,
		"spaceId":       r.SpaceID,
		"periodStart":   r.Period.Start.UTC().Format(time.RFC3339),
		"periodEnd":     r.Period.End.UTC().Format(time.RFC3339),
		"state":         string(r.State),
	})
	if err != nil {
		return err
	}
	return b.Outbox.Append(ctx, domain.OutboxEvent{
		EventID:       b.newID(),
		EventType:     eventType,
		AggregateID:   r.ID,
		AggregateType: "Reservation",
		Topic:         topic,
		OccurredAt:    b.now(),
		Version:       1,
		PayloadJSON:   string(payload),
	})
}
