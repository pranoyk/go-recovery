package main

import (
	"fmt"
	"time"

	"github.com/pranoyk/go-recovery"
)

func main() {
	fmt.Println("Starting application with default logger...")

	// We simply pass the function.
	// The library will catch the panic and print to console via fmt.
	gorecovery.Go(func() {
		fmt.Println("Goroutine is running...")

		// Simulating a runtime error (e.g., nil pointer or manual panic)
		panic("critical failure in background task")
	})

	// Wait a moment to allow the goroutine to panic and log
	time.Sleep(100 * time.Millisecond)
	fmt.Println("Main function exiting.")
}
