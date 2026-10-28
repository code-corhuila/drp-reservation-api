package httpadapter

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/code-corhuila/drp-reservation-api/internal/adapters/memory"
	"github.com/code-corhuila/drp-reservation-api/internal/adapters/security"
	"github.com/code-corhuila/drp-reservation-api/internal/app"
	"github.com/golang-jwt/jwt/v5"
)

const testKID = "spacehub-identity-2026"
const norte = "11111111-1111-1111-1111-111111111111"
const member = "44444444-4444-4444-4444-444444444444"

type testIssuer struct {
	priv *rsa.PrivateKey
}

func reservationMux(t *testing.T) (http.Handler, testIssuer) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwks := security.PublicJWKS(&priv.PublicKey, testKID)
	ver := security.NewVerifier(security.StaticJWKS(jwks))
	fixed := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	mux := NewMux(Deps{
		Service: "reservation-service",
		Bookings: app.Bookings{
			Reservations: memory.NewReservations(),
			Spaces:       memory.Corte2Spaces(),
			Idempotency:  memory.NewIdempotency(),
			Outbox:       memory.NewOutbox(),
			Inbox:        memory.NewInbox(),
			Now:          func() time.Time { return fixed },
		},
		Tokens:        ver,
		InternalToken: "spacehub-dev-internal",
	})
	return mux, testIssuer{priv: priv}
}

func (i testIssuer) bearer(t *testing.T, sub, role string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":  sub,
		"role": role,
		"iat":  time.Now().UTC().Unix(),
		"exp":  time.Now().UTC().Add(time.Hour).Unix(),
	})
	tok.Header["kid"] = testKID
	signed, err := tok.SignedString(i.priv)
	if err != nil {
		t.Fatal(err)
	}
	return "Bearer " + signed
}

func TestListRequiresToken(t *testing.T) {
	mux, _ := reservationMux(t)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	res, err := http.Get(srv.URL + "/api/v1/reservations")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestCreateAndListEnvelope(t *testing.T) {
	mux, keys := reservationMux(t)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	body := []byte(`{"spaceId":"` + norte + `","startAt":"2026-10-21T14:00:00Z","endAt":"2026-10-21T16:00:00Z"}`)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/reservations", bytes.NewReader(body))
	req.Header.Set("Authorization", keys.bearer(t, member, "USER"))
	req.Header.Set("Idempotency-Key", "key-create-1")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create %d", res.StatusCode)
	}
	var created reservationDTO
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.State != "PAYMENT_PENDING" || created.UserID != member {
		t.Fatalf("created %+v", created)
	}

	list, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/reservations", nil)
	list.Header.Set("Authorization", keys.bearer(t, member, "USER"))
	got, err := http.DefaultClient.Do(list)
	if err != nil {
		t.Fatal(err)
	}
	defer got.Body.Close()
	if got.StatusCode != http.StatusOK {
		t.Fatalf("list %d", got.StatusCode)
	}
	var env listEnvelope
	if err := json.NewDecoder(got.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if env.Data == nil || env.Meta.Total != 1 || env.Data[0].ID != created.ID {
		t.Fatalf("envelope %+v", env)
	}
}

func TestMalformedUUID(t *testing.T) {
	mux, keys := reservationMux(t)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/reservations/not-a-uuid", nil)
	req.Header.Set("Authorization", keys.bearer(t, member, "USER"))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestMissingReservation(t *testing.T) {
	mux, keys := reservationMux(t)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/reservations/99999999-9999-9999-9999-999999999999", nil)
	req.Header.Set("Authorization", keys.bearer(t, member, "USER"))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestCreateWithoutIdempotencyKey(t *testing.T) {
	mux, keys := reservationMux(t)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	body := []byte(`{"spaceId":"` + norte + `","startAt":"2026-10-21T14:00:00Z","endAt":"2026-10-21T16:00:00Z"}`)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/reservations", bytes.NewReader(body))
	req.Header.Set("Authorization", keys.bearer(t, member, "USER"))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestHealth(t *testing.T) {
	mux, _ := reservationMux(t)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	res, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestCreateInPastReturns422Envelope(t *testing.T) {
	mux, keys := reservationMux(t)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	body := []byte(`{"spaceId":"` + norte + `","startAt":"2026-10-19T14:00:00Z","endAt":"2026-10-19T16:00:00Z"}`)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/reservations", bytes.NewReader(body))
	req.Header.Set("Authorization", keys.bearer(t, member, "USER"))
	req.Header.Set("Idempotency-Key", "key-past-422")
	req.Header.Set("X-Correlation-Id", "33333333-3333-3333-3333-333333333333")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	assertErrorEnvelope(t, res, http.StatusUnprocessableEntity, "BUSINESS_RULE_VIOLATION", "33333333-3333-3333-3333-333333333333")
}

func TestCancelTooSoonReturns422Envelope(t *testing.T) {
	mux, keys := reservationMux(t)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	body := []byte(`{"spaceId":"` + norte + `","startAt":"2026-10-20T13:00:00Z","endAt":"2026-10-20T14:00:00Z"}`)
	create, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/reservations", bytes.NewReader(body))
	create.Header.Set("Authorization", keys.bearer(t, member, "USER"))
	create.Header.Set("Idempotency-Key", "key-cancel-422")
	created, err := http.DefaultClient.Do(create)
	if err != nil {
		t.Fatal(err)
	}
	defer created.Body.Close()
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("create %d", created.StatusCode)
	}
	var dto reservationDTO
	if err := json.NewDecoder(created.Body).Decode(&dto); err != nil {
		t.Fatal(err)
	}

	cancel, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/reservations/"+dto.ID+"/cancel", nil)
	cancel.Header.Set("Authorization", keys.bearer(t, member, "USER"))
	cancel.Header.Set("X-Correlation-Id", "44444444-4444-4444-4444-444444444444")
	res, err := http.DefaultClient.Do(cancel)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	assertErrorEnvelope(t, res, http.StatusUnprocessableEntity, "INVALID_STATUS_TRANSITION", "44444444-4444-4444-4444-444444444444")
}

func TestInternalEventConfirmsReservation(t *testing.T) {
	mux, keys := reservationMux(t)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	body := []byte(`{"spaceId":"` + norte + `","startAt":"2026-10-21T14:00:00Z","endAt":"2026-10-21T16:00:00Z"}`)
	create, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/reservations", bytes.NewReader(body))
	create.Header.Set("Authorization", keys.bearer(t, member, "USER"))
	create.Header.Set("Idempotency-Key", "key-pay-confirm")
	created, err := http.DefaultClient.Do(create)
	if err != nil {
		t.Fatal(err)
	}
	defer created.Body.Close()
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("create %d", created.StatusCode)
	}
	var dto reservationDTO
	if err := json.NewDecoder(created.Body).Decode(&dto); err != nil {
		t.Fatal(err)
	}

	event := []byte(`{"eventId":"77777777-7777-4777-8777-777777777777","eventType":"PaymentConfirmed","payload":{"paymentId":"88888888-8888-4888-8888-888888888888","reservationId":"` + dto.ID + `","amountCents":5000000,"currency":"COP"},"correlationId":"7a3c91e2-1111-4111-8111-000000000001"}`)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/internal/events", bytes.NewReader(event))
	req.Header.Set("X-Internal-Token", "spacehub-dev-internal")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusAccepted {
		t.Fatalf("consume %d", res.StatusCode)
	}

	get, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/reservations/"+dto.ID, nil)
	get.Header.Set("Authorization", keys.bearer(t, member, "USER"))
	got, err := http.DefaultClient.Do(get)
	if err != nil {
		t.Fatal(err)
	}
	defer got.Body.Close()
	var after reservationDTO
	if err := json.NewDecoder(got.Body).Decode(&after); err != nil {
		t.Fatal(err)
	}
	if after.State != "CONFIRMED" {
		t.Fatalf("state %s", after.State)
	}

	confirm, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/reservations/"+dto.ID+"/confirm", nil)
	confirm.Header.Set("Authorization", keys.bearer(t, member, "USER"))
	denied, err := http.DefaultClient.Do(confirm)
	if err != nil {
		t.Fatal(err)
	}
	defer denied.Body.Close()
	if denied.StatusCode != http.StatusNotFound {
		t.Fatalf("confirm route %d", denied.StatusCode)
	}
}

func TestInternalEventWithoutTokenIs401(t *testing.T) {
	mux, _ := reservationMux(t)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/internal/events", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("X-User-Id", member)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func assertErrorEnvelope(t *testing.T, res *http.Response, status int, code, traceID string) {
	t.Helper()
	if res.StatusCode != status {
		t.Fatalf("status %d", res.StatusCode)
	}
	var raw map[string]any
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		t.Fatal(err)
	}
	if raw["error"] != code || raw["message"] == "" || raw["traceId"] != traceID {
		t.Fatalf("envelope %+v", raw)
	}
}
