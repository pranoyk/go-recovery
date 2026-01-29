package gorecovery

import (
	"fmt"
	"log/slog"
	"runtime/debug"

	"go.uber.org/zap"
)

type PanicHandler func(metadata map[string]any)

type logFn func(err any, stack []byte, meta map[string]any)

type options struct {
	log      logFn
	handler  PanicHandler
	metadata map[string]any
}

type Option func(*options)

// WithZap sets Zap as the exclusive logger
func WithZap(l *zap.Logger) Option {
	return func(o *options) {
		o.log = func(err any, stack []byte, meta map[string]any) {
			fields := []zap.Field{
				zap.Any("error", err),
				zap.ByteString("stacktrace", stack),
			}
			for k, v := range meta {
				fields = append(fields, zap.Any(k, v))
			}
			l.Error("goroutine panic recovered", fields...)
		}
	}
}

// WithSlog sets Slog as the exclusive logger
func WithSlog(l *slog.Logger) Option {
	return func(o *options) {
		o.log = func(err any, stack []byte, meta map[string]any) {
			attrs := []any{
				slog.Any("error", err),
				slog.String("stacktrace", string(stack)),
			}
			for k, v := range meta {
				attrs = append(attrs, slog.Any(k, v))
			}
			l.Error("goroutine panic recovered", attrs...)
		}
	}
}

func WithMetadata(data map[string]any) Option {
	return func(o *options) { o.metadata = data }
}

func WithHandler(h PanicHandler) Option {
	return func(o *options) { o.handler = h }
}

func Go(fn func(), opts ...Option) {
	config := &options{metadata: make(map[string]any)}
	for _, opt := range opts {
		opt(config)
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()

				// Log using the configured function or fallback to fmt
				if config.log != nil {
					config.log(r, stack, config.metadata)
				} else {
					fmt.Printf("--- PANIC RECOVERED ---\nError: %v\nMetadata: %v\nstacktrace:\n%s\n", r, config.metadata, stack)
				}

				// Run custom handler
				if config.handler != nil {
					config.handler(config.metadata)
				}
			}
		}()
		fn()
	}()
}