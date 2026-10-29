package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/code-corhuila/drp-reservation-api/internal/adapters/memory"
	"github.com/code-corhuila/drp-reservation-api/internal/app"
	"github.com/code-corhuila/drp-reservation-api/internal/domain"
)

const (
	member      = "44444444-4444-4444-4444-444444444444"
	norte       = "11111111-1111-1111-1111-111111111111"
	reservation = "22222222-2222-2222-2222-222222222222"
	eventOK     = "77777777-7777-4777-8777-777777777777"
	paymentOK   = "88888888-8888-4888-8888-888888888888"
)

func consumeSvc(t *testing.T) (app.Bookings, *memory.Reservations, *memory.Outbox) {
	t.Helper()
	fixed := time.Date(2026, 10, 24, 17, 0, 0, 0, time.UTC)
	store := memory.NewReservations()
	box := memory.NewOutbox()
	pending, err := domain.NewReservation(reservation, member, norte, fixed.Add(3*time.Hour), fixed.Add(4*time.Hour), fixed)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(context.Background(), pending); err != nil {
		t.Fatal(err)
	}
	return app.Bookings{
		Reservations: store,
		Spaces:       memory.Corte2Spaces(),
		Idempotency:  memory.NewIdempotency(),
		Outbox:       box,
		Inbox:        memory.NewInbox(),
		Now:          func() time.Time { return fixed },
		NewID:        func() string { return "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa" },
	}, store, box
}

func confirmedEvent() app.PaymentOutcome {
	return app.PaymentOutcome{
		EventID:       eventOK,
		EventType:     domain.EventPaymentConfirmed,
		ReservationID: reservation,
		PaymentID:     paymentOK,
		CorrelationID: "trace-pay-1",
	}
}

func TestPaymentConfirmedMovesPendingToConfirmed(t *testing.T) {
	svc, _, box := consumeSvc(t)
	got, applied, err := svc.ApplyPayment(context.Background(), confirmedEvent())
	if err != nil {
		t.Fatal(err)
	}
	if !applied || got.State != domain.StateConfirmed {
		t.Fatalf("applied=%v state=%s", applied, got.State)
	}
	unpublished, err := box.Unpublished(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(unpublished) != 1 || unpublished[0].EventType != domain.EventReservationConfirmed {
		t.Fatalf("outbox %#v", unpublished)
	}
	if unpublished[0].Topic != domain.TopicReservationConfirmed {
		t.Fatalf("topic %s", unpublished[0].Topic)
	}
}

func TestPaymentConfirmedIsIdempotentOnEventID(t *testing.T) {
	svc, _, box := consumeSvc(t)
	if _, _, err := svc.ApplyPayment(context.Background(), confirmedEvent()); err != nil {
		t.Fatal(err)
	}
	got, applied, err := svc.ApplyPayment(context.Background(), confirmedEvent())
	if err != nil {
		t.Fatal(err)
	}
	if applied || got.State != domain.StateConfirmed {
		t.Fatalf("replay applied=%v state=%s", applied, got.State)
	}
	unpublished, _ := box.Unpublished(context.Background())
	if len(unpublished) != 1 {
		t.Fatalf("outbox grew to %d", len(unpublished))
	}
}

func TestAlreadyConfirmedIsNoOp(t *testing.T) {
	svc, store, box := consumeSvc(t)
	current, _ := store.FindByID(context.Background(), reservation)
	confirmed, err := current.Confirm(time.Date(2026, 10, 24, 17, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(context.Background(), confirmed); err != nil {
		t.Fatal(err)
	}
	got, applied, err := svc.ApplyPayment(context.Background(), confirmedEvent())
	if err != nil {
		t.Fatal(err)
	}
	if applied || got.State != domain.StateConfirmed {
		t.Fatalf("applied=%v state=%s", applied, got.State)
	}
	unpublished, _ := box.Unpublished(context.Background())
	if len(unpublished) != 0 {
		t.Fatalf("must not publish again: %d", len(unpublished))
	}
}

func TestCancelledIsNotRevived(t *testing.T) {
	svc, store, box := consumeSvc(t)
	current, _ := store.FindByID(context.Background(), reservation)
	cancelled, err := current.Cancel(time.Date(2026, 10, 24, 17, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(context.Background(), cancelled); err != nil {
		t.Fatal(err)
	}
	got, applied, err := svc.ApplyPayment(context.Background(), confirmedEvent())
	if err != nil {
		t.Fatal(err)
	}
	if applied || got.State != domain.StateCancelled {
		t.Fatalf("applied=%v state=%s", applied, got.State)
	}
	unpublished, _ := box.Unpublished(context.Background())
	if len(unpublished) != 0 {
		t.Fatal("must not confirm a cancelled reservation")
	}
}

func TestPaymentFailedLeavesPending(t *testing.T) {
	svc, _, box := consumeSvc(t)
	got, applied, err := svc.ApplyPayment(context.Background(), app.PaymentOutcome{
		EventID:       eventOK,
		EventType:     domain.EventPaymentFailed,
		ReservationID: reservation,
		PaymentID:     paymentOK,
	})
	if err != nil {
		t.Fatal(err)
	}
	if applied || got.State != domain.StatePaymentPending {
		t.Fatalf("applied=%v state=%s", applied, got.State)
	}
	unpublished, _ := box.Unpublished(context.Background())
	if len(unpublished) != 0 {
		t.Fatalf("failed must not write confirm: %d", len(unpublished))
	}
}

func TestMissingReservationIsNotFound(t *testing.T) {
	svc, _, _ := consumeSvc(t)
	ev := confirmedEvent()
	ev.ReservationID = "99999999-9999-9999-9999-999999999999"
	_, _, err := svc.ApplyPayment(context.Background(), ev)
	if err != app.ErrNotFound {
		t.Fatalf("err %v", err)
	}
}

type capturePublisher struct {
	types []string
	fail  bool
}

func (c *capturePublisher) Publish(_ context.Context, e domain.OutboxEvent) error {
	if c.fail {
		return errPublish
	}
	c.types = append(c.types, e.EventType)
	return nil
}

var errPublish = errString("notification down")

type errString string

func (e errString) Error() string { return string(e) }

func TestConfirmDrainMarksPublished(t *testing.T) {
	svc, _, box := consumeSvc(t)
	pub := &capturePublisher{}
	svc.Publisher = pub
	if _, _, err := svc.ApplyPayment(context.Background(), confirmedEvent()); err != nil {
		t.Fatal(err)
	}
	unpublished, _ := box.Unpublished(context.Background())
	if len(unpublished) != 0 {
		t.Fatalf("expected drain, still %d", len(unpublished))
	}
	if len(pub.types) != 1 || pub.types[0] != domain.EventReservationConfirmed {
		t.Fatalf("published %#v", pub.types)
	}
}

func TestConfirmDrainKeepsRowWhenSinkFails(t *testing.T) {
	svc, _, box := consumeSvc(t)
	svc.Publisher = &capturePublisher{fail: true}
	if _, applied, err := svc.ApplyPayment(context.Background(), confirmedEvent()); err != nil || !applied {
		t.Fatalf("reservation must stay confirmed: applied=%v err=%v", applied, err)
	}
	unpublished, _ := box.Unpublished(context.Background())
	if len(unpublished) != 1 {
		t.Fatalf("failed sink must leave the row: %d", len(unpublished))
	}
}
