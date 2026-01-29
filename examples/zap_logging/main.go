package main

import (
	"time"

	"github.com/pranoyk/go-recovery"
	"go.uber.org/zap"
)

func main() {
	// 1. Initialize a structured zap logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// 2. Define an optional custom handler (e.g., for metrics or Sentry)
	onPanic := func(metadata map[string]any) {
		logger.Info("Custom handler executed: Sending alert to monitoring system...")
	}

	logger.Info("Starting application with Zap logger...")

	// 3. Use the library with Functional Options
	gorecovery.Go(func() {
		logger.Info("Executing risky logic...")

		// Trigger a panic
		var slice []string
		_ = slice[0] // This will cause an index out of range panic
	},
		gorecovery.WithZap(logger),
		gorecovery.WithHandler(onPanic),
	)

	time.Sleep(100 * time.Millisecond)
}
