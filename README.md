# drp-reservation-api

Reservation API (Go). **Hexagonal:** `internal/domain` has no web/DB imports. DDL in [`drp-reservation-db`](https://github.com/code-corhuila/drp-reservation-db).

Walking HTTP for E-11…E-14 (`:8083`): lists `{data, meta}`, create with `Idempotency-Key` starts in `PAYMENT_PENDING`, overlap of `CONFIRMED` is **422**, foreign GET is **404** (D-C20). RS256 against identity JWKS. Catalog lookup is an in-memory Corte 2 seed (no HTTP to `drp-space-api` yet; blocks come later). Confirm is **not** an HTTP command (D-C14): there is no `POST /api/v1/reservations/{id}/confirm`. The walking consumer is `POST /internal/events` on `:8083` (not on the gateway), gated by `X-Internal-Token`. It maps `PaymentConfirmed` / `PaymentFailed` onto `ApplyPayment` (PAYMENT_PENDING → CONFIRMED + `ReservationConfirmed` outbox; already CONFIRMED is no-op; CANCELLED is not revived). Duplicate `eventId` is absorbed by an in-memory inbox. After a write, unpublished outbox rows are POSTed to `drp-notification-api` `POST /internal/events` when `NOTIFICATION_API_URL` is set (best-effort: a failed inbox must not undo the reservation). AMQP replaces this hop later.

```bash
go test ./...
```

Child of `develop` named `feat/…`. Promote with `cherry-pick -x`.
