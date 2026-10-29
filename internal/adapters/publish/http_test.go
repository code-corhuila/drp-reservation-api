package publish

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/code-corhuila/drp-reservation-api/internal/domain"
)

func TestPublishSendsCorrelationHeaderAndUserId(t *testing.T) {
	var gotHeader, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Correlation-Id")
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		if r.Header.Get("X-Internal-Token") != "tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	err := (HTTP{BaseURL: srv.URL, Token: "tok", Client: srv.Client()}).Publish(context.Background(), domain.OutboxEvent{
		EventID:       "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		EventType:     domain.EventReservationConfirmed,
		PayloadJSON:   `{"reservationId":"22222222-2222-2222-2222-222222222222","userId":"44444444-4444-4444-4444-444444444444"}`,
		CorrelationID: "trace-pay-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotHeader != "trace-pay-1" {
		t.Fatalf("header %s", gotHeader)
	}
	if !strings.Contains(gotBody, `"correlationId":"trace-pay-1"`) || !strings.Contains(gotBody, `"userId":"44444444-4444-4444-4444-444444444444"`) {
		t.Fatalf("body %s", gotBody)
	}
}
