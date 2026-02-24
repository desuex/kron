package main

import (
	"strings"
	"testing"
)

func TestLintReaderValid(t *testing.T) {
	content := `
# backup job
0 0 * * * @win(after,2h) @dist(uniform) name=backup command=/usr/bin/backup timeout=10m
`
	errs, err := lintReader(strings.NewReader(content))
	if err != nil {
		t.Fatalf("lintReader error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("expected no lint errors, got: %v", errs)
	}
}

func TestLintReaderInvalid(t *testing.T) {
	content := `
0 0 * * * @dist(unknown) name=Bad_Name command=
`
	errs, err := lintReader(strings.NewReader(content))
	if err != nil {
		t.Fatalf("lintReader error: %v", err)
	}
	if len(errs) == 0 {
		t.Fatalf("expected lint errors")
	}
}

func TestLintReaderDuplicateNames(t *testing.T) {
	content := `
0 0 * * * name=backup command=/usr/bin/backup
5 0 * * * name=backup command=/usr/bin/backup2
`
	errs, err := lintReader(strings.NewReader(content))
	if err != nil {
		t.Fatalf("lintReader error: %v", err)
	}
	if len(errs) == 0 {
		t.Fatalf("expected duplicate-name lint error")
	}

	found := false
	for _, e := range errs {
		if strings.Contains(e, "duplicate name") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected duplicate-name error, got: %v", errs)
	}
}

func TestSplitTokensQuotedCommand(t *testing.T) {
	line := `0 0 * * * name=backup command="/usr/bin/backup --full" env=MODE=prod`
	toks, err := splitTokens(line)
	if err != nil {
		t.Fatalf("splitTokens error: %v", err)
	}
	if len(toks) == 0 {
		t.Fatalf("expected tokens")
	}
	want := "command=/usr/bin/backup --full"
	found := false
	for _, tok := range toks {
		if tok == want {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected token %q in %v", want, toks)
	}
}
