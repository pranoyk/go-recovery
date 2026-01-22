package gorecovery

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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
	}, WithHandler(func(err any, stack []byte) {
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
	var capturedErr any
	var capturedStack []byte
	var wg sync.WaitGroup
	wg.Add(1)

	Go(func() {
		panic(expectedPanic)
	}, WithHandler(func(err any, stack []byte) {
		defer wg.Done()
		capturedErr = err
		capturedStack = stack
	}))

	wg.Wait()

	if capturedErr != expectedPanic {
		t.Errorf("Expected panic %v, got %v", expectedPanic, capturedErr)
	}
	if len(capturedStack) == 0 {
		t.Error("Stack trace should not be empty")
	}
	if !strings.Contains(string(capturedStack), "goroutine") {
		t.Error("Stack trace should contain goroutine information")
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
	}, WithLogger(logger), WithHandler(func(err any, stack []byte) {
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
	}, WithLogger(logger), WithHandler(func(err any, stack []byte) {
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
		name      string
		panicVal  any
		expectStr string
	}{
		{"String", "string panic", "string panic"},
		{"Error", errors.New("error panic"), "error panic"},
		{"Integer", 42, "42"},
		{"Struct", struct{ Msg string }{"custom panic"}, "{custom panic}"},
		{"Boolean", false, "false"},
		{"Float", 3.14, "3.14"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var capturedErr any
			var wg sync.WaitGroup
			wg.Add(1)

			Go(func() {
				panic(tc.panicVal)
			}, WithHandler(func(err any, stack []byte) {
				defer wg.Done()
				capturedErr = err
			}))

			wg.Wait()

			if fmt.Sprint(capturedErr) != tc.expectStr {
				t.Errorf("Expected %v, got %v", tc.expectStr, capturedErr)
			}
		})
	}
}

// TestGoStackTraceContent verifies stack trace contains relevant information
func TestGoStackTraceContent(t *testing.T) {
	var capturedStack []byte
	var wg sync.WaitGroup
	wg.Add(1)

	Go(func() {
		panicInNestedFunction()
	}, WithHandler(func(err any, stack []byte) {
		defer wg.Done()
		capturedStack = stack
	}))

	wg.Wait()

	stackStr := string(capturedStack)

	if len(capturedStack) == 0 {
		t.Fatal("Stack trace should not be empty")
	}
	if !strings.Contains(stackStr, "goroutine") {
		t.Error("Stack trace should contain 'goroutine'")
	}
	if !strings.Contains(stackStr, "panicInNestedFunction") {
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
			}, WithHandler(func(err any, stack []byte) {
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
	}, WithHandler(func(err any, stack []byte) {
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
	if !strings.Contains(output, "Stack Trace:") {
		t.Error("Output should contain stack trace header")
	}
}

// TestGoWithNilFunction verifies behavior when nil function is passed
func TestGoWithNilFunction(t *testing.T) {
	var capturedErr any
	var wg sync.WaitGroup
	wg.Add(1)

	Go(nil, WithHandler(func(err any, stack []byte) {
		defer wg.Done()
		capturedErr = err
	}))

	wg.Wait()

	if capturedErr == nil {
		t.Error("Should have captured a panic from nil function call")
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
		}, WithHandler(func(err any, stack []byte) {
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

// TestGoResourceCleanup verifies defer in function executes before recovery
func TestGoResourceCleanup(t *testing.T) {
	executionOrder := []string{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(1)

	Go(func() {
		defer func() {
			mu.Lock()
			executionOrder = append(executionOrder, "function defer")
			mu.Unlock()
		}()
		mu.Lock()
		executionOrder = append(executionOrder, "function body")
		mu.Unlock()
		panic("test panic")
	}, WithHandler(func(err any, stack []byte) {
		defer wg.Done()
		mu.Lock()
		executionOrder = append(executionOrder, "recovery handler")
		mu.Unlock()
	}))

	wg.Wait()

	expected := []string{"function body", "function defer", "recovery handler"}
	if len(executionOrder) != len(expected) {
		t.Fatalf("Expected %d steps, got %d", len(expected), len(executionOrder))
	}

	for i, step := range expected {
		if executionOrder[i] != step {
			t.Errorf("Step %d: expected %s, got %s", i, step, executionOrder[i])
		}
	}
}

// TestGoPanicAfterDelay verifies panic can occur after some execution
func TestGoPanicAfterDelay(t *testing.T) {
	executed := false
	var capturedErr any
	var wg sync.WaitGroup
	wg.Add(1)

	Go(func() {
		executed = true
		time.Sleep(50 * time.Millisecond)
		panic("delayed panic")
	}, WithHandler(func(err any, stack []byte) {
		defer wg.Done()
		capturedErr = err
	}))

	wg.Wait()

	if !executed {
		t.Error("Function should have executed before panic")
	}
	if capturedErr != "delayed panic" {
		t.Errorf("Expected 'delayed panic', got %v", capturedErr)
	}
}

// TestGoEmptyFunction verifies empty function executes without issues
func TestGoEmptyFunction(t *testing.T) {
	handlerCalled := false
	done := make(chan bool, 1)

	Go(func() {
		// Empty function
		done <- true
	}, WithHandler(func(err any, stack []byte) {
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

	var handlerCalled bool
	var capturedErr any
	var wg sync.WaitGroup
	wg.Add(1)

	Go(func() {
		panic("multi-option test")
	}, WithLogger(logger), WithHandler(func(err any, stack []byte) {
		defer wg.Done()
		handlerCalled = true
		capturedErr = err
	}))

	wg.Wait()

	if !handlerCalled {
		t.Error("Handler should have been called")
	}
	if capturedErr != "multi-option test" {
		t.Errorf("Expected 'multi-option test', got %v", capturedErr)
	}

	output := buf.String()
	if !strings.Contains(output, "goroutine panic recovered") {
		t.Error("Zap logger should have logged the panic")
	}
}

// TestGoNoOptions verifies function works with no options
func TestGoNoOptions(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	done := make(chan bool, 1)
	go func() {
		Go(func() {
			panic("no options panic")
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

	// Should use default fmt logging
	if !strings.Contains(output, "--- PANIC RECOVERED ---") {
		t.Error("Should use default logging when no options provided")
	}
	if !strings.Contains(output, "no options panic") {
		t.Error("Output should contain panic value")
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
		}, WithLogger(logger))
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

// TestGoHandlerOnlyOption verifies handler works without logger
func TestGoHandlerOnlyOption(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	var handlerCalled bool
	var wg sync.WaitGroup
	wg.Add(1)

	Go(func() {
		panic("handler only test")
	}, WithHandler(func(err any, stack []byte) {
		defer wg.Done()
		handlerCalled = true
	}))

	wg.Wait()
	time.Sleep(10 * time.Millisecond)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if !handlerCalled {
		t.Error("Handler should have been called")
	}
	// Should still use default fmt logging
	if !strings.Contains(output, "--- PANIC RECOVERED ---") {
		t.Error("Should use default logging when only handler provided")
	}
}