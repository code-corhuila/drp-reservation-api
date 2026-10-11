package app

import (
	"testing"
	"time"
)

func TestCheckHealth(t *testing.T) {
	now := time.Date(2026, 10, 8, 19, 0, 0, 0, time.UTC)
	got := CheckHealth("reservation-service", now)
	if got.Status != "ok" || got.Service != "reservation-service" || !got.Timestamp.Equal(now) {
		t.Fatalf("got %+v", got)
	}
}
