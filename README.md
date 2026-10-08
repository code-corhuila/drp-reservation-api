# drp-reservation-api

Reservation API (Go). **Hexagonal:** `internal/domain` has no web/DB imports. DDL in [`drp-reservation-db`](https://github.com/code-corhuila/drp-reservation-db).

This increment is the **Reservation aggregate**: create starts in `PAYMENT_PENDING`; `Confirm` / `Cancel` follow BR-005; `ConfirmedOverlap` is BR-001 (`PAYMENT_PENDING` does not block). HTTP adapters come later.

```bash
go test ./...
```

Child of `develop` named `feat/…`. Promote with `cherry-pick -x`.
