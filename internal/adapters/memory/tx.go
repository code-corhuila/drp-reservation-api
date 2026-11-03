package memory

import "context"

type rollbackKey struct{}

type hooks struct {
	fns []func()
}

// Tx rolls back in-memory Save/Append if the use-case returns an error.
type Tx struct{}

func (Tx) Within(ctx context.Context, fn func(context.Context) error) error {
	h := &hooks{}
	err := fn(context.WithValue(ctx, rollbackKey{}, h))
	if err != nil {
		for i := len(h.fns) - 1; i >= 0; i-- {
			h.fns[i]()
		}
	}
	return err
}

func registerRollback(ctx context.Context, fn func()) {
	h, ok := ctx.Value(rollbackKey{}).(*hooks)
	if !ok || h == nil {
		return
	}
	h.fns = append(h.fns, fn)
}
