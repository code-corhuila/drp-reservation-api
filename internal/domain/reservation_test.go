package domain

import (
	"testing"
	"time"
)

func TestReservationLifecycle(t *testing.T) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	start := now.Add(24 * time.Hour)
	end := start.Add(2 * time.Hour)
	r, err := NewReservation("r1", "u1", "s1", start, end, now)
	if err != nil {
		t.Fatal(err)
	}
	if r.State != StatePaymentPending {
		t.Fatalf("initial %s", r.State)
	}
	c, err := r.Confirm(now.Add(time.Minute))
	if err != nil || c.State != StateConfirmed {
		t.Fatalf("confirm %v %s", err, c.State)
	}
	if _, err := c.Confirm(now); err == nil {
		t.Fatal("double confirm")
	}
	x, err := c.Cancel(now.Add(2 * time.Minute))
	if err != nil || x.State != StateCancelled {
		t.Fatal(err)
	}
}

func TestConfirmedOverlap(t *testing.T) {
	now := time.Now().UTC()
	start := now.Add(time.Hour)
	end := start.Add(2 * time.Hour)
	a, _ := NewReservation("a", "u", "s", start, end, now)
	b, _ := NewReservation("b", "u", "s", start.Add(time.Hour), end.Add(time.Hour), now)
	a, _ = a.Confirm(now)
	if ConfirmedOverlap(a, b) {
		t.Fatal("PAYMENT_PENDING must not block")
	}
	b, _ = b.Confirm(now)
	if !ConfirmedOverlap(a, b) {
		t.Fatal("two CONFIRMED overlapping must collide")
	}
}
