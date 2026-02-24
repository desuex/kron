package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"kron/core/pkg/core"
)

var errJobNotFound = errors.New("job not found")

type explainSettings struct {
	Window       time.Duration
	Mode         core.WindowMode
	Dist         core.Distribution
	Timezone     string
	SeedStrategy core.SeedStrategy
	Salt         string
	Constraints  core.ConstraintSpec
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

func loadJobDefinition(path, job string, fallback explainSettings) (jobDefinition, error) {
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
			return jobDefinition{}, fmt.Errorf("line %d: %w", lineNo, err)
		}
		if _, errs := validateEntry(tokens); len(errs) > 0 {
			return jobDefinition{}, fmt.Errorf("line %d: %s", lineNo, errs[0])
		}

		fieldStart, err := findFieldStart(tokens)
		if err != nil {
			return jobDefinition{}, fmt.Errorf("line %d: %w", lineNo, err)
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
			return jobDefinition{}, fmt.Errorf("line %d: %w", lineNo, err)
		}

		cronFields := [5]string{tokens[0], tokens[1], tokens[2], tokens[3], tokens[4]}
		sched, err := parseCronSpec(cronFields, parsed.Timezone)
		if err != nil {
			return jobDefinition{}, fmt.Errorf("line %d: %w", lineNo, err)
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

func parseExplainModifiers(modifiers []string, fallback explainSettings) (explainSettings, error) {
	settings := fallback

	for _, tok := range modifiers {
		if strings.HasPrefix(tok, "@tz(") && strings.HasSuffix(tok, ")") {
			tz := strings.TrimSpace(tok[len("@tz(") : len(tok)-1])
			if _, err := time.LoadLocation(tz); err != nil {
				return explainSettings{}, fmt.Errorf("invalid timezone %q", tz)
			}
			settings.Timezone = tz
			continue
		}

		if strings.HasPrefix(tok, "@win(") && strings.HasSuffix(tok, ")") {
			body := tok[len("@win(") : len(tok)-1]
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
			continue
		}

		if strings.HasPrefix(tok, "@dist(") && strings.HasSuffix(tok, ")") {
			body := tok[len("@dist(") : len(tok)-1]
			name := strings.Split(body, ",")[0]
			if name == "" {
				return explainSettings{}, errors.New("invalid @dist arguments")
			}
			if name != string(core.DistributionUniform) &&
				name != string(core.DistributionSkewEarly) &&
				name != string(core.DistributionSkewLate) {
				return explainSettings{}, fmt.Errorf("distribution %q is not supported in MVP explain", name)
			}
			settings.Dist = core.Distribution(name)
		}

		if strings.HasPrefix(tok, "@seed(") && strings.HasSuffix(tok, ")") {
			body := tok[len("@seed(") : len(tok)-1]
			strategy, salt, err := parseSeedModifier(body)
			if err != nil {
				return explainSettings{}, err
			}
			settings.SeedStrategy = strategy
			settings.Salt = salt
			continue
		}

		if strings.HasPrefix(tok, "@only(") && strings.HasSuffix(tok, ")") {
			body := tok[len("@only(") : len(tok)-1]
			if err := applyConstraintModifier(&settings.Constraints, "only", body); err != nil {
				return explainSettings{}, err
			}
			continue
		}

		if strings.HasPrefix(tok, "@avoid(") && strings.HasSuffix(tok, ")") {
			body := tok[len("@avoid(") : len(tok)-1]
			if err := applyConstraintModifier(&settings.Constraints, "avoid", body); err != nil {
				return explainSettings{}, err
			}
			continue
		}
	}

	return settings, nil
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
