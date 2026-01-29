package gorecovery

import (
	"fmt"
	"log/slog"
	"runtime/debug"

	"go.uber.org/zap"
)

// PanicHandler is a function signature for custom logic after a panic.
type PanicHandler func(metadata map[string]any)

type options struct {
	zapLogger  *zap.Logger
	slogLogger *slog.Logger
	handler    PanicHandler
	metadata   map[string]any
}

type Option func(*options)

// WithMetadata allows the handler function to use metadata to perform operation
func WithMetadata(data map[string]any) Option {
	return func(o *options) {
		o.metadata = data
	}
}

func WithZap(l *zap.Logger) Option {
	return func(o *options) {
		o.zapLogger = l
	}
}

func WithSlog(l *slog.Logger) Option {
	return func(o *options) { o.slogLogger = l }
}

// WithHandler allows custome handling on panic
func WithHandler(h PanicHandler) Option {
	return func(o *options) {
		o.handler = h
	}
}

// Go executes the provided function in a new goroutine and catches panics.
func Go(fn func(), opts ...Option) {
	config := &options{}
	for _, opt := range opts {
		opt(config)
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()

				switch{
				case config.slogLogger != nil:
					attrs := []any{
						slog.Any("panic_error", r),
						slog.String("stacktrace", string(stack)),
					}
					for k, v := range config.metadata {
						attrs = append(attrs, slog.Any(k, v))
					}
					config.slogLogger.Error("goroutine panic recovered", attrs...)
				case config.zapLogger != nil:
					fields := []zap.Field{
						zap.Any("panic_error", r),
						zap.ByteString("stacktrace", stack),
					}
					for k, v := range config.metadata {
						fields = append(fields, zap.Any(k, v))
					}
					config.zapLogger.Error("goroutine panic recovered", fields...)
				default:
					fmt.Printf("--- PANIC RECOVERED ---\nError: %v\nStack Trace:\n%s\n-----------------------\n", r, stack)
				}

				// CUSTOM HANDLER LOGIC
				if config.handler != nil {
					config.handler(config.metadata)
				}
			}
		}()

		fn()
	}()
}
