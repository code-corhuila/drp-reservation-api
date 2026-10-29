package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/code-corhuila/drp-reservation-api/internal/adapters/catalog"
	httpadapter "github.com/code-corhuila/drp-reservation-api/internal/adapters/http"
	"github.com/code-corhuila/drp-reservation-api/internal/adapters/memory"
	"github.com/code-corhuila/drp-reservation-api/internal/adapters/postgres"
	"github.com/code-corhuila/drp-reservation-api/internal/adapters/publish"
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
	internalToken := os.Getenv("INTERNAL_EVENT_TOKEN")
	var publisher app.EventPublisher
	if notifyURL := os.Getenv("NOTIFICATION_API_URL"); notifyURL != "" && internalToken != "" {
		publisher = publish.HTTP{
			BaseURL: notifyURL,
			Token:   internalToken,
			Client:  &http.Client{Timeout: 3 * time.Second},
		}
	}
	var bookings app.ReservationRepository = memory.NewReservations()
	store := "memory bookings"
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		db, err := postgres.Open(dsn)
		if err != nil {
			log.Fatal(err)
		}
		db.SetMaxOpenConns(8)
		bookings = postgres.NewReservations(db)
		store = "postgres reservation schema"
	}
	var spaces app.SpaceCatalog = memory.Corte2Spaces()
	if spaceURL := os.Getenv("SPACE_API_URL"); spaceURL != "" {
		spaces = catalog.HTTP{BaseURL: spaceURL, Client: &http.Client{Timeout: 3 * time.Second}}
	}
	mux := httpadapter.NewMux(httpadapter.Deps{
		Service: service,
		Bookings: app.Bookings{
			Reservations: bookings,
			Spaces:       spaces,
			Idempotency:  memory.NewIdempotency(),
			Outbox:       memory.NewOutbox(),
			Inbox:        memory.NewInbox(),
			Publisher:    publisher,
		},
		Tokens:        tokens,
		InternalToken: internalToken,
	})

	log.Printf("drp-reservation-api listening on %s (%s; POST /internal/events consumes PaymentConfirmed; space lookup HTTP when SPACE_API_URL is set; JWKS %s; no DDL in this repo)", addr, store, jwksURL)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
