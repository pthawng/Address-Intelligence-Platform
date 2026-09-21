package bootstrap

import (
	"address-intelligence-platform/internal/platform/config"
	"context"
)

func NewIndexer(options ...Option) (*App, error) {
	a, err := newApp(config.Indexer, options...)
	if err != nil {
		return nil, err
	}
	a.run = func(ctx context.Context, _ *resources) error {
		a.logger.Info("indexer runtime initialized", "poll_interval", a.cfg.OutboxPollInterval)
		<-ctx.Done()
		return nil
	}
	return a, nil
}

func RunIndexer(ctx context.Context) error {
	a, err := NewIndexer()
	if err != nil {
		return err
	}
	return a.Run(ctx)
}
