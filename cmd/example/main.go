// Package main runs a usage example of the worker manager: register handlers,
// start workers, and shut down on signal.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
	"workermanager"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wm := workermanager.NewWorkerManager(ctx)

	_, err := wm.AddHandler("ticker", &handler{tick: 30 * time.Second, name: "worker one"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "AddHandler ticker:", err)
		os.Exit(1)
	}

	// Start all workers (each runs in its own goroutine).
	if err := wm.StartAll(); err != nil {
		fmt.Fprintln(os.Stderr, "StartAll:", err)
		os.Exit(1)
	}

	// Run until interrupt (Ctrl+C) or SIGTERM.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("shutting down...")
	cancel()
	<-wm.StopAll()
	fmt.Println("stopped")
}

// handler simulates work sometimes and "no work" other times (returns None to sleep).
type handler struct {
	tick time.Duration
	name string
	n    int
}

func (h *handler) Handle(ctx context.Context) workermanager.HandleResult {
	select {
	case <-ctx.Done():
		return workermanager.Done()
	default:
		h.n++
		if h.n%3 == 0 {
			fmt.Printf("[worker] %s handler done (n=%d)\n", h.name, h.n)
			return workermanager.Done()
		}
		// No work: tell worker to sleep so we don't busy-loop.
		return workermanager.None(h.tick)
	}
}
