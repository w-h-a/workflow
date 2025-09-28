package local

import (
	"context"
	"log/slog"

	"github.com/w-h-a/workflow/internal/engine/clients/notifier"
)

type localNotifier struct {
	options notifier.Options
}

func (n *localNotifier) Notify(ctx context.Context, event notifier.Event, opts ...notifier.NotifyOption) error {
	slog.InfoContext(
		ctx,
		"NOTIFY EVENT",
		"title", event.Title,
		"source", event.Source,
		"type", event.Type,
		"payload", event.Payload,
	)

	return nil
}

func NewNotifier(opts ...notifier.Option) notifier.Notifier {
	options := notifier.NewOptions(opts...)

	n := &localNotifier{
		options: options,
	}

	return n
}
