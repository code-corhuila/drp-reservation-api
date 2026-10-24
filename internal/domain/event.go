package domain

import "time"

const (
	EventReservationCreated    = "ReservationCreated"
	EventReservationConfirmed  = "ReservationConfirmed"
	EventReservationCancelled  = "ReservationCancelled"
	EventPaymentConfirmed      = "PaymentConfirmed"
	EventPaymentFailed         = "PaymentFailed"
	TopicReservationCreated    = "reservation.created"
	TopicReservationConfirmed  = "reservation.confirmed"
	TopicReservationCancelled  = "reservation.cancelled"
)

// OutboxEvent is a fact already stored with the aggregate write (norma: same transaction).
type OutboxEvent struct {
	EventID       string
	EventType     string
	AggregateID   string
	AggregateType string
	Topic         string
	OccurredAt    time.Time
	Version       int
	PayloadJSON   string
	Published     bool
}
