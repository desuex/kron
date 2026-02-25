package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"kron/core/pkg/core"
)

var errJobNotFound = errors.New("job not found")

const errLineWrap = "line %d: %w"

type explainSettings struct {
	Window       time.Duration
	Mode         core.WindowMode
	Dist         core.Distribution
	SkewShape    float64
	Timezone     string
	SeedStrategy core.SeedStrategy
	Salt         string
	Constraints  core.ConstraintSpec
	Policy       policySettings
}

type policySettings struct {
	Concurrency string
	Deadline    time.Duration
	Suspend     bool
}

type jobDefinition struct {
	Name     string
	Schedule CronSpec
	Settings explainSettings
}

func loadJobSettings(path, job string, fallback explainSettings) (explainSettings, error) {
	def, err := loadJobDefinition(path, job, fallback)
	if err != nil {
		return explainSettings{}, err
	}
	return def.Settings, nil
}

func loadJobDefinition(path, job string, fallback explainSettings) (jobDefinition, error) { // NOSONAR
	f, err := os.Open(path)
	if err != nil {
		return jobDefinition{}, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	lineNo := 0
	found := false
	var def jobDefinition

	for s.Scan() {
		lineNo++
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		tokens, err := splitTokens(line)
		if err != nil {
			return jobDefinition{}, fmt.Errorf(errLineWrap, lineNo, err)
		}
		if _, errs := validateEntry(tokens); len(errs) > 0 {
			return jobDefinition{}, fmt.Errorf("line %d: %s", lineNo, errs[0])
		}

		fieldStart, err := findFieldStart(tokens)
		if err != nil {
			return jobDefinition{}, fmt.Errorf(errLineWrap, lineNo, err)
		}

		name := extractName(tokens[fieldStart:])
		if name != job {
			continue
		}

		if found {
			return jobDefinition{}, fmt.Errorf("duplicate job %q in file", job)
		}
		found = true

		modifiers := tokens[5:fieldStart]
		parsed, err := parseExplainModifiers(modifiers, fallback)
		if err != nil {
			return jobDefinition{}, fmt.Errorf(errLineWrap, lineNo, err)
		}

		cronFields := [5]string{tokens[0], tokens[1], tokens[2], tokens[3], tokens[4]}
		sched, err := parseCronSpec(cronFields, parsed.Timezone)
		if err != nil {
			return jobDefinition{}, fmt.Errorf(errLineWrap, lineNo, err)
		}

		def = jobDefinition{
			Name:     name,
			Schedule: sched,
			Settings: parsed,
		}
	}

	if err := s.Err(); err != nil {
		return jobDefinition{}, fmt.Errorf("read file: %w", err)
	}
	if !found {
		return jobDefinition{}, errJobNotFound
	}

	return def, nil
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

func extractName(fieldTokens []string) string {
	for _, tok := range fieldTokens {
		kv := strings.SplitN(tok, "=", 2)
		if len(kv) != 2 {
			continue
		}
		if kv[0] == "name" {
			return kv[1]
		}
	}
	return ""
}

func parseExplainModifiers(modifiers []string, fallback explainSettings) (explainSettings, error) { // NOSONAR
	settings := fallback

	for _, tok := range modifiers {
		name, body, err := splitModifier(tok)
		if err != nil {
			return explainSettings{}, err
		}

		switch name {
		case "tz":
			tz := strings.TrimSpace(body)
			if _, err := time.LoadLocation(tz); err != nil {
				return explainSettings{}, fmt.Errorf("invalid timezone %q", tz)
			}
			settings.Timezone = tz
		case "win":
			parts := strings.SplitN(body, ",", 2)
			if len(parts) != 2 {
				return explainSettings{}, fmt.Errorf("invalid @win arguments %q", body)
			}

			mode, err := mapWindowMode(parts[0])
			if err != nil {
				return explainSettings{}, err
			}
			dur, err := time.ParseDuration(parts[1])
			if err != nil {
				return explainSettings{}, fmt.Errorf("invalid @win duration %q", parts[1])
			}
			settings.Mode = mode
			settings.Window = dur
		case "dist":
			dist, skewShape, err := parseDistModifier(body)
			if err != nil {
				return explainSettings{}, err
			}
			settings.Dist = dist
			settings.SkewShape = skewShape
		case "seed":
			strategy, salt, err := parseSeedModifier(body)
			if err != nil {
				return explainSettings{}, err
			}
			settings.SeedStrategy = strategy
			settings.Salt = salt
		case "only":
			if err := applyConstraintModifier(&settings.Constraints, "only", body); err != nil {
				return explainSettings{}, err
			}
		case "avoid":
			if err := applyConstraintModifier(&settings.Constraints, "avoid", body); err != nil {
				return explainSettings{}, err
			}
		case "policy":
			policy, err := parsePolicyModifier(body)
			if err != nil {
				return explainSettings{}, err
			}
			settings.Policy = policy
		default:
			return explainSettings{}, fmt.Errorf("unknown modifier @%s", name)
		}
	}

	return settings, nil
}

func parseDistModifier(body string) (core.Distribution, float64, error) { // NOSONAR
	parts := strings.Split(body, ",")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return "", 0, errors.New("invalid @dist arguments")
	}

	dist := core.Distribution(strings.TrimSpace(parts[0]))
	switch dist {
	case core.DistributionUniform, core.DistributionSkewEarly, core.DistributionSkewLate:
	default:
		return "", 0, fmt.Errorf("distribution %q is not supported in MVP explain", parts[0])
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
			return "", 0, fmt.Errorf("distribution %q does not accept parameters in MVP explain", dist)
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

func parsePolicyModifier(body string) (policySettings, error) { // NOSONAR
	policy := policySettings{
		Concurrency: "forbid",
		Deadline:    0,
		Suspend:     false,
	}

	for _, p := range strings.Split(body, ",") {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 || strings.TrimSpace(kv[0]) == "" {
			return policySettings{}, fmt.Errorf("invalid policy parameter %q", p)
		}

		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])

		switch key {
		case "concurrency":
			if val != "allow" && val != "forbid" && val != "replace" {
				return policySettings{}, fmt.Errorf("invalid concurrency %q", val)
			}
			policy.Concurrency = val
		case "deadline":
			d, err := time.ParseDuration(val)
			if err != nil {
				return policySettings{}, fmt.Errorf("invalid deadline %q", val)
			}
			policy.Deadline = d
		case "suspend":
			if val != "true" && val != "false" {
				return policySettings{}, fmt.Errorf("invalid suspend %q", val)
			}
			policy.Suspend = val == "true"
		default:
			return policySettings{}, fmt.Errorf("unknown policy key %q", key)
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
