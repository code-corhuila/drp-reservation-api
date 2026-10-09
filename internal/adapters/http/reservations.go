package httpadapter

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/code-corhuila/drp-reservation-api/internal/app"
	"github.com/code-corhuila/drp-reservation-api/internal/domain"
)

var canonicalUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type reservationDTO struct {
	ID        string `json:"id"`
	UserID    string `json:"userId"`
	SpaceID   string `json:"spaceId"`
	StartAt   string `json:"startAt"`
	EndAt     string `json:"endAt"`
	State     string `json:"state"`
	CreatedAt string `json:"createdAt,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

type listEnvelope struct {
	Data []reservationDTO `json:"data"`
	Meta metaDTO          `json:"meta"`
}

type metaDTO struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"totalPages"`
}

func toDTO(r domain.Reservation) reservationDTO {
	return reservationDTO{
		ID:        r.ID,
		UserID:    r.UserID,
		SpaceID:   r.SpaceID,
		StartAt:   r.Period.Start.UTC().Format("2006-01-02T15:04:05Z"),
		EndAt:     r.Period.End.UTC().Format("2006-01-02T15:04:05Z"),
		State:     string(r.State),
		CreatedAt: r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt: r.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func ListReservations(b app.Bookings) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, limit, berr := parsePageLimit(r)
		if berr != nil {
			writeErr(w, r, http.StatusBadRequest, "VALIDATION_ERROR", berr.Error(), berr.details)
			return
		}
		filter := app.ListFilter{Actor: principalFrom(r), Page: page, Limit: limit}
		if raw := strings.TrimSpace(r.URL.Query().Get("state")); raw != "" {
			st := domain.State(raw)
			if st != domain.StatePaymentPending && st != domain.StateConfirmed && st != domain.StateCancelled {
				writeErr(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "El estado no es válido", []map[string]string{{"field": "state", "message": "Debe ser un estado de reserva conocido"}})
				return
			}
			filter.State = &st
		}
		if raw := strings.TrimSpace(r.URL.Query().Get("spaceId")); raw != "" {
			if !canonicalUUID.MatchString(raw) {
				writeErr(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "spaceId no es un UUID canónico", []map[string]string{{"field": "spaceId", "message": "Debe ser un UUID en minúsculas con guiones"}})
				return
			}
			filter.SpaceID = raw
		}
		got, err := b.List(r.Context(), filter)
		if errors.Is(err, app.ErrValidation) {
			writeErr(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Parámetros de paginación inválidos", nil)
			return
		}
		if err != nil {
			writeErr(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Error interno", nil)
			return
		}
		data := make([]reservationDTO, 0, len(got.Items))
		for _, item := range got.Items {
			data = append(data, toDTO(item))
		}
		writeJSON(w, http.StatusOK, listEnvelope{
			Data: data,
			Meta: metaDTO{Page: got.Page, Limit: got.Limit, Total: got.Total, TotalPages: got.TotalPages},
		})
	}
}

func GetReservation(b app.Bookings) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("reservationId")
		if !canonicalUUID.MatchString(id) {
			writeErr(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "El identificador no es un UUID canónico", []map[string]string{{"field": "reservationId", "message": "Debe ser un UUID en minúsculas con guiones"}})
			return
		}
		got, err := b.Get(r.Context(), principalFrom(r), id)
		if errors.Is(err, app.ErrNotFound) {
			writeErr(w, r, http.StatusNotFound, "NOT_FOUND", "La reserva no existe", nil)
			return
		}
		if err != nil {
			writeErr(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Error interno", nil)
			return
		}
		writeJSON(w, http.StatusOK, toDTO(got))
	}
}

type createBody struct {
	SpaceID string `json:"spaceId"`
	StartAt string `json:"startAt"`
	EndAt   string `json:"endAt"`
	UserID  string `json:"userId"`
}

func CreateReservation(b app.Bookings) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if key == "" {
			writeErr(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Idempotency-Key es obligatoria", []map[string]string{{"field": "Idempotency-Key", "message": "Obligatoria, 8 a 128 caracteres"}})
			return
		}
		if len(key) < 8 || len(key) > 128 {
			writeErr(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Idempotency-Key inválida", []map[string]string{{"field": "Idempotency-Key", "message": "Debe tener entre 8 y 128 caracteres"}})
			return
		}
		raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeErr(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Datos de entrada inválidos", nil)
			return
		}
		var body createBody
		if err := json.Unmarshal(raw, &body); err != nil {
			writeErr(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "JSON malformado", nil)
			return
		}
		if !canonicalUUID.MatchString(body.SpaceID) {
			writeErr(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "spaceId no es un UUID canónico", []map[string]string{{"field": "spaceId", "message": "Debe ser un UUID en minúsculas con guiones"}})
			return
		}
		start, berr := parseInstant("startAt", body.StartAt)
		if berr != nil {
			writeErr(w, r, http.StatusBadRequest, "VALIDATION_ERROR", berr.Error(), berr.details)
			return
		}
		end, berr := parseInstant("endAt", body.EndAt)
		if berr != nil {
			writeErr(w, r, http.StatusBadRequest, "VALIDATION_ERROR", berr.Error(), berr.details)
			return
		}
		if !end.After(start) {
			writeErr(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "endAt debe ser posterior a startAt", []map[string]string{{"field": "endAt", "message": "Debe ser posterior a startAt"}})
			return
		}
		if !aligned30(start) || !aligned30(end) {
			writeErr(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "La granularidad es de 30 minutos", []map[string]string{{"field": "startAt", "message": "Debe caer en un múltiplo de 30 minutos"}})
			return
		}
		got, replay, cerr := b.Create(r.Context(), app.CreateCommand{
			Actor:          principalFrom(r),
			SpaceID:        body.SpaceID,
			StartAt:        start,
			EndAt:          end,
			IdempotencyKey: key,
			RawBody:        string(raw),
		})
		if errors.Is(cerr, app.ErrConflict) {
			writeErr(w, r, http.StatusConflict, "IDEMPOTENCY_CONFLICT", "La misma Idempotency-Key se usó con otro cuerpo", nil)
			return
		}
		if errors.Is(cerr, app.ErrNotFound) {
			writeErr(w, r, http.StatusNotFound, "NOT_FOUND", "El espacio solicitado no existe", nil)
			return
		}
		if errors.Is(cerr, app.ErrInactiveSpace) {
			writeErr(w, r, http.StatusUnprocessableEntity, "BUSINESS_RULE_VIOLATION", "El espacio no está activo.", nil)
			return
		}
		if errors.Is(cerr, app.ErrInPast) {
			writeErr(w, r, http.StatusUnprocessableEntity, "BUSINESS_RULE_VIOLATION", "No se puede reservar en el pasado.", nil)
			return
		}
		if errors.Is(cerr, app.ErrTooLong) {
			writeErr(w, r, http.StatusUnprocessableEntity, "BUSINESS_RULE_VIOLATION", "La reserva no puede durar más de 12 horas.", nil)
			return
		}
		if errors.Is(cerr, app.ErrOverlap) {
			writeErr(w, r, http.StatusUnprocessableEntity, "BUSINESS_RULE_VIOLATION", "Ya existe una reserva CONFIRMED que se solapa en este espacio.", nil)
			return
		}
		if errors.Is(cerr, app.ErrValidation) {
			writeErr(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Datos de entrada inválidos", nil)
			return
		}
		if cerr != nil {
			writeErr(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Error interno", nil)
			return
		}
		status := http.StatusCreated
		if replay {
			status = http.StatusOK
		}
		writeJSON(w, status, toDTO(got))
	}
}

func CancelReservation(b app.Bookings) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("reservationId")
		if !canonicalUUID.MatchString(id) {
			writeErr(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "El identificador no es un UUID canónico", []map[string]string{{"field": "reservationId", "message": "Debe ser un UUID en minúsculas con guiones"}})
			return
		}
		got, err := b.Cancel(r.Context(), principalFrom(r), id)
		if errors.Is(err, app.ErrNotFound) {
			writeErr(w, r, http.StatusNotFound, "NOT_FOUND", "La reserva no existe", nil)
			return
		}
		if errors.Is(err, app.ErrTransition) {
			writeErr(w, r, http.StatusUnprocessableEntity, "INVALID_STATUS_TRANSITION", "La reserva no puede cancelarse en el estado actual", nil)
			return
		}
		if err != nil {
			writeErr(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Error interno", nil)
			return
		}
		writeJSON(w, http.StatusOK, toDTO(got))
	}
}

type bindError struct {
	msg     string
	details []map[string]string
}

func (e *bindError) Error() string { return e.msg }

func parseInstant(field, raw string) (time.Time, *bindError) {
	if !strings.HasSuffix(raw, "Z") {
		return time.Time{}, &bindError{msg: field + " debe ser RFC 3339 UTC con Z", details: []map[string]string{{"field": field, "message": "Debe terminar en Z"}}}
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, &bindError{msg: field + " no es RFC 3339", details: []map[string]string{{"field": field, "message": "Debe ser RFC 3339 en UTC"}}}
	}
	return t.UTC(), nil
}

func aligned30(t time.Time) bool {
	return t.Second() == 0 && t.Nanosecond() == 0 && t.Minute()%30 == 0
}

func parsePageLimit(r *http.Request) (int, int, *bindError) {
	page := 1
	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("page")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return 0, 0, &bindError{msg: "El parámetro page debe ser un entero", details: []map[string]string{{"field": "page", "message": "Debe ser un entero"}}}
		}
		if n < 1 {
			return 0, 0, &bindError{msg: "El parámetro page debe ser mayor o igual a 1", details: []map[string]string{{"field": "page", "message": "Debe ser mayor o igual a 1"}}}
		}
		page = n
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return 0, 0, &bindError{msg: "El parámetro limit debe ser un entero", details: []map[string]string{{"field": "limit", "message": "Debe ser un entero"}}}
		}
		if n < 1 || n > 100 {
			return 0, 0, &bindError{msg: "El parámetro limit debe estar entre 1 y 100", details: []map[string]string{{"field": "limit", "message": "Debe estar entre 1 y 100"}}}
		}
		limit = n
	}
	return page, limit, nil
}
