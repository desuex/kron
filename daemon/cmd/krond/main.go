package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"kron/daemon/pkg/daemon"
)

func main() {
	os.Exit(run(os.Args))
}

func run(args []string) int {
	if len(args) < 2 {
		printUsage()
		return 2
	}

	switch args[1] {
	case "start":
		if err := runStart(args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 2
		}
		return 0
	case "-h", "--help", "help":
		printUsage()
		return 0
	default:
		printUsage()
		return 2
	}
}

func runStart(args []string) error {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	configPath := fs.String("config", "", "path to krond config file or directory")
	stateDir := fs.String("state-dir", ".krond-state", "directory for persistent krond state")
	tick := fs.Duration("tick", time.Second, "scheduler tick interval")
	once := fs.Bool("once", false, "run one scheduling step and exit")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("start does not accept positional arguments")
	}
	if *configPath == "" {
		return fmt.Errorf("--config is required")
	}

	ctx := context.Background()
	if !*once {
		var cancel context.CancelFunc
		ctx, cancel = signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
	}

	return daemon.Start(ctx, daemon.StartOptions{
		ConfigPath: *configPath,
		StateDir:   *stateDir,
		Tick:       *tick,
		Once:       *once,
	})
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  krond start --config <path> [--state-dir <path>] [--tick <duration>] [--once]")
}
