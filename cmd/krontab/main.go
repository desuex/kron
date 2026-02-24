package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"kron/core/pkg/core"
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
	case "explain":
		if err := runExplain(args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			if errors.Is(err, errJobNotFound) {
				return 1
			}
			return 2
		}
		return 0
	case "next":
		if err := runNext(args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			if errors.Is(err, errJobNotFound) {
				return 1
			}
			return 2
		}
		return 0
	case "lint":
		return runLint(args[2:])
	case "-h", "--help", "help":
		printUsage()
		return 0
	default:
		printUsage()
		return 2
	}
}

func runExplain(args []string) error {
	job, parseArgs, err := normalizeExplainArgs(args)
	if err != nil {
		return err
	}

	fs := flag.NewFlagSet("explain", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	file := fs.String("file", "", "path to krontab file")
	at := fs.String("at", "", "period start timestamp (RFC3339)")
	window := fs.Duration("window", time.Hour, "window duration")
	mode := fs.String("mode", string(core.WindowModeAfter), "window mode: after|before|center")
	dist := fs.String("dist", string(core.DistributionUniform), "distribution (MVP: uniform)")
	format := fs.String("format", "text", "output format: text|json")

	if err := fs.Parse(parseArgs); err != nil {
		return err
	}
	if *at == "" {
		return fmt.Errorf("--at is required")
	}

	periodStart, err := time.Parse(time.RFC3339, *at)
	if err != nil {
		return fmt.Errorf("invalid --at value: %w", err)
	}

	settings := explainSettings{
		Window: *window,
		Mode:   core.WindowMode(*mode),
		Dist:   core.Distribution(*dist),
	}
	if *file != "" {
		var loadErr error
		settings, loadErr = loadJobSettings(*file, job, settings)
		if loadErr != nil {
			if errors.Is(loadErr, errJobNotFound) {
				return fmt.Errorf("%w: %s", errJobNotFound, job)
			}
			return loadErr
		}
	}

	decision, err := core.Decide(core.DecideInput{
		Job:         job,
		PeriodStart: periodStart,
		Window:      settings.Window,
		Mode:        settings.Mode,
		Dist:        settings.Dist,
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

type nextResult struct {
	Job       string          `json:"job"`
	Count     int             `json:"count"`
	Anchor    time.Time       `json:"anchor"`
	Decisions []core.Decision `json:"decisions"`
}

func runNext(args []string) error {
	job, parseArgs, err := normalizeNextArgs(args)
	if err != nil {
		return err
	}

	fs := flag.NewFlagSet("next", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	file := fs.String("file", "", "path to krontab file")
	count := fs.Int("count", 1, "number of next decisions to compute")
	at := fs.String("at", "", "anchor timestamp (RFC3339), default now")
	format := fs.String("format", "text", "output format: text|json")

	if err := fs.Parse(parseArgs); err != nil {
		return err
	}
	if *file == "" {
		return fmt.Errorf("--file is required for MVP")
	}
	if *count <= 0 {
		return fmt.Errorf("--count must be > 0")
	}

	anchor := time.Now().UTC()
	if *at != "" {
		parsed, err := time.Parse(time.RFC3339, *at)
		if err != nil {
			return fmt.Errorf("invalid --at value: %w", err)
		}
		anchor = parsed.UTC()
	}

	def, err := loadJobDefinition(*file, job, explainSettings{
		Window: 0,
		Mode:   core.WindowModeAfter,
		Dist:   core.DistributionUniform,
	})
	if err != nil {
		if errors.Is(err, errJobNotFound) {
			return fmt.Errorf("%w: %s", errJobNotFound, job)
		}
		return err
	}

	periods, err := def.Schedule.NextN(anchor, *count)
	if err != nil {
		return err
	}

	decisions := make([]core.Decision, 0, len(periods))
	for _, period := range periods {
		d, err := core.Decide(core.DecideInput{
			Job:         job,
			PeriodStart: period,
			Window:      def.Settings.Window,
			Mode:        def.Settings.Mode,
			Dist:        def.Settings.Dist,
		})
		if err != nil {
			return err
		}
		decisions = append(decisions, d)
	}

	res := nextResult{
		Job:       job,
		Count:     *count,
		Anchor:    anchor,
		Decisions: decisions,
	}

	switch strings.ToLower(*format) {
	case "text":
		printNextText(res)
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			return fmt.Errorf("encode json: %w", err)
		}
	default:
		return fmt.Errorf("invalid --format value: %q", *format)
	}

	return nil
}

func normalizeNextArgs(args []string) (string, []string, error) {
	var job string
	normalized := make([]string, 0, len(args))

	valueFlags := map[string]bool{
		"--file":   true,
		"--count":  true,
		"--at":     true,
		"--format": true,
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			normalized = append(normalized, arg)
			if strings.Contains(arg, "=") {
				continue
			}
			if valueFlags[arg] {
				if i+1 >= len(args) {
					return "", nil, fmt.Errorf("flag needs an argument: %s", arg)
				}
				i++
				normalized = append(normalized, args[i])
			}
			continue
		}

		if job != "" {
			return "", nil, fmt.Errorf("next requires exactly one <job> argument")
		}
		job = arg
	}
	if job == "" {
		return "", nil, fmt.Errorf("next requires exactly one <job> argument")
	}
	return job, normalized, nil
}

func printNextText(res nextResult) {
	fmt.Printf("job: %s\n", res.Job)
	fmt.Printf("anchor: %s\n", res.Anchor.Format(time.RFC3339))
	for i, d := range res.Decisions {
		fmt.Printf("%d. period_start=%s chosen_time=%s window=[%s, %s) mode=%s dist=%s\n",
			i+1,
			d.PeriodStart.Format(time.RFC3339),
			d.ChosenTime.Format(time.RFC3339),
			d.WindowStart.Format(time.RFC3339),
			d.WindowEnd.Format(time.RFC3339),
			d.Mode,
			d.Distribution,
		)
	}
}

func normalizeExplainArgs(args []string) (string, []string, error) {
	var job string
	normalized := make([]string, 0, len(args))

	valueFlags := map[string]bool{
		"--file":   true,
		"--at":     true,
		"--window": true,
		"--mode":   true,
		"--dist":   true,
		"--format": true,
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			normalized = append(normalized, arg)
			if strings.Contains(arg, "=") {
				continue
			}
			if valueFlags[arg] {
				if i+1 >= len(args) {
					return "", nil, fmt.Errorf("flag needs an argument: %s", arg)
				}
				i++
				normalized = append(normalized, args[i])
			}
			continue
		}

		if job != "" {
			return "", nil, fmt.Errorf("explain requires exactly one <job> argument")
		}
		job = arg
	}

	if job == "" {
		return "", nil, fmt.Errorf("explain requires exactly one <job> argument")
	}

	return job, normalized, nil
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
	fmt.Println("  krontab explain <job> --at <RFC3339> [--file <path>] [--window <duration>] [--mode after|before|center] [--dist uniform] [--format text|json]")
	fmt.Println("  krontab next <job> --file <path> [--count N] [--at <RFC3339>] [--format text|json]")
}
