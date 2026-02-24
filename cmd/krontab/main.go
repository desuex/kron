package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"kron/core/pkg/core"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "explain":
		if err := runExplain(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(2)
		}
	case "lint":
		os.Exit(runLint(os.Args[2:]))
	case "-h", "--help", "help":
		printUsage()
	default:
		printUsage()
		os.Exit(2)
	}
}

func runExplain(args []string) error {
	fs := flag.NewFlagSet("explain", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	at := fs.String("at", "", "period start timestamp (RFC3339)")
	window := fs.Duration("window", time.Hour, "window duration")
	mode := fs.String("mode", string(core.WindowModeAfter), "window mode: after|before|center")
	format := fs.String("format", "text", "output format: text|json")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		return fmt.Errorf("explain requires exactly one <job> argument")
	}
	if *at == "" {
		return fmt.Errorf("--at is required")
	}

	periodStart, err := time.Parse(time.RFC3339, *at)
	if err != nil {
		return fmt.Errorf("invalid --at value: %w", err)
	}

	decision, err := core.Decide(core.DecideInput{
		Job:         fs.Arg(0),
		PeriodStart: periodStart,
		Window:      *window,
		Mode:        core.WindowMode(*mode),
		Dist:        core.DistributionUniform,
	})
	if err != nil {
		return err
	}

	switch strings.ToLower(*format) {
	case "text":
		printText(decision)
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(decision); err != nil {
			return fmt.Errorf("encode json: %w", err)
		}
	default:
		return fmt.Errorf("invalid --format value: %q", *format)
	}

	return nil
}

func printText(d core.Decision) {
	fmt.Printf("job: %s\n", d.Job)
	fmt.Printf("period_start: %s\n", d.PeriodStart.Format(time.RFC3339))
	fmt.Printf("window: [%s, %s)\n", d.WindowStart.Format(time.RFC3339), d.WindowEnd.Format(time.RFC3339))
	fmt.Printf("mode: %s\n", d.Mode)
	fmt.Printf("distribution: %s\n", d.Distribution)
	fmt.Printf("seed_hash: %s\n", d.SeedHash)
	fmt.Printf("chosen_time: %s\n", d.ChosenTime.Format(time.RFC3339))
}

func printUsage() {
	fmt.Println("krontab - Kron CLI (MVP)")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  krontab lint --file <path> [--format text|json]")
	fmt.Println("  krontab explain <job> --at <RFC3339> [--window <duration>] [--mode after|before|center] [--format text|json]")
}
