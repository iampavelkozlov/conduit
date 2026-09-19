package eventing

import "context"

type Publisher interface {
	Publish(context.Context, string, string, *Envelope) error
}

type Handler interface {
	Handle(context.Context, *Envelope) error
}

type HandlerFunc func(context.Context, *Envelope) error

func (f HandlerFunc) Handle(ctx context.Context, event *Envelope) error {
	return f(ctx, event)
}

type Consumer interface {
	Run(context.Context, Handler) error
}
