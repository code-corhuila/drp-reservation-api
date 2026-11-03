package consume

import (
	"encoding/json"
	"strings"

	"github.com/code-corhuila/drp-reservation-api/internal/app"
	"github.com/code-corhuila/drp-reservation-api/internal/domain"
)

type payload struct {
	PaymentID     string `json:"paymentId"`
	ReservationID string `json:"reservationId"`
}

// FromPaymentOutbox maps the walking payment outbox row onto ApplyPayment.
func FromPaymentOutbox(eventType, eventID, payloadJSON, correlationID string) (app.PaymentOutcome, error) {
	var body payload
	if err := json.Unmarshal([]byte(payloadJSON), &body); err != nil {
		return app.PaymentOutcome{}, err
	}
	kind := strings.TrimSpace(eventType)
	if kind != domain.EventPaymentConfirmed && kind != domain.EventPaymentFailed {
		return app.PaymentOutcome{}, app.ErrValidation
	}
	return app.PaymentOutcome{
		EventID:       eventID,
		EventType:     kind,
		ReservationID: body.ReservationID,
		PaymentID:     body.PaymentID,
		CorrelationID: correlationID,
	}, nil
}
