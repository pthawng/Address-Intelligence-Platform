package bootstrap

import (
	"address-intelligence-platform/internal/platform/config"
	"context"
)

func NewWorker(options ...Option) (*App, error) {
	a, err := newApp(config.Worker, options...)
	if err != nil {
		return nil, err
	}
	a.run = func(ctx context.Context, _ *resources) error {
		a.logger.Info("worker runtime initialized")
		<-ctx.Done()
		return nil
	}
	return a, nil
}

func RunWorker(ctx context.Context) error {
	a, err := NewWorker()
	if err != nil {
		return err
	}
	return a.Run(ctx)
}
