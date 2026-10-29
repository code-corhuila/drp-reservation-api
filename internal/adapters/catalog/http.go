package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/code-corhuila/drp-reservation-api/internal/app"
)

type ctxKey struct{}

func WithBearer(ctx context.Context, authorization string) context.Context {
	return context.WithValue(ctx, ctxKey{}, strings.TrimSpace(authorization))
}

func bearer(ctx context.Context) string {
	v, _ := ctx.Value(ctxKey{}).(string)
	return v
}

// HTTP looks up space.active on drp-space-api. Never opens schema space.
type HTTP struct {
	BaseURL string
	Client  *http.Client
}

type spaceDTO struct {
	Active bool `json:"active"`
}

func (h HTTP) Lookup(ctx context.Context, spaceID string) (bool, error) {
	if h.BaseURL == "" {
		return false, app.ErrNotFound
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(h.BaseURL, "/")+"/api/v1/spaces/"+spaceID, nil)
	if err != nil {
		return false, err
	}
	if tok := bearer(ctx); tok != "" {
		req.Header.Set("Authorization", tok)
	}
	client := h.Client
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return false, app.ErrNotFound
	}
	if res.StatusCode >= 400 {
		return false, fmt.Errorf("space catalog HTTP %d", res.StatusCode)
	}
	var body spaceDTO
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return false, err
	}
	return body.Active, nil
}
