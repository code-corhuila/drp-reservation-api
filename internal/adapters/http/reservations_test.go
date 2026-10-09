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
			Now:          func() time.Time { return fixed },
		},
		Tokens: ver,
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
