package daemon

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"kron/core/pkg/core"
)

func LoadJobs(path string) ([]JobConfig, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat config path: %w", err)
	}

	if info.IsDir() {
		return loadJobsFromDir(path)
	}
	return loadJobsFromFile(path)
}

func loadJobsFromDir(path string) ([]JobConfig, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read config dir: %w", err)
	}

	out := make([]JobConfig, 0)
	seen := map[string]bool{}
	for _, ent := range entries {
		if ent.IsDir() || strings.HasPrefix(ent.Name(), ".") {
			continue
		}
		filePath := filepath.Join(path, ent.Name())
		jobs, err := loadJobsFromFile(filePath)
		if err != nil {
			return nil, err
		}
		for _, job := range jobs {
			if seen[job.Identity] {
				return nil, fmt.Errorf("duplicate job identity %q", job.Identity)
			}
			seen[job.Identity] = true
			out = append(out, job)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no jobs found in %s", path)
	}
	return out, nil
}

func loadJobsFromFile(path string) ([]JobConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}

	s := bufio.NewScanner(f)
	lineNo := 0
	out := make([]JobConfig, 0)
	seen := map[string]bool{}
	for s.Scan() {
		lineNo++
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		job, err := parseJobLine(absPath, line)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", absPath, lineNo, err)
		}
		if seen[job.Name] {
			return nil, fmt.Errorf("%s:%d: duplicate job %q in file", absPath, lineNo, job.Name)
		}
		seen[job.Name] = true
		out = append(out, job)
	}
	if err := s.Err(); err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no jobs found in %s", absPath)
	}
	return out, nil
}

func parseJobLine(configPath, line string) (JobConfig, error) {
	tokens, err := splitTokens(line)
	if err != nil {
		return JobConfig{}, err
	}

	fieldStart, err := findFieldStart(tokens)
	if err != nil {
		return JobConfig{}, err
	}

	defaults := JobConfig{
		Window:    0,
		Mode:      core.WindowModeAfter,
		Dist:      core.DistributionUniform,
		SkewShape: 0,
		Timezone:  "UTC",
		Seed:      core.SeedStrategyStable,
		Policy: PolicySpec{
			Concurrency: DefaultConcurrency,
			Deadline:    0,
			Suspend:     false,
		},
	}

	if err := parseJobModifiers(&defaults, tokens[5:fieldStart]); err != nil {
		return JobConfig{}, err
	}

	schedule, err := parseCronSpec([5]string{tokens[0], tokens[1], tokens[2], tokens[3], tokens[4]}, defaults.Timezone)
	if err != nil {
		return JobConfig{}, err
	}
	defaults.Schedule = schedule

	if err := parseJobFields(&defaults, tokens[fieldStart:]); err != nil {
		return JobConfig{}, err
	}

	defaults.Identity = configPath + ":" + defaults.Name
	return defaults, nil
}

func findFieldStart(tokens []string) (int, error) {
	for i, tok := range tokens {
		if isFieldToken(tok) {
			if i < 5 {
				return -1, errors.New("invalid cron expression: expected 5 fields before modifiers")
			}
			return i, nil
		}
	}
	return -1, errors.New("missing key=value fields")
}

func parseJobModifiers(job *JobConfig, modifiers []string) error {
	for _, tok := range modifiers {
		name, body, err := splitModifier(tok)
		if err != nil {
			return err
		}
		switch name {
		case "tz":
			tz := strings.TrimSpace(body)
			if _, err := time.LoadLocation(tz); err != nil {
				return fmt.Errorf("invalid timezone %q", tz)
			}
			job.Timezone = tz
		case "win":
			parts := strings.SplitN(body, ",", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid @win arguments %q", body)
			}
			mode, err := mapWindowMode(parts[0])
			if err != nil {
				return err
			}
			dur, err := time.ParseDuration(parts[1])
			if err != nil {
				return fmt.Errorf("invalid @win duration %q", parts[1])
			}
			job.Mode = mode
			job.Window = dur
		case "dist":
			dist, skewShape, err := parseDistModifier(body)
			if err != nil {
				return err
			}
			job.Dist = dist
			job.SkewShape = skewShape
		case "seed":
			strategy, salt, err := parseSeedModifier(body)
			if err != nil {
				return err
			}
			job.Seed = strategy
			job.Salt = salt
		case "policy":
			policy, err := parsePolicyModifier(body)
			if err != nil {
				return err
			}
			job.Policy = policy
		case "only":
			if err := applyConstraintModifier(&job.Constraints, "only", body); err != nil {
				return err
			}
		case "avoid":
			if err := applyConstraintModifier(&job.Constraints, "avoid", body); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown modifier @%s", name)
		}
	}
	return nil
}

func splitModifier(tok string) (string, string, error) {
	if !strings.HasPrefix(tok, "@") {
		return "", "", fmt.Errorf("unexpected token before fields: %q", tok)
	}
	i := strings.IndexByte(tok, '(')
	if i <= 1 || !strings.HasSuffix(tok, ")") {
		return "", "", fmt.Errorf("invalid modifier syntax: %q", tok)
	}

	name := tok[1:i]
	body := tok[i+1 : len(tok)-1]
	if body == "" {
		if name == "avoid" || name == "only" {
			return "", "", fmt.Errorf("%s spec cannot be empty", name)
		}
		return "", "", fmt.Errorf("modifier %q body cannot be empty", name)
	}
	return name, body, nil
}

func parseDistModifier(body string) (core.Distribution, float64, error) {
	parts := strings.Split(body, ",")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return "", 0, errors.New("invalid @dist arguments")
	}

	dist := core.Distribution(strings.TrimSpace(parts[0]))
	switch dist {
	case core.DistributionUniform, core.DistributionSkewEarly, core.DistributionSkewLate:
	default:
		return "", 0, fmt.Errorf("distribution %q is not supported in current krond runtime", parts[0])
	}

	var skewShape float64
	for _, p := range parts[1:] {
		kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
		if len(kv) != 2 || kv[0] == "" || kv[1] == "" {
			return "", 0, fmt.Errorf("invalid distribution parameter %q", p)
		}

		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])

		switch dist {
		case core.DistributionUniform:
			return "", 0, fmt.Errorf("distribution %q does not accept parameters in current krond runtime", dist)
		case core.DistributionSkewEarly, core.DistributionSkewLate:
			if key != "shape" {
				return "", 0, fmt.Errorf("unknown %s parameter %q", dist, key)
			}
			parsed, err := strconv.ParseFloat(val, 64)
			if err != nil || parsed <= 0 {
				return "", 0, fmt.Errorf("invalid shape %q", val)
			}
			skewShape = parsed
		}
	}

	return dist, skewShape, nil
}

func parseSeedModifier(body string) (core.SeedStrategy, string, error) {
	parts := strings.Split(body, ",")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return "", "", errors.New("invalid @seed arguments")
	}

	strategy := core.SeedStrategy(strings.TrimSpace(parts[0]))
	switch strategy {
	case core.SeedStrategyStable, core.SeedStrategyDaily, core.SeedStrategyWeekly:
	default:
		return "", "", fmt.Errorf("invalid seed strategy %q", parts[0])
	}

	salt := ""
	for _, p := range parts[1:] {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 || strings.TrimSpace(kv[0]) == "" {
			return "", "", fmt.Errorf("invalid seed parameter %q", p)
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		if key != "salt" {
			return "", "", fmt.Errorf("unknown seed key %q", key)
		}
		salt = val
	}

	return strategy, salt, nil
}

func parsePolicyModifier(body string) (PolicySpec, error) {
	policy := PolicySpec{
		Concurrency: DefaultConcurrency,
		Deadline:    0,
		Suspend:     false,
	}

	for _, p := range strings.Split(body, ",") {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 || strings.TrimSpace(kv[0]) == "" {
			return PolicySpec{}, fmt.Errorf("invalid policy parameter %q", p)
		}

		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])

		switch key {
		case "concurrency":
			if val != "allow" && val != "forbid" && val != "replace" {
				return PolicySpec{}, fmt.Errorf("invalid concurrency %q", val)
			}
			policy.Concurrency = val
		case "deadline":
			d, err := time.ParseDuration(val)
			if err != nil {
				return PolicySpec{}, fmt.Errorf("invalid deadline %q", val)
			}
			policy.Deadline = d
		case "suspend":
			if val != "true" && val != "false" {
				return PolicySpec{}, fmt.Errorf("invalid suspend %q", val)
			}
			policy.Suspend = val == "true"
		default:
			return PolicySpec{}, fmt.Errorf("unknown policy key %q", key)
		}
	}

	return policy, nil
}

func mapWindowMode(mode string) (core.WindowMode, error) {
	switch mode {
	case "after":
		return core.WindowModeAfter, nil
	case "around", "center":
		return core.WindowModeCenter, nil
	case "before":
		return core.WindowModeBefore, nil
	default:
		return "", fmt.Errorf("invalid @win mode %q", mode)
	}
}

func parseJobFields(job *JobConfig, fields []string) error {
	var hasName, hasCommand bool

	for _, tok := range fields {
		kv := strings.SplitN(tok, "=", 2)
		if len(kv) != 2 {
			return fmt.Errorf("invalid field %q", tok)
		}
		key := kv[0]
		val := kv[1]

		switch key {
		case "name":
			if val == "" {
				return fmt.Errorf("name cannot be empty")
			}
			job.Name = val
			hasName = true
		case "command":
			if strings.TrimSpace(val) == "" {
				return fmt.Errorf("command cannot be empty")
			}
			job.Command.Raw = val
			hasCommand = true
		case "shell":
			if val != "true" && val != "false" {
				return fmt.Errorf("invalid shell value %q", val)
			}
			job.Command.Shell = val == "true"
		case "env":
			parts := strings.SplitN(val, "=", 2)
			if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
				return fmt.Errorf("invalid env value %q", val)
			}
			job.Command.Env = append(job.Command.Env, val)
		case "cwd":
			job.Command.Cwd = val
		case "timeout":
			d, err := time.ParseDuration(val)
			if err != nil {
				return fmt.Errorf("invalid timeout %q", val)
			}
			job.Command.Timeout = d
		case "user", "group", "umask", "stdout", "stderr", "description":
			// Parsed for format compatibility; runtime handling is added incrementally.
			continue
		default:
			return fmt.Errorf("unknown field %q", key)
		}
	}

	if !hasName {
		return fmt.Errorf("missing required field %q", "name")
	}
	if !hasCommand {
		return fmt.Errorf("missing required field %q", "command")
	}
	return nil
}

func isFieldToken(tok string) bool {
	return strings.Contains(tok, "=") && !strings.HasPrefix(tok, "@")
}

func splitTokens(line string) ([]string, error) {
	var tokens []string
	var cur strings.Builder
	inQuote := false

	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case c == '"':
			inQuote = !inQuote
		case inQuote && c == '\\':
			if i+1 >= len(line) {
				return nil, errors.New("invalid escape at end of quoted value")
			}
			n := line[i+1]
			if n == '"' || n == '\\' {
				cur.WriteByte(n)
				i++
			} else {
				cur.WriteByte(c)
			}
		case !inQuote && (c == ' ' || c == '\t'):
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteByte(c)
		}
	}

	if inQuote {
		return nil, errors.New("unterminated quote")
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	if len(tokens) == 0 {
		return nil, errors.New("empty entry")
	}
	return tokens, nil
}
