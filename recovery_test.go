package gorecovery

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// TestGoNoPanic verifies normal execution without panic
func TestGoNoPanic(t *testing.T) {
	executed := false
	handlerCalled := false
	var wg sync.WaitGroup
	wg.Add(1)

	Go(func() {
		defer wg.Done()
		executed = true
	}, WithHandler(func(metadata map[string]any) {
		handlerCalled = true
	}))

	wg.Wait()

	if !executed {
		t.Error("Function should have executed")
	}
	if handlerCalled {
		t.Error("Panic handler should not have been called")
	}
}

// TestGoPanicWithHandler verifies panic is caught and handler is invoked
func TestGoPanicWithHandler(t *testing.T) {
	expectedPanic := "test panic"
	handlerCalled := false
	var wg sync.WaitGroup
	wg.Add(1)

	Go(func() {
		panic(expectedPanic)
	}, WithHandler(func(metadata map[string]any) {
		defer wg.Done()
		handlerCalled = true
	}))

	wg.Wait()

	if !handlerCalled {
		t.Error("Handler should have been called")
	}
}

// TestGoPanicWithHandlerAndMetadata verifies handler receives metadata
func TestGoPanicWithHandlerAndMetadata(t *testing.T) {
	expectedMetadata := map[string]any{
		"user_id":  123,
		"action":   "test_action",
		"endpoint": "/api/test",
	}

	var receivedMetadata map[string]any
	var wg sync.WaitGroup
	wg.Add(1)

	Go(func() {
		panic("test panic with metadata")
	}, WithMetadata(expectedMetadata), WithHandler(func(metadata map[string]any) {
		defer wg.Done()
		receivedMetadata = metadata
	}))

	wg.Wait()

	if receivedMetadata == nil {
		t.Fatal("Handler should have received metadata")
	}

	if receivedMetadata["user_id"] != 123 {
		t.Errorf("Expected user_id 123, got %v", receivedMetadata["user_id"])
	}
	if receivedMetadata["action"] != "test_action" {
		t.Errorf("Expected action 'test_action', got %v", receivedMetadata["action"])
	}
	if receivedMetadata["endpoint"] != "/api/test" {
		t.Errorf("Expected endpoint '/api/test', got %v", receivedMetadata["endpoint"])
	}
}

// TestGoPanicWithoutHandler verifies panic is caught even without handler
func TestGoPanicWithoutHandler(t *testing.T) {
	// Capture stdout to verify default logging happens
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	done := make(chan bool, 1)
	go func() {
		Go(func() {
			panic("test panic without handler")
		})
		time.Sleep(100 * time.Millisecond)
		done <- true
	}()

	<-done

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "--- PANIC RECOVERED ---") {
		t.Error("Output should contain default panic header")
	}
	if !strings.Contains(output, "test panic without handler") {
		t.Error("Output should contain panic value")
	}
}

// TestGoPanicWithZapLogger verifies zap logger is used when provided
func TestGoPanicWithZapLogger(t *testing.T) {
	// Create a buffer to capture zap logs
	var buf bytes.Buffer
	writer := zapcore.AddSync(&buf)

	encoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
		MessageKey:  "msg",
		LevelKey:    "level",
		EncodeLevel: zapcore.LowercaseLevelEncoder,
	})

	core := zapcore.NewCore(encoder, writer, zapcore.DebugLevel)
	logger := zap.New(core)

	var wg sync.WaitGroup
	wg.Add(1)

	Go(func() {
		panic("zap panic test")
	}, WithZap(logger), WithHandler(func(metadata map[string]any) {
		defer wg.Done()
	}))

	wg.Wait()

	output := buf.String()

	if !strings.Contains(output, "goroutine panic recovered") {
		t.Error("Zap log should contain panic message")
	}
	if !strings.Contains(output, "zap panic test") {
		t.Error("Zap log should contain panic value")
	}
	if !strings.Contains(output, "error") {
		t.Error("Zap log should contain error field")
	}
}

// TestGoWithZapLoggerAndHandler verifies both logger and handler work together
func TestGoWithZapLoggerAndHandler(t *testing.T) {
	var buf bytes.Buffer
	writer := zapcore.AddSync(&buf)

	encoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
		MessageKey:  "msg",
		LevelKey:    "level",
		EncodeLevel: zapcore.LowercaseLevelEncoder,
	})

	core := zapcore.NewCore(encoder, writer, zapcore.DebugLevel)
	logger := zap.New(core)

	var handlerCalled bool
	var wg sync.WaitGroup
	wg.Add(1)

	Go(func() {
		panic("combined test")
	}, WithZap(logger), WithHandler(func(metadata map[string]any) {
		defer wg.Done()
		handlerCalled = true
	}))

	wg.Wait()

	if !handlerCalled {
		t.Error("Handler should have been called")
	}

	output := buf.String()
	if !strings.Contains(output, "goroutine panic recovered") {
		t.Error("Zap logger should have logged the panic")
	}
}

// TestGoDifferentPanicTypes verifies different panic value types are handled
func TestGoDifferentPanicTypes(t *testing.T) {
	testCases := []struct {
		name     string
		panicVal any
	}{
		{"String", "string panic"},
		{"Error", errors.New("error panic")},
		{"Integer", 42},
		{"Struct", struct{ Msg string }{"custom panic"}},
		{"Boolean", false},
		{"Float", 3.14},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handlerCalled := false
			var wg sync.WaitGroup
			wg.Add(1)

			Go(func() {
				panic(tc.panicVal)
			}, WithHandler(func(metadata map[string]any) {
				defer wg.Done()
				handlerCalled = true
			}))

			wg.Wait()

			if !handlerCalled {
				t.Error("Handler should have been called for panic type:", tc.name)
			}
		})
	}
}

// TestGoStackTraceLogged verifies stack trace is logged
func TestGoStackTraceLogged(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	var wg sync.WaitGroup
	wg.Add(1)

	Go(func() {
		panicInNestedFunction()
	}, WithHandler(func(metadata map[string]any) {
		defer wg.Done()
	}))

	wg.Wait()
	time.Sleep(10 * time.Millisecond)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "stacktrace") {
		t.Error("Output should contain stack trace header")
	}
	if !strings.Contains(output, "goroutine") {
		t.Error("Stack trace should contain goroutine information")
	}
	if !strings.Contains(output, "panicInNestedFunction") {
		t.Error("Stack trace should contain function name")
	}
}

func panicInNestedFunction() {
	panic("nested panic")
}

// TestGoMultipleConcurrentGoroutines verifies multiple goroutines are handled independently
func TestGoMultipleConcurrentGoroutines(t *testing.T) {
	const numGoroutines = 10
	panicCount := 0
	successCount := 0
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		shouldPanic := i%2 == 0
		index := i // Capture loop variable

		if shouldPanic {
			Go(func() {
				panic(fmt.Sprintf("panic %d", index))
			}, WithHandler(func(metadata map[string]any) {
				mu.Lock()
				panicCount++
				mu.Unlock()
				wg.Done()
			}))
		} else {
			Go(func() {
				defer wg.Done()
				mu.Lock()
				successCount++
				mu.Unlock()
			})
		}
	}

	wg.Wait()

	expectedPanics := 5
	expectedSuccess := 5

	if panicCount != expectedPanics {
		t.Errorf("Expected %d panics, got %d", expectedPanics, panicCount)
	}
	if successCount != expectedSuccess {
		t.Errorf("Expected %d successful executions, got %d", expectedSuccess, successCount)
	}
}

// TestGoRunsInSeparateGoroutine verifies function runs asynchronously
func TestGoRunsInSeparateGoroutine(t *testing.T) {
	executed := false
	var wg sync.WaitGroup
	wg.Add(1)

	Go(func() {
		defer wg.Done()
		time.Sleep(50 * time.Millisecond)
		executed = true
	})

	// Check that we don't block immediately
	select {
	case <-time.After(10 * time.Millisecond):
		// Good - we didn't block
		if executed {
			t.Error("Function should not have completed yet")
		}
	}

	wg.Wait()

	if !executed {
		t.Error("Function should have completed after wait")
	}
}

// TestGoDefaultLogging verifies default fmt panic message is printed
func TestGoDefaultLogging(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	var wg sync.WaitGroup
	wg.Add(1)

	Go(func() {
		panic("logged panic")
	}, WithHandler(func(metadata map[string]any) {
		defer wg.Done()
	}))

	wg.Wait()

	// Give a small delay to ensure output is written
	time.Sleep(10 * time.Millisecond)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "--- PANIC RECOVERED ---") {
		t.Error("Output should contain default panic header")
	}
	if !strings.Contains(output, "logged panic") {
		t.Error("Output should contain panic value")
	}
	if !strings.Contains(output, "stacktrace") {
		t.Error("Output should contain stack trace header")
	}
}

// TestGoRapidSuccessiveCalls verifies no race conditions
func TestGoRapidSuccessiveCalls(t *testing.T) {
	const numCalls = 100
	handlerCount := 0
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < numCalls; i++ {
		wg.Add(1)
		Go(func() {
			panic("rapid panic")
		}, WithHandler(func(metadata map[string]any) {
			mu.Lock()
			handlerCount++
			mu.Unlock()
			wg.Done()
		}))
	}

	wg.Wait()

	if handlerCount != numCalls {
		t.Errorf("Expected %d handler calls, got %d", numCalls, handlerCount)
	}
}

// TestGoEmptyFunction verifies empty function executes without issues
func TestGoEmptyFunction(t *testing.T) {
	handlerCalled := false
	done := make(chan bool, 1)

	Go(func() {
		// Empty function
		done <- true
	}, WithHandler(func(metadata map[string]any) {
		handlerCalled = true
	}))

	select {
	case <-done:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("Empty function should complete")
	}

	if handlerCalled {
		t.Error("Handler should not be called for empty function")
	}
}

// TestGoMultipleOptions verifies multiple options can be combined
func TestGoMultipleOptions(t *testing.T) {
	var buf bytes.Buffer
	writer := zapcore.AddSync(&buf)

	encoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
		MessageKey:  "msg",
		LevelKey:    "level",
		EncodeLevel: zapcore.LowercaseLevelEncoder,
	})

	core := zapcore.NewCore(encoder, writer, zapcore.DebugLevel)
	logger := zap.New(core)

	metadata := map[string]any{
		"request_id": "12345",
		"user":       "test_user",
	}

	var handlerCalled bool
	var receivedMetadata map[string]any
	var wg sync.WaitGroup
	wg.Add(1)

	Go(func() {
		panic("multi-option test")
	}, WithZap(logger), WithMetadata(metadata), WithHandler(func(meta map[string]any) {
		defer wg.Done()
		handlerCalled = true
		receivedMetadata = meta
	}))

	wg.Wait()

	if !handlerCalled {
		t.Error("Handler should have been called")
	}
	if receivedMetadata["request_id"] != "12345" {
		t.Error("Metadata should have been passed to handler")
	}

	output := buf.String()
	if !strings.Contains(output, "goroutine panic recovered") {
		t.Error("Zap logger should have logged the panic")
	}
}

// TestGoZapLoggerWithoutHandler verifies zap logger works without handler
func TestGoZapLoggerWithoutHandler(t *testing.T) {
	var buf bytes.Buffer
	writer := zapcore.AddSync(&buf)

	encoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
		MessageKey:  "msg",
		LevelKey:    "level",
		EncodeLevel: zapcore.LowercaseLevelEncoder,
	})

	core := zapcore.NewCore(encoder, writer, zapcore.DebugLevel)
	logger := zap.New(core)

	done := make(chan bool, 1)
	go func() {
		Go(func() {
			panic("zap only test")
		}, WithZap(logger))
		time.Sleep(100 * time.Millisecond)
		done <- true
	}()

	<-done

	output := buf.String()
	if !strings.Contains(output, "goroutine panic recovered") {
		t.Error("Zap logger should have logged the panic")
	}
	if !strings.Contains(output, "zap only test") {
		t.Error("Zap log should contain panic value")
	}
}

// TestGoMetadataWithoutHandler verifies metadata is stored but not used if no handler
func TestGoMetadataWithoutHandler(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	metadata := map[string]any{
		"key": "value",
	}

	done := make(chan bool, 1)
	go func() {
		Go(func() {
			panic("metadata without handler")
		}, WithMetadata(metadata))
		time.Sleep(100 * time.Millisecond)
		done <- true
	}()

	<-done

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Should still log the panic even without handler
	if !strings.Contains(output, "--- PANIC RECOVERED ---") {
		t.Error("Should use default logging")
	}
}

// TestGoHandlerWithEmptyMetadata verifies handler works with empty metadata
func TestGoHandlerWithEmptyMetadata(t *testing.T) {
	emptyMetadata := map[string]any{}
	var receivedMetadata map[string]any
	var wg sync.WaitGroup
	wg.Add(1)

	Go(func() {
		panic("empty metadata test")
	}, WithMetadata(emptyMetadata), WithHandler(func(metadata map[string]any) {
		defer wg.Done()
		receivedMetadata = metadata
	}))

	wg.Wait()

	if receivedMetadata == nil {
		t.Error("Metadata should not be nil")
	}
	if len(receivedMetadata) != 0 {
		t.Error("Metadata should be empty")
	}
}

// TestGoMetadataIntegrity verifies metadata is not modified during panic handling
func TestGoMetadataIntegrity(t *testing.T) {
	originalMetadata := map[string]any{
		"id":     1,
		"name":   "test",
		"active": true,
	}

	var receivedMetadata map[string]any
	var wg sync.WaitGroup
	wg.Add(1)

	Go(func() {
		panic("metadata integrity test")
	}, WithMetadata(originalMetadata), WithHandler(func(metadata map[string]any) {
		defer wg.Done()
		receivedMetadata = metadata
	}))

	wg.Wait()

	// Verify all fields are preserved
	if receivedMetadata["id"] != 1 {
		t.Errorf("Expected id 1, got %v", receivedMetadata["id"])
	}
	if receivedMetadata["name"] != "test" {
		t.Errorf("Expected name 'test', got %v", receivedMetadata["name"])
	}
	if receivedMetadata["active"] != true {
		t.Errorf("Expected active true, got %v", receivedMetadata["active"])
	}
}

// TestGoComplexMetadata verifies handler receives complex metadata types
func TestGoComplexMetadata(t *testing.T) {
	type CustomStruct struct {
		Field1 string
		Field2 int
	}

	complexMetadata := map[string]any{
		"string":  "value",
		"int":     42,
		"float":   3.14,
		"bool":    true,
		"slice":   []string{"a", "b", "c"},
		"map":     map[string]int{"x": 1, "y": 2},
		"struct":  CustomStruct{Field1: "test", Field2: 100},
		"pointer": &CustomStruct{Field1: "ptr", Field2: 200},
	}

	var receivedMetadata map[string]any
	var wg sync.WaitGroup
	wg.Add(1)

	Go(func() {
		panic("complex metadata test")
	}, WithMetadata(complexMetadata), WithHandler(func(metadata map[string]any) {
		defer wg.Done()
		receivedMetadata = metadata
	}))

	wg.Wait()

	if receivedMetadata["string"] != "value" {
		t.Error("String metadata not preserved")
	}
	if receivedMetadata["int"] != 42 {
		t.Error("Int metadata not preserved")
	}
	if receivedMetadata["bool"] != true {
		t.Error("Bool metadata not preserved")
	}

	// Verify complex types
	slice, ok := receivedMetadata["slice"].([]string)
	if !ok || len(slice) != 3 {
		t.Error("Slice metadata not preserved")
	}

	mapData, ok := receivedMetadata["map"].(map[string]int)
	if !ok || mapData["x"] != 1 {
		t.Error("Map metadata not preserved")
	}
}

// TestGoHandlerCanAccessMetadataForRecovery verifies practical use case
func TestGoHandlerCanAccessMetadataForRecovery(t *testing.T) {
	// Simulate a real-world scenario where metadata helps with recovery
	metadata := map[string]any{
		"transaction_id": "txn_12345",
		"user_id":        "user_999",
		"operation":      "database_write",
		"retry_count":    0,
	}

	handlerExecuted := false
	var capturedTransactionID string
	var wg sync.WaitGroup
	wg.Add(1)

	Go(func() {
		panic("database connection lost")
	}, WithMetadata(metadata), WithHandler(func(meta map[string]any) {
		defer wg.Done()
		handlerExecuted = true

		// Handler can use metadata to perform recovery actions
		if txnID, ok := meta["transaction_id"].(string); ok {
			capturedTransactionID = txnID
			// In real scenario: mark transaction for retry, send alert, etc.
		}
	}))

	wg.Wait()

	if !handlerExecuted {
		t.Error("Handler should have been executed")
	}
	if capturedTransactionID != "txn_12345" {
		t.Errorf("Expected transaction_id 'txn_12345', got '%s'", capturedTransactionID)
	}
}

func TestGoPanicWithSlogLogger(t *testing.T) {
	// Create a buffer to capture slog logs
	var buf bytes.Buffer

	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)

	var wg sync.WaitGroup
	wg.Add(1)

	Go(func() {
		panic("slog panic test")
	}, WithSlog(logger), WithHandler(func(metadata map[string]any) {
		defer wg.Done()
	}))

	wg.Wait()

	output := buf.String()

	if !strings.Contains(output, "goroutine panic recovered") {
		t.Error("Slog should contain panic message")
	}
	if !strings.Contains(output, "slog panic test") {
		t.Error("Slog should contain panic value")
	}
	if !strings.Contains(output, "stacktrace") {
		t.Error("Slog should contain stacktrace field")
	}
}

// TestGoWithSlogLoggerAndHandler verifies both slog logger and handler work together
func TestGoWithSlogLoggerAndHandler(t *testing.T) {
	var buf bytes.Buffer

	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)

	var handlerCalled bool
	var wg sync.WaitGroup
	wg.Add(1)

	Go(func() {
		panic("slog combined test")
	}, WithSlog(logger), WithHandler(func(metadata map[string]any) {
		defer wg.Done()
		handlerCalled = true
	}))

	wg.Wait()

	if !handlerCalled {
		t.Error("Handler should have been called")
	}

	output := buf.String()
	if !strings.Contains(output, "goroutine panic recovered") {
		t.Error("Slog logger should have logged the panic")
	}
}

// TestGoSlogWithMetadata verifies slog logs metadata fields
func TestGoSlogWithMetadata(t *testing.T) {
	var buf bytes.Buffer

	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)

	metadata := map[string]any{
		"request_id": "req_123",
		"user_id":    "user_456",
		"endpoint":   "/api/test",
	}

	var wg sync.WaitGroup
	wg.Add(1)

	Go(func() {
		panic("slog metadata test")
	}, WithSlog(logger), WithMetadata(metadata), WithHandler(func(meta map[string]any) {
		defer wg.Done()
	}))

	wg.Wait()

	output := buf.String()

	if !strings.Contains(output, "request_id") {
		t.Error("Slog should contain request_id metadata")
	}
	if !strings.Contains(output, "req_123") {
		t.Error("Slog should contain request_id value")
	}
	if !strings.Contains(output, "user_id") {
		t.Error("Slog should contain user_id metadata")
	}
	if !strings.Contains(output, "user_456") {
		t.Error("Slog should contain user_id value")
	}
}

// TestGoSlogLoggerWithoutHandler verifies slog logger works without handler
func TestGoSlogLoggerWithoutHandler(t *testing.T) {
	var buf bytes.Buffer

	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)

	done := make(chan bool, 1)
	go func() {
		Go(func() {
			panic("slog only test")
		}, WithSlog(logger))
		time.Sleep(100 * time.Millisecond)
		done <- true
	}()

	<-done

	output := buf.String()
	if !strings.Contains(output, "goroutine panic recovered") {
		t.Error("Slog logger should have logged the panic")
	}
	if !strings.Contains(output, "slog only test") {
		t.Error("Slog log should contain panic value")
	}
}