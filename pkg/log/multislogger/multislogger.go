package multislogger

import (
	"context"
	"log/slog"

	slogmulti "github.com/samber/slog-multi"
)

type contextKey string

func (c contextKey) String() string {
	return string(c)
}

const (
	// KolideSessionIdKey this the also the saml session id
	KolideSessionIdKey contextKey = "kolide_session_id"
	SpanIdKey          contextKey = "span_id"
	TraceIdKey         contextKey = "trace_id"
	TraceSampledKey    contextKey = "trace_sampled"
)

// ctxValueKeysToAdd is a list of context keys that will be
// added as log attributes
var ctxValueKeysToAdd = []contextKey{
	SpanIdKey,
	TraceIdKey,
	KolideSessionIdKey,
	TraceSampledKey,
}

type MultiSlogger struct {
	*slog.Logger
	fanoutHandler *slogmulti.FanoutHandlerPersistent
}

// New creates a new multislogger if no handlers are passed in, it will
// create a logger that discards all logs
func New(h ...slog.Handler) *MultiSlogger {
	ms := new(MultiSlogger)
	ms.fanoutHandler = &slogmulti.FanoutHandlerPersistent{}
	ms.Logger = slog.New(
		slogmulti.
			Pipe(slogmulti.NewHandleInlineMiddleware(utcTimeMiddleware)).
			Pipe(slogmulti.NewHandleInlineMiddleware(ctxValuesMiddleWare)).
			Handler(ms.fanoutHandler),
	)

	ms.fanoutHandler.AddHandler(h...)
	return ms
}

// AddHandler adds a handler to the multislogger, this creates a branch new
// slog.Logger under the the hood, mean any attributes added with
// Logger.With will be lost
func (ms *MultiSlogger) AddHandler(handler ...slog.Handler) {
	ms.fanoutHandler.AddHandler(handler...)
}

func utcTimeMiddleware(ctx context.Context, record slog.Record, next func(context.Context, slog.Record) error) error {
	record.Time = record.Time.UTC()
	return next(ctx, record)
}

func ctxValuesMiddleWare(ctx context.Context, record slog.Record, next func(context.Context, slog.Record) error) error {
	for _, key := range ctxValueKeysToAdd {
		if v := ctx.Value(key); v != nil {
			record.AddAttrs(slog.Attr{
				Key:   key.String(),
				Value: slog.AnyValue(v),
			})
		}
	}

	return next(ctx, record)
}
