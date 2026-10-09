package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/code-corhuila/drp-reservation-api/internal/adapters/memory"
	"github.com/code-corhuila/drp-reservation-api/internal/app"
	"github.com/code-corhuila/drp-reservation-api/internal/domain"
)

func TestCreateAppendsReservationCreatedToOutbox(t *testing.T) {
	fixed := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	box := memory.NewOutbox()
	svc := app.Bookings{
		Reservations: memory.NewReservations(),
		Spaces:       memory.Corte2Spaces(),
		Idempotency:  memory.NewIdempotency(),
		Outbox:       box,
		Now:          func() time.Time { return fixed },
		NewID: func() string {
			return "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		},
	}
	got, replay, err := svc.Create(context.Background(), app.CreateCommand{
		Actor:          app.Principal{Subject: "44444444-4444-4444-4444-444444444444", Role: "USER"},
		SpaceID:        "11111111-1111-1111-1111-111111111111",
		StartAt:        fixed.Add(3 * time.Hour),
		EndAt:          fixed.Add(4 * time.Hour),
		IdempotencyKey: "idem-key-create-outbox",
		RawBody:        `{"spaceId":"11111111-1111-1111-1111-111111111111"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if replay {
		t.Fatal("expected insert")
	}
	if got.State != domain.StatePaymentPending {
		t.Fatalf("state %s", got.State)
	}
	unpublished, err := box.Unpublished(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(unpublished) != 1 {
		t.Fatalf("outbox %d", len(unpublished))
	}
	if unpublished[0].EventType != domain.EventReservationCreated {
		t.Fatalf("event %s", unpublished[0].EventType)
	}
	if unpublished[0].Topic != domain.TopicReservationCreated {
		t.Fatalf("topic %s", unpublished[0].Topic)
	}
	if unpublished[0].Published {
		t.Fatal("must start unpublished")
	}
}
