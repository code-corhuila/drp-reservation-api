package main

import (
	"log"
	"net/http"
	"os"
	"time"

	httpadapter "github.com/code-corhuila/drp-reservation-api/internal/adapters/http"
	"github.com/code-corhuila/drp-reservation-api/internal/adapters/memory"
	"github.com/code-corhuila/drp-reservation-api/internal/adapters/security"
	"github.com/code-corhuila/drp-reservation-api/internal/app"
)

func main() {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8083"
	}
	service := os.Getenv("SERVICE_NAME")
	if service == "" {
		service = "reservation-service"
	}
	jwksURL := os.Getenv("IDENTITY_JWKS_URL")
	if jwksURL == "" {
		jwksURL = "http://identity-api:8081/api/v1/auth/jwks"
	}

	tokens := security.NewVerifier(security.HTTPJWKS{
		URL: jwksURL,
		Client: &http.Client{
			Timeout: 3 * time.Second,
		},
	})
	mux := httpadapter.NewMux(httpadapter.Deps{
		Service: service,
		Bookings: app.Bookings{
			Reservations: memory.NewReservations(),
			Spaces:       memory.Corte2Spaces(),
			Idempotency:  memory.NewIdempotency(),
			Outbox:       memory.NewOutbox(),
		},
		Tokens: tokens,
	})

	log.Printf("drp-reservation-api listening on %s (memory bookings; JWKS %s; no DDL in this repo)", addr, jwksURL)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
