package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type lintResult struct {
	File   string   `json:"file"`
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors"`
}

var (
	namePattern    = regexp.MustCompile(`^[a-z0-9/-]+$`)
	octalPattern   = regexp.MustCompile(`^[0-7]{3,4}$`)
	cronAtom       = regexp.MustCompile(`^[A-Za-z0-9*/,\-]+$`)
	allowedFields  = map[string]bool{"name": true, "command": true, "user": true, "group": true, "cwd": true, "env": true, "shell": true, "umask": true, "timeout": true, "stdout": true, "stderr": true, "description": true}
	singletonField = map[string]bool{"name": true, "command": true, "user": true, "group": true, "cwd": true, "shell": true, "umask": true, "timeout": true, "stdout": true, "stderr": true, "description": true}
)

func runLint(args []string) int {
	fs := flag.NewFlagSet("lint", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	file := fs.String("file", "", "path to krontab file")
	format := fs.String("format", "text", "output format: text|json")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "error: lint does not accept positional arguments")
		return 2
	}
	if *file == "" {
		fmt.Fprintln(os.Stderr, "error: --file is required for MVP")
		return 2
	}

	abs, err := filepath.Abs(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: resolve file path:", err)
		return 2
	}

	result, err := lintFile(abs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 2
	}

	switch strings.ToLower(*format) {
	case "text":
		printLintText(result)
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			fmt.Fprintln(os.Stderr, "error: encode json:", err)
			return 2
		}
	default:
		fmt.Fprintln(os.Stderr, "error: invalid --format value:", *format)
		return 2
	}

	if result.Valid {
		return 0
	}
	return 1
}

func printLintText(res lintResult) {
	if res.Valid {
		fmt.Printf("OK: %s\n", res.File)
		return
	}
	fmt.Printf("INVALID: %s\n", res.File)
	for _, e := range res.Errors {
		fmt.Printf("- %s\n", e)
	}
}

func lintFile(path string) (lintResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return lintResult{}, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	errs, err := lintReader(f)
	if err != nil {
		return lintResult{}, err
	}

	return lintResult{File: path, Valid: len(errs) == 0, Errors: errs}, nil
}

func lintReader(r io.Reader) ([]string, error) {
	s := bufio.NewScanner(r)
	lineNo := 0
	seenNames := map[string]int{}
	errs := make([]string, 0)

	for s.Scan() {
		lineNo++
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		tokens, err := splitTokens(line)
		if err != nil {
			errs = append(errs, fmt.Sprintf("line %d: %v", lineNo, err))
			continue
		}

		name, lineErrs := validateEntry(tokens)
		for _, e := range lineErrs {
			errs = append(errs, fmt.Sprintf("line %d: %s", lineNo, e))
		}

		if name != "" {
			if prev, ok := seenNames[name]; ok {
				errs = append(errs, fmt.Sprintf("line %d: duplicate name %q (already used on line %d)", lineNo, name, prev))
			} else {
				seenNames[name] = lineNo
			}
		}
	}

	if err := s.Err(); err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	return errs, nil
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

func validateEntry(tokens []string) (string, []string) {
	var errs []string

	fieldStart := -1
	for i, tok := range tokens {
		if isFieldToken(tok) {
			fieldStart = i
			break
		}
	}
	if fieldStart < 0 {
		return "", []string{"missing key=value fields"}
	}
	if fieldStart < 5 {
		return "", []string{"invalid cron expression: expected 5 fields before modifiers"}
	}

	for i := 0; i < 5; i++ {
		tok := tokens[i]
		if strings.HasPrefix(tok, "@") || strings.Contains(tok, "=") || !cronAtom.MatchString(tok) {
			errs = append(errs, fmt.Sprintf("invalid cron field %q", tok))
		}
	}

	for _, mod := range tokens[5:fieldStart] {
		if err := validateModifier(mod); err != nil {
			errs = append(errs, err.Error())
		}
	}

	name, fieldErrs := validateFields(tokens[fieldStart:])
	errs = append(errs, fieldErrs...)
	return name, errs
}

func isFieldToken(tok string) bool {
	return strings.Contains(tok, "=") && !strings.HasPrefix(tok, "@")
}

func validateModifier(tok string) error {
	if !strings.HasPrefix(tok, "@") {
		return fmt.Errorf("unexpected token before fields: %q", tok)
	}
	i := strings.IndexByte(tok, '(')
	if i <= 1 || !strings.HasSuffix(tok, ")") {
		return fmt.Errorf("invalid modifier syntax: %q", tok)
	}
	name := tok[1:i]
	body := tok[i+1 : len(tok)-1]
	if body == "" {
		if name == "avoid" || name == "only" {
			return fmt.Errorf("%s spec cannot be empty", name)
		}
		return fmt.Errorf("modifier %q body cannot be empty", name)
	}

	switch name {
	case "tz":
		if _, err := time.LoadLocation(body); err != nil {
			return fmt.Errorf("invalid timezone %q", body)
		}
		return nil
	case "win":
		parts := strings.SplitN(body, ",", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid @win arguments %q", body)
		}
		mode := parts[0]
		dur := parts[1]
		if mode != "after" && mode != "around" && mode != "before" && mode != "center" {
			return fmt.Errorf("invalid @win mode %q", mode)
		}
		if _, err := time.ParseDuration(dur); err != nil {
			return fmt.Errorf("invalid @win duration %q", dur)
		}
		return nil
	case "dist":
		parts := strings.Split(body, ",")
		if len(parts) == 0 || parts[0] == "" {
			return fmt.Errorf("invalid @dist arguments %q", body)
		}
		name := parts[0]
		switch name {
		case "uniform", "normal", "skewEarly", "skewLate", "exponential":
		default:
			return fmt.Errorf("unknown distribution %q", name)
		}
		for _, p := range parts[1:] {
			kv := strings.SplitN(p, "=", 2)
			if len(kv) != 2 || kv[0] == "" || kv[1] == "" {
				return fmt.Errorf("invalid distribution parameter %q", p)
			}
			k := kv[0]
			v := kv[1]
			switch name {
			case "uniform":
				return fmt.Errorf("distribution %q does not accept parameters", name)
			case "normal":
				switch k {
				case "sigma":
					if _, err := time.ParseDuration(v); err != nil {
						return fmt.Errorf("invalid normal sigma %q", v)
					}
				case "mu":
					if v != "nominal" && v != "start" && v != "mid" && v != "end" {
						return fmt.Errorf("invalid normal mu %q", v)
					}
				default:
					return fmt.Errorf("unknown normal parameter %q", k)
				}
			case "skewEarly", "skewLate":
				if k != "shape" {
					return fmt.Errorf("unknown %s parameter %q", name, k)
				}
				f, err := strconv.ParseFloat(v, 64)
				if err != nil || f <= 0 {
					return fmt.Errorf("invalid shape %q", v)
				}
			case "exponential":
				switch k {
				case "lambda":
					f, err := strconv.ParseFloat(v, 64)
					if err != nil || f <= 0 {
						return fmt.Errorf("invalid lambda %q", v)
					}
				case "dir":
					if v != "early" && v != "late" {
						return fmt.Errorf("invalid exponential dir %q", v)
					}
				default:
					return fmt.Errorf("unknown exponential parameter %q", k)
				}
			}
		}
		return nil
	case "seed":
		parts := strings.Split(body, ",")
		if len(parts) == 0 {
			return fmt.Errorf("invalid @seed arguments %q", body)
		}
		strategy := parts[0]
		if strategy != "stable" && strategy != "daily" && strategy != "weekly" {
			return fmt.Errorf("invalid seed strategy %q", strategy)
		}
		for _, p := range parts[1:] {
			kv := strings.SplitN(p, "=", 2)
			if len(kv) != 2 || kv[0] == "" {
				return fmt.Errorf("invalid seed parameter %q", p)
			}
			if kv[0] != "salt" {
				return fmt.Errorf("unknown seed key %q", kv[0])
			}
		}
		return nil
	case "policy":
		for _, p := range strings.Split(body, ",") {
			kv := strings.SplitN(p, "=", 2)
			if len(kv) != 2 || kv[0] == "" {
				return fmt.Errorf("invalid policy parameter %q", p)
			}
			k := kv[0]
			v := kv[1]
			switch k {
			case "concurrency":
				if v != "allow" && v != "forbid" && v != "replace" {
					return fmt.Errorf("invalid concurrency %q", v)
				}
			case "deadline":
				if _, err := time.ParseDuration(v); err != nil {
					return fmt.Errorf("invalid deadline %q", v)
				}
			case "suspend":
				if v != "true" && v != "false" {
					return fmt.Errorf("invalid suspend %q", v)
				}
			default:
				return fmt.Errorf("unknown policy key %q", k)
			}
		}
		return nil
	case "avoid", "only":
		if _, err := parseConstraintSpec(body); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("unknown modifier @%s", name)
	}
}

func validateFields(tokens []string) (string, []string) {
	var errs []string
	seen := map[string]int{}
	name := ""
	hasCommand := false

	for _, tok := range tokens {
		kv := strings.SplitN(tok, "=", 2)
		if len(kv) != 2 {
			errs = append(errs, fmt.Sprintf("invalid field %q", tok))
			continue
		}
		k := kv[0]
		v := kv[1]

		if !allowedFields[k] {
			errs = append(errs, fmt.Sprintf("unknown field %q", k))
			continue
		}
		seen[k]++
		if singletonField[k] && seen[k] > 1 {
			errs = append(errs, fmt.Sprintf("duplicate field %q", k))
		}

		switch k {
		case "name":
			if v == "" {
				errs = append(errs, "name cannot be empty")
				continue
			}
			if !namePattern.MatchString(v) {
				errs = append(errs, fmt.Sprintf("invalid name %q", v))
			}
			name = v
		case "command":
			hasCommand = true
			if strings.TrimSpace(v) == "" {
				errs = append(errs, "command cannot be empty")
			}
		case "env":
			parts := strings.SplitN(v, "=", 2)
			if len(parts) != 2 || parts[0] == "" {
				errs = append(errs, fmt.Sprintf("invalid env value %q", v))
			}
		case "shell":
			if v != "true" && v != "false" {
				errs = append(errs, fmt.Sprintf("invalid shell value %q", v))
			}
		case "umask":
			if !octalPattern.MatchString(v) {
				errs = append(errs, fmt.Sprintf("invalid umask %q", v))
			}
		case "timeout":
			if _, err := time.ParseDuration(v); err != nil {
				errs = append(errs, fmt.Sprintf("invalid timeout %q", v))
			}
		case "stdout", "stderr":
			if v != "inherit" && v != "discard" && v != "syslog" && !strings.HasPrefix(v, "file:") {
				errs = append(errs, fmt.Sprintf("invalid %s value %q", k, v))
			}
			if strings.HasPrefix(v, "file:") && len(v) == len("file:") {
				errs = append(errs, fmt.Sprintf("invalid %s file path", k))
			}
		}
	}

	if name == "" {
		errs = append(errs, "missing required field \"name\"")
	}
	if !hasCommand {
		errs = append(errs, "missing required field \"command\"")
	}
	return name, errs
}
