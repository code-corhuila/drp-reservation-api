package httpadapter

import (
	"net/http"

	"github.com/code-corhuila/drp-reservation-api/internal/app"
)

type Deps struct {
	Service  string
	Bookings app.Bookings
	Tokens   app.TokenVerifier
}

func NewMux(d Deps) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /health", HealthHandler(d.Service))
	if d.Tokens != nil {
		auth := RequireBearer(d.Tokens)
		mux.Handle("GET /api/v1/reservations", auth(ListReservations(d.Bookings)))
		mux.Handle("POST /api/v1/reservations", auth(CreateReservation(d.Bookings)))
		mux.Handle("GET /api/v1/reservations/{reservationId}", auth(GetReservation(d.Bookings)))
		mux.Handle("POST /api/v1/reservations/{reservationId}/cancel", auth(CancelReservation(d.Bookings)))
	}
	mux.Handle("/", NotFoundHandler())
	return WithCorrelation(stripSpoofedIdentity(mux))
}
