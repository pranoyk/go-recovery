package gorecovery

import (
	"fmt"
	"runtime/debug"
)

// PanicHandler is a function signature for custom logic after a panic.
type PanicHandler func(err any, stack []byte)

// Go executes the provided function in a new goroutine and catches panics.
func Go(fn func(), onPanic PanicHandler) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				fmt.Printf("Panic caught in goroutine: %v\n%s\n", r, stack)

				if onPanic != nil {
					onPanic(r, stack)
				}
			}
		}()

		fn()
	}()
}