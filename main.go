package main

import (
	"os"

	"ocstats/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
