package bootstrap

import (
	"context"
	"errors"
	"fmt"
)

// resources is owned by the Run goroutine. Register cleanup immediately after
// acquisition. Closers must honor the shared deadline and tolerate expired ctx.
type resources struct {
	entries []resource
}

type resource struct {
	name  string
	close func(context.Context) error
}

func (r *resources) add(name string, close func(context.Context) error) {
	r.entries = append(r.entries, resource{name, close})
}

func (r *resources) close(ctx context.Context) error {
	entries := r.entries
	r.entries = nil
	var result error
	for i := len(entries) - 1; i >= 0; i-- {
		if err := entries[i].close(ctx); err != nil {
			result = errors.Join(result, fmt.Errorf("close %s: %w", entries[i].name, err))
		}
	}
	return result
}
