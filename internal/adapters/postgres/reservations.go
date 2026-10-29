package postgres

import (
	"context"
	"database/sql"

	"github.com/code-corhuila/drp-reservation-api/internal/app"
	"github.com/code-corhuila/drp-reservation-api/internal/domain"
)

type Reservations struct {
	DB *sql.DB
}

func NewReservations(db *sql.DB) *Reservations {
	return &Reservations{DB: db}
}

func (r *Reservations) List(ctx context.Context) ([]domain.Reservation, error) {
	rows, err := r.DB.QueryContext(ctx, `
SELECT id, user_id, space_id, start_at, end_at, state, created_at, updated_at
FROM reservations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Reservation{}
	for rows.Next() {
		got, err := scanReservation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, got)
	}
	return out, rows.Err()
}

func (r *Reservations) FindByID(ctx context.Context, id string) (domain.Reservation, error) {
	row := r.DB.QueryRowContext(ctx, `
SELECT id, user_id, space_id, start_at, end_at, state, created_at, updated_at
FROM reservations WHERE id = $1`, id)
	got, err := scanReservation(row)
	if err == sql.ErrNoRows {
		return domain.Reservation{}, app.ErrNotFound
	}
	return got, err
}

func (r *Reservations) Save(ctx context.Context, reservation domain.Reservation) error {
	_, err := r.DB.ExecContext(ctx, `
INSERT INTO reservations (id, user_id, space_id, start_at, end_at, state, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
ON CONFLICT (id) DO UPDATE SET
  state = EXCLUDED.state,
  updated_at = EXCLUDED.updated_at`,
		reservation.ID, reservation.UserID, reservation.SpaceID,
		reservation.Period.Start.UTC(), reservation.Period.End.UTC(),
		string(reservation.State), reservation.CreatedAt.UTC(), reservation.UpdatedAt.UTC())
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanReservation(sc scanner) (domain.Reservation, error) {
	var got domain.Reservation
	var state string
	if err := sc.Scan(&got.ID, &got.UserID, &got.SpaceID, &got.Period.Start, &got.Period.End, &state, &got.CreatedAt, &got.UpdatedAt); err != nil {
		return domain.Reservation{}, err
	}
	got.State = domain.State(state)
	got.Period.Start = got.Period.Start.UTC()
	got.Period.End = got.Period.End.UTC()
	got.CreatedAt = got.CreatedAt.UTC()
	got.UpdatedAt = got.UpdatedAt.UTC()
	return got, nil
}
