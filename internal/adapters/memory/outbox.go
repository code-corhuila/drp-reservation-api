package memory

import (
	"context"
	"sync"

	"github.com/code-corhuila/drp-reservation-api/internal/domain"
)

type Outbox struct {
	mu     sync.Mutex
	events []domain.OutboxEvent
}

func NewOutbox() *Outbox {
	return &Outbox{}
}

func (o *Outbox) Append(_ context.Context, e domain.OutboxEvent) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.events = append(o.events, e)
	return nil
}

func (o *Outbox) Unpublished(_ context.Context) ([]domain.OutboxEvent, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]domain.OutboxEvent, 0, len(o.events))
	for _, e := range o.events {
		if !e.Published {
			out = append(out, e)
		}
	}
	return out, nil
}

func (o *Outbox) MarkPublished(_ context.Context, eventID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	for i, e := range o.events {
		if e.EventID == eventID {
			e.Published = true
			o.events[i] = e
		}
	}
	return nil
}
