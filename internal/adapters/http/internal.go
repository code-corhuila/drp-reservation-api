package httpadapter

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/code-corhuila/drp-reservation-api/internal/adapters/consume"
	"github.com/code-corhuila/drp-reservation-api/internal/app"
)

const internalTokenHeader = "X-Internal-Token"

type inboundEvent struct {
	EventID       string          `json:"eventId"`
	EventType     string          `json:"eventType"`
	Payload       json.RawMessage `json:"payload"`
	CorrelationID string          `json:"correlationId"`
}

func RequireInternalToken(expected string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if expected == "" {
				writeErr(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "El consumo interno no está configurado", nil)
				return
			}
			got := strings.TrimSpace(r.Header.Get(internalTokenHeader))
			if got == "" || got != expected {
				writeErr(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Token de autenticación requerido", nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func ConsumePayment(b app.Bookings) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeErr(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Datos de entrada inválidos", nil)
			return
		}
		var body inboundEvent
		if err := json.Unmarshal(raw, &body); err != nil {
			writeErr(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "JSON malformado", nil)
			return
		}
		ev, err := consume.FromPaymentOutbox(body.EventType, body.EventID, string(body.Payload), firstNonEmpty(body.CorrelationID, TraceID(r)))
		if err != nil {
			writeErr(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Evento de pago inválido", nil)
			return
		}
		got, applied, err := b.ApplyPayment(r.Context(), ev)
		if errors.Is(err, app.ErrNotFound) {
			writeErr(w, r, http.StatusNotFound, "NOT_FOUND", "La reserva no existe", nil)
			return
		}
		if errors.Is(err, app.ErrOverlap) {
			writeErr(w, r, http.StatusUnprocessableEntity, "BUSINESS_RULE_VIOLATION", "Ya existe una reserva CONFIRMED que se solapa en este espacio.", nil)
			return
		}
		if errors.Is(err, app.ErrTransition) {
			writeErr(w, r, http.StatusUnprocessableEntity, "INVALID_STATUS_TRANSITION", "La reserva no puede confirmarse en el estado actual", nil)
			return
		}
		if errors.Is(err, app.ErrValidation) {
			writeErr(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Evento de pago inválido", nil)
			return
		}
		if err != nil {
			writeErr(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Error interno", nil)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{
			"applied":       applied,
			"reservationId": got.ID,
			"state":         string(got.State),
		})
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
