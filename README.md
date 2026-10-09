# drp-reservation-api

Reservation API (Go). **Hexagonal:** `internal/domain` has no web/DB imports. DDL in [`drp-reservation-db`](https://github.com/code-corhuila/drp-reservation-db).

Walking HTTP for E-11…E-14 (`:8083`): lists `{data, meta}`, create with `Idempotency-Key` starts in `PAYMENT_PENDING`, overlap of `CONFIRMED` is **422**, foreign GET is **404** (D-C20). RS256 against identity JWKS. Catalog lookup is an in-memory Corte 2 seed (no HTTP to `drp-space-api` yet; blocks come later). Confirm remains an event, not an HTTP command (D-C14). Create/cancel append `ReservationCreated` / `ReservationCancelled` to an **in-memory outbox** in the same use case (publisher + `reservation.outbox` Flyway come with Postgres).

```bash
go test ./...
```

Child of `develop` named `feat/…`. Promote with `cherry-pick -x`.
