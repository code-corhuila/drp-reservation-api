package memory

import (
	"context"
	"sync"
)

type Inbox struct {
	mu   sync.Mutex
	seen map[string]struct{}
}

func NewInbox() *Inbox {
	return &Inbox{seen: map[string]struct{}{}}
}

func (i *Inbox) Seen(_ context.Context, eventID string) bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	_, ok := i.seen[eventID]
	return ok
}

func (i *Inbox) Mark(_ context.Context, eventID string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.seen[eventID] = struct{}{}
}
