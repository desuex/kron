package daemon

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"kron/core/pkg/core"
)

var (
	envAssignmentPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*\s*=`)
	cronEntryPattern     = regexp.MustCompile(`^(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(.+)$`)
	cronMacroPattern     = regexp.MustCompile(`^(@[A-Za-z]+)\s+(\S+)\s+(.+)$`)
)

var cronMacros = map[string][5]string{
	"@yearly":   {"0", "0", "1", "1", "*"},
	"@annually": {"0", "0", "1", "1", "*"},
	"@monthly":  {"0", "0", "1", "*", "*"},
	"@weekly":   {"0", "0", "*", "*", "0"},
	"@daily":    {"0", "0", "*", "*", "*"},
	"@midnight": {"0", "0", "*", "*", "*"},
	"@hourly":   {"0", "*", "*", "*", "*"},
}

// LoadSystemCron loads /etc/crontab or /etc/cron.d style sources.
// File input is parsed as one cron source; directory input is parsed as cron.d entries.
func LoadSystemCron(path string) ([]JobConfig, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat config path: %w", err)
	}

	if info.IsDir() {
		return loadSystemCronDir(path)
	}
	return loadSystemCronFile(path)
}

func loadSystemCronDir(path string) ([]JobConfig, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read cron dir: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	out := make([]JobConfig, 0)
	seen := map[string]bool{}
	for _, ent := range entries {
		if ent.IsDir() || strings.HasPrefix(ent.Name(), ".") {
			continue
		}

		filePath := filepath.Join(path, ent.Name())
		jobs, err := loadSystemCronFile(filePath)
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

func loadSystemCronFile(path string) ([]JobConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}

	env := newOrderedEnv()
	scanner := bufio.NewScanner(f)
	lineNo := 0
	out := make([]JobConfig, 0)
	seen := map[string]bool{}

	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if isEnvAssignmentLine(line) {
			key, value, err := parseEnvAssignment(line)
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %w", absPath, lineNo, err)
			}
			env.Set(key, value)
			continue
		}

		job, err := parseSystemCronLine(absPath, lineNo, line, env)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", absPath, lineNo, err)
		}
		if seen[job.Identity] {
			return nil, fmt.Errorf("%s:%d: duplicate job identity %q", absPath, lineNo, job.Identity)
		}
		seen[job.Identity] = true
		out = append(out, job)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no jobs found in %s", absPath)
	}
	return out, nil
}

func parseSystemCronLine(configPath string, lineNo int, line string, env *orderedEnv) (JobConfig, error) {
	fields, user, command, err := parseSystemCronEntry(line)
	if err != nil {
		return JobConfig{}, err
	}
	if user == "" {
		return JobConfig{}, fmt.Errorf("user field cannot be empty")
	}
	if strings.TrimSpace(command) == "" {
		return JobConfig{}, fmt.Errorf("command cannot be empty")
	}

	tz := env.Timezone()
	schedule, err := parseCronSpec(fields, tz)
	if err != nil {
		return JobConfig{}, err
	}

	name := fmt.Sprintf("cron/%s/%d/%s", sanitizeJobPart(filepath.Base(configPath)), lineNo, sanitizeJobPart(user))
	identity := fmt.Sprintf("%s:%d:%s", configPath, lineNo, user)

	return JobConfig{
		Identity:  identity,
		Name:      name,
		Schedule:  schedule,
		Command:   CommandSpec{Raw: command, Shell: true, Env: env.Snapshot(), User: user},
		Window:    0,
		Mode:      core.WindowModeAfter,
		Dist:      core.DistributionUniform,
		SkewShape: 0,
		Timezone:  tz,
		Seed:      core.SeedStrategyStable,
		Policy: PolicySpec{
			Concurrency: DefaultConcurrency,
			Deadline:    0,
			Suspend:     false,
		},
	}, nil
}

func parseSystemCronEntry(line string) ([5]string, string, string, error) {
	if parts := cronMacroPattern.FindStringSubmatch(line); len(parts) == 4 {
		macro := strings.ToLower(parts[1])
		fields, ok := cronMacros[macro]
		if !ok {
			if macro == "@reboot" {
				return [5]string{}, "", "", fmt.Errorf("@reboot is not supported in current krond runtime")
			}
			return [5]string{}, "", "", fmt.Errorf("unsupported cron macro %q", parts[1])
		}
		return fields, parts[2], parts[3], nil
	}

	parts := cronEntryPattern.FindStringSubmatch(line)
	if len(parts) != 8 {
		return [5]string{}, "", "", fmt.Errorf("invalid cron entry, expected 5 fields + user + command")
	}

	return [5]string{parts[1], parts[2], parts[3], parts[4], parts[5]}, parts[6], parts[7], nil
}

func isEnvAssignmentLine(line string) bool {
	return envAssignmentPattern.MatchString(line)
}

func parseEnvAssignment(line string) (string, string, error) {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid env assignment %q", line)
	}

	key := strings.TrimSpace(parts[0])
	if key == "" {
		return "", "", fmt.Errorf("env key cannot be empty")
	}
	value := strings.TrimSpace(parts[1])
	if len(value) >= 2 {
		first := value[0]
		last := value[len(value)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			value = value[1 : len(value)-1]
		}
	}
	return key, value, nil
}

func sanitizeJobPart(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return "job"
	}

	var b strings.Builder
	b.Grow(len(s))
	lastDash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}

	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "job"
	}
	return out
}

type orderedEnv struct {
	order  []string
	values map[string]string
}

func newOrderedEnv() *orderedEnv {
	return &orderedEnv{
		order:  make([]string, 0),
		values: map[string]string{},
	}
}

func (e *orderedEnv) Set(key, value string) {
	if _, exists := e.values[key]; !exists {
		e.order = append(e.order, key)
	}
	e.values[key] = value
}

func (e *orderedEnv) Snapshot() []string {
	out := make([]string, 0, len(e.order))
	for _, key := range e.order {
		out = append(out, key+"="+e.values[key])
	}
	return out
}

func (e *orderedEnv) Timezone() string {
	if v, ok := e.values["CRON_TZ"]; ok {
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	if v, ok := e.values["TZ"]; ok {
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	return time.UTC.String()
}
