package gorecovery

import (
	"fmt"
	"runtime/debug"

	"go.uber.org/zap"
)

// PanicHandler is a function signature for custom logic after a panic.
type PanicHandler func(metadata map[string]any)

type options struct {
	logger   *zap.Logger
	handler  PanicHandler
	metadata map[string]any
}

type Option func(*options)

// WithMetadata allows the handler function to use metadata to perform operation
func WithMetadata(data map[string]any) Option {
	return func(o *options) {
		o.metadata = data
	}
}

// WithLogger overrides the default fmt logger with a Zap instance.
func WithLogger(l *zap.Logger) Option {
	return func(o *options) {
		o.logger = l
	}
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
					config.handler(config.metadata)
				}
			}
		}()

		fn()
	}()
}
