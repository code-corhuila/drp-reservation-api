package publish

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/code-corhuila/drp-reservation-api/internal/domain"
)

type HTTP struct {
	BaseURL string
	Token   string
	Client  *http.Client
}

func (h HTTP) Publish(ctx context.Context, e domain.OutboxEvent) error {
	if h.BaseURL == "" || h.Token == "" {
		return nil
	}
	var payload any
	if err := json.Unmarshal([]byte(e.PayloadJSON), &payload); err != nil {
		payload = map[string]string{"raw": e.PayloadJSON}
	}
	body, err := json.Marshal(map[string]any{
		"eventId":   e.EventID,
		"eventType": e.EventType,
		"payload":   payload,
	})
	if err != nil {
		return err
	}
	base := strings.TrimRight(h.BaseURL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/internal/events", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Token", h.Token)
	client := h.Client
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("notification consume HTTP %d", res.StatusCode)
	}
	return nil
}
