package main

import (
	"errors"
	"fmt"
	"os"
)

// Populated by GoReleaser via -ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "app: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return errors.New("subcommand is required")
	}

	switch args[0] {
	case "serve":
		return runServe(args[1:])
	case "version":
		fmt.Printf("version=%s commit=%s built=%s\n", version, commit, date)
		return nil
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("unknown subcommand: %q", args[0])
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  app serve [--config config.json] [--addr :8080] [--dry-run]")
	fmt.Fprintln(os.Stderr, "  app version")
	fmt.Fprintln(os.Stderr, "  app help")
}
