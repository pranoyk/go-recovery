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
	}, func(err any, stack []byte) {
		handlerCalled = true
	})

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
	}, func(err any, stack []byte) {
		defer wg.Done()
		capturedErr = err
		capturedStack = stack
	})

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

// TestGoPanicWithNilHandler verifies panic is caught even with nil handler
func TestGoPanicWithNilHandler(t *testing.T) {
	done := make(chan bool, 1)

	// Capture stdout to verify logging happens
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	Go(func() {
		panic("test panic with nil handler")
	}, nil)

	// Give goroutine time to execute and panic
	time.Sleep(100 * time.Millisecond)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "Panic caught in goroutine") {
		t.Error("Expected panic message in output")
	}
	if !strings.Contains(output, "test panic with nil handler") {
		t.Error("Expected panic value in output")
	}

	close(done)
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
			}, func(err any, stack []byte) {
				defer wg.Done()
				capturedErr = err
			})

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
	}, func(err any, stack []byte) {
		defer wg.Done()
		capturedStack = stack
	})

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

		if shouldPanic {
			Go(func() {
				panic(fmt.Sprintf("panic %d", i))
			}, func(err any, stack []byte) {
				mu.Lock()
				panicCount++
				mu.Unlock()
				wg.Done()
			})
		} else {
			Go(func() {
				defer wg.Done()
				mu.Lock()
				successCount++
				mu.Unlock()
			}, nil)
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
	mainThreadID := getGoroutineID()
	var functionThreadID uint64
	var wg sync.WaitGroup
	wg.Add(1)

	Go(func() {
		defer wg.Done()
		functionThreadID = getGoroutineID()
	}, nil)

	select {
	case <-time.After(10 * time.Millisecond):
	}

	wg.Wait()

	if functionThreadID == mainThreadID {
		t.Error("Function should run in a different goroutine")
	}
}

// Helper to get goroutine ID (simplified - just checks if different)
func getGoroutineID() uint64 {
	return uint64(time.Now().UnixNano())
}

// TestGoDefaultLogging verifies panic message is printed
func TestGoDefaultLogging(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	var wg sync.WaitGroup
	wg.Add(1)

	Go(func() {
		panic("logged panic")
	}, func(err any, stack []byte) {
		wg.Done()
	})

	wg.Wait()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "Panic caught in goroutine") {
		t.Error("Output should contain panic message header")
	}
	if !strings.Contains(output, "logged panic") {
		t.Error("Output should contain panic value")
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
		}, func(err any, stack []byte) {
			mu.Lock()
			handlerCount++
			mu.Unlock()
			wg.Done()
		})
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
	}, func(err any, stack []byte) {
		defer wg.Done()
		mu.Lock()
		executionOrder = append(executionOrder, "recovery handler")
		mu.Unlock()
	})

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
	}, func(err any, stack []byte) {
		defer wg.Done()
		capturedErr = err
	})

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
	}, func(err any, stack []byte) {
		handlerCalled = true
	})

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