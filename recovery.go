package gorecovery

import (
	"fmt"
	"runtime/debug"

	"go.uber.org/zap"
)

// PanicHandler is a function signature for custom logic after a panic.
type PanicHandler func(err any, stack []byte)

type options struct {
	logger  *zap.Logger
	handler PanicHandler
}

type Option func(*options)

// WithLogger overrides the default fmt logger with a Zap instance.
func WithLogger(l *zap.Logger) Option {
	return func(o *options) {
		o.logger = l
	}
}

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

				if config.logger != nil {
					// Use Zap if provided
					config.logger.Error("goroutine panic recovered",
						zap.Any("error", r),
						zap.ByteString("stacktrace", stack),
					)
				} else {
					// Fallback to default fmt logging
					fmt.Printf("--- PANIC RECOVERED ---\nError: %v\nStack Trace:\n%s\n-----------------------\n", r, stack)
				}

				// CUSTOM HANDLER LOGIC
				if config.handler != nil {
					config.handler(r, stack)
				}
			}
		}()

		fn()
	}()
}