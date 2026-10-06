package domain

import (
	"time"
)

type DateTimeRange struct {
	Start time.Time
	End   time.Time
}

func NewDateTimeRange(start, end time.Time) (DateTimeRange, error) {
	if start.IsZero() || end.IsZero() || !end.After(start) {
		return DateTimeRange{}, ErrInvalidInput
	}
	return DateTimeRange{Start: start.UTC(), End: end.UTC()}, nil
}

func (r DateTimeRange) Overlaps(other DateTimeRange) bool {
	return r.Start.Before(other.End) && other.Start.Before(r.End)
}

type State string

const (
	StatePaymentPending State = "PAYMENT_PENDING"
	StateConfirmed      State = "CONFIRMED"
	StateCancelled      State = "CANCELLED"
)

// Reservation is the booking aggregate. userId/spaceId are copies, not FKs.
type Reservation struct {
	ID        string
	UserID    string
	SpaceID   string
	Period    DateTimeRange
	State     State
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewReservation(id, userID, spaceID string, start, end, now time.Time) (Reservation, error) {
	p, err := NewDateTimeRange(start, end)
	if err != nil {
		return Reservation{}, err
	}
	if id == "" || userID == "" || spaceID == "" {
		return Reservation{}, ErrInvalidInput
	}
	return Reservation{
		ID: id, UserID: userID, SpaceID: spaceID, Period: p,
		State: StatePaymentPending, CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (r Reservation) Confirm(now time.Time) (Reservation, error) {
	if r.State != StatePaymentPending {
		return Reservation{}, ErrInvalidTransition
	}
	r.State = StateConfirmed
	r.UpdatedAt = now
	return r, nil
}

func (r Reservation) Cancel(now time.Time) (Reservation, error) {
	if r.State == StateCancelled {
		return Reservation{}, ErrInvalidTransition
	}
	r.State = StateCancelled
	r.UpdatedAt = now
	return r, nil
}

func (r Reservation) Overlaps(other Reservation) bool {
	if r.SpaceID != other.SpaceID {
		return false
	}
	return r.Period.Overlaps(other.Period)
}

// ConfirmedOverlap is BR-001. PAYMENT_PENDING must not block (DEC-001).
func ConfirmedOverlap(a, b Reservation) bool {
	return a.State == StateConfirmed && b.State == StateConfirmed && a.ID != b.ID && a.Overlaps(b)
}
