package main

import (
	"fmt"
	"os"

	"github.com/distiled/orphion/internal/cli"
)

func main() {
	if err := cli.New(nil).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "orphion:", err)
		os.Exit(1)
	}
}