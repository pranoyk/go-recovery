# go-recovery

[![Go Report Card](https://goreportcard.com/badge/github.com/pranoyk/go-recovery)](https://goreportcard.com/report/github.com/pranoyk/go-recovery)
[![Go Reference](https://pkg.go.dev/badge/github.com/pranoyk/go-recovery.svg)](https://pkg.go.dev/github.com/pranoyk/go-recovery)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A simple, lightweight Go library for graceful panic recovery in goroutines with structured logging and custom recovery handlers.

---

`go-recovery` ensures that a panic in a background goroutine doesn't crash your entire application. It automatically captures stack traces, provides structured logging via `zap` or `slog` (with a `fmt` fallback), and allows you to execute custom cleanup logic with contextual metadata.

Features

-  ✅ **Safe Goroutines**: Prevents panic from propagating and killing the process.

- ✅ **Structured Logging**: Deep integration with uber-go/zap and log/slog

- ✅ **Stack Trace Capture**: Automatically captures and logs full stack traces

- ✅ **Zero Config**: Defaults to standard fmt logging if no logger is provided.

- ✅ **Contextual Metadata**: Pass variables (IDs, connections, etc.) into the recovery flow for cleanup.

- ✅ **Custom Handlers**: Execute specific logic (like closing DB connections) after a panic occurs.

---

## Installation

```sh
go get github.com/pranoyk/go-recovery
```

---

## Quick Start

For simple background tasks where you just want to ensure the app stays alive and logs the error to the console:
Go

```go
import "github.com/pranoyk/go-recovery"

func main() {
    // Zero-config: Automatically logs panic and stack trace using fmt
    panicwrap.Go(func() {
        panic("something went wrong!")
    })

    // Keep main alive to see output
    select {}
}
```

---

## Advanced Usage

**Using log/slog (Standard Library)**

Ideal for modern Go applications using the standard structured logger.

```go
import (
    "log/slog"
    "github.com/pranoyk/go-recovery"
)

func main() {
    logger := slog.Default()

    panicwrap.Go(func() {
        panic("slog panic")
    }, panicwrap.WithSlog(logger))
}
```

**Using Zap Logger & Metadata**

In production, you likely want structured logs and a way to clean up resources if a goroutine fails.

```go
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
		gorecovery.WithLogger(logger),
		gorecovery.WithHandler(onPanic),
	)

	time.Sleep(100 * time.Millisecond)
}
```

**With Metadata**
```go
metadata := map[string]any{
    "user_id":        "user_123",
    "transaction_id": "txn_456",
    "operation":      "payment_processing",
}

gorecovery.Go(func() {
    // Process payment
    panic("payment gateway timeout")
}, 
    gorecovery.WithMetadata(metadata),
    gorecovery.WithHandler(func(meta map[string]any) {
        // Use metadata for recovery
        userID := meta["user_id"].(string)
        txnID := meta["transaction_id"].(string)
        
        // Retry transaction, notify user, log to monitoring system
        fmt.Printf("Payment failed for user %s, transaction %s\n", userID, txnID)
    }),
)
```

---

## API Reference

`Go(fn func(), opts ...Option)`

Runs the provided function in a new goroutine wrapped in a defer/recover block.

**Functional Options**

| Option | Description |
| ------ | ----------- |
| `WithSlog(*slog.Logger)` | Uses log/slog for structured error reporting. |
| `WithZap(*zap.Logger)` | Replaces default fmt logging with structured zap logs. |
| `WithMetadata(map[string]any)` | Attaches context to the panic. Metadata is included in logs and passed to the handler. |
| `WithHandler(PanicHandler)` | Defines a callback function to run after a panic is caught and logged. |


**Note on Logging:** The library is designed to be simple. If you provide both WithSlog and WithZap, the last one called will be used as the exclusive logger.

---

## License

MIT © [Pranoy K](https://github.com/pranoyk)