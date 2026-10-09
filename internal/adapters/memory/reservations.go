package memory

import (
	"context"
	"sync"

	"github.com/code-corhuila/drp-reservation-api/internal/app"
	"github.com/code-corhuila/drp-reservation-api/internal/domain"
)

type Reservations struct {
	mu   sync.RWMutex
	byID map[string]domain.Reservation
}

func NewReservations() *Reservations {
	return &Reservations{byID: map[string]domain.Reservation{}}
}

func (r *Reservations) List(_ context.Context) ([]domain.Reservation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]domain.Reservation, 0, len(r.byID))
	for _, v := range r.byID {
		out = append(out, v)
	}
	return out, nil
}

func (r *Reservations) FindByID(_ context.Context, id string) (domain.Reservation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	got, ok := r.byID[id]
	if !ok {
		return domain.Reservation{}, app.ErrNotFound
	}
	return got, nil
}

func (r *Reservations) Save(_ context.Context, reservation domain.Reservation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[reservation.ID] = reservation
	return nil
}

type Spaces struct {
	active map[string]bool
}

func Corte2Spaces() *Spaces {
	return &Spaces{active: map[string]bool{
		"11111111-1111-1111-1111-111111111111": true,
		"22222222-2222-2222-2222-222222222222": true,
		"33333333-3333-3333-3333-333333333333": true,
		"55555555-5555-5555-5555-555555555555": false,
	}}
}

func (s *Spaces) Lookup(_ context.Context, spaceID string) (bool, error) {
	active, ok := s.active[spaceID]
	if !ok {
		return false, app.ErrNotFound
	}
	return active, nil
}

type Idempotency struct {
	mu    sync.Mutex
	items map[string]record
}

type record struct {
	body string
	r    domain.Reservation
}

func NewIdempotency() *Idempotency {
	return &Idempotency{items: map[string]record{}}
}

func key(actor, k string) string { return actor + "\x00" + k }

func (s *Idempotency) Get(_ context.Context, actor, k string) (string, domain.Reservation, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	got, ok := s.items[key(actor, k)]
	if !ok {
		return "", domain.Reservation{}, false
	}
	return got.body, got.r, true
}

func (s *Idempotency) Put(_ context.Context, actor, k, body string, r domain.Reservation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[key(actor, k)] = record{body: body, r: r}
}
