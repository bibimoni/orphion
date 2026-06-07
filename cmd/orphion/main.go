package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/distiled/orphion/internal/cli"
)

func main() {
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt, syscall.SIGTERM,
	)
	defer cancel()

	_ = ctx

	if err := cli.New(nil).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "orphion:", err)
		os.Exit(exitCodeFor(err))
	}
}

func exitCodeFor(err error) int {
	if err == nil {
		return 0
	}
	return 1
}
