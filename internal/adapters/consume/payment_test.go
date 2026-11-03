package consume

import (
	"testing"

	"github.com/code-corhuila/drp-reservation-api/internal/domain"
)

func TestFromPaymentOutbox(t *testing.T) {
	got, err := FromPaymentOutbox(
		"PaymentConfirmed",
		"77777777-7777-4777-8777-777777777777",
		`{"paymentId":"88888888-8888-4888-8888-888888888888","reservationId":"22222222-2222-2222-2222-222222222222","amountCents":5000000,"currency":"COP"}`,
		"trace-1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if got.EventType != domain.EventPaymentConfirmed || got.ReservationID != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("%+v", got)
	}
}
