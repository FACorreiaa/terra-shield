// Command shield is the Shield API server and ingestion worker. All wiring
// lives in internal/app; see its package doc for the commands.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"shield/internal/app"
)

// gitSHA is set at build time with -ldflags "-X main.gitSHA=...".
var gitSHA = "unknown"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := app.Run(ctx, os.Args[1:], gitSHA)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "shield:", err)
		os.Exit(1)
	}
}
