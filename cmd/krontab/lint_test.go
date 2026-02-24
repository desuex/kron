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

func TestSplitTokensErrors(t *testing.T) {
	if _, err := splitTokens(`0 0 * * * name=backup command="/bin/echo`); err == nil {
		t.Fatalf("expected unterminated quote error")
	}
	if _, err := splitTokens(`0 0 * * * name=backup command="abc\`); err == nil {
		t.Fatalf("expected invalid escape error")
	}
}

func TestSplitTokensBackslashLiteralInQuotes(t *testing.T) {
	toks, err := splitTokens(`0 0 * * * name=backup command="a\qb"`)
	if err != nil {
		t.Fatalf("splitTokens error: %v", err)
	}
	found := false
	for _, tok := range toks {
		if tok == `command=a\qb` {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected literal backslash token, got %v", toks)
	}
}

func TestValidateModifierMatrix(t *testing.T) {
	tests := []struct {
		token string
		ok    bool
	}{
		{token: "@tz(UTC)", ok: true},
		{token: "@tz(America/New_York)", ok: true},
		{token: "@tz(Bad TZ)", ok: false},
		{token: "@win(after,1h)", ok: true},
		{token: "@win(around,45m)", ok: true},
		{token: "@win(center,45m)", ok: true},
		{token: "@win(before,45m)", ok: true},
		{token: "@win(bad,1h)", ok: false},
		{token: "@win(after,bad)", ok: false},
		{token: "@dist(uniform)", ok: true},
		{token: "@dist(normal,sigma=10m)", ok: true},
		{token: "@dist(normal,sigma=bad)", ok: false},
		{token: "@dist(skewEarly,shape=2)", ok: true},
		{token: "@dist(skewLate,shape=2)", ok: true},
		{token: "@dist(skewLate,shape=bad)", ok: false},
		{token: "@dist(exponential,lambda=1.2,dir=early)", ok: true},
		{token: "@dist(exponential,lambda=0,dir=early)", ok: false},
		{token: "@dist(exponential,lambda=1.2,dir=bad)", ok: false},
		{token: "@dist(unknown)", ok: false},
		{token: "@dist(uniform,bad)", ok: false},
		{token: "@seed(stable)", ok: true},
		{token: "@seed(daily,salt=team)", ok: true},
		{token: "@seed(weekly,salt=team)", ok: true},
		{token: "@seed(bad)", ok: false},
		{token: "@seed(stable,bad)", ok: false},
		{token: "@policy(concurrency=allow,deadline=1m,suspend=false)", ok: true},
		{token: "@policy(concurrency=bad)", ok: false},
		{token: "@policy(deadline=bad)", ok: false},
		{token: "@policy(suspend=bad)", ok: false},
		{token: "@policy(unknown=v)", ok: false},
		{token: "@policy(bad)", ok: false},
		{token: "@avoid(dow=MON-FRI)", ok: true},
		{token: "@only(hours=8-17)", ok: true},
		{token: "@avoid( )", ok: false},
		{token: "@only( )", ok: false},
		{token: "@bad(x)", ok: false},
		{token: "naked", ok: false},
		{token: "@dist", ok: false},
		{token: "@dist()", ok: false},
	}

	for _, tt := range tests {
		err := validateModifier(tt.token)
		if tt.ok && err != nil {
			t.Fatalf("token %q expected ok, got error: %v", tt.token, err)
		}
		if !tt.ok && err == nil {
			t.Fatalf("token %q expected error", tt.token)
		}
	}
}

func TestValidateFieldsMatrix(t *testing.T) {
	_, errs := validateFields([]string{"name=backup", "command=/bin/echo"})
	if len(errs) != 0 {
		t.Fatalf("expected valid fields, got %v", errs)
	}

	_, errs = validateFields([]string{"name=backup"})
	if len(errs) == 0 {
		t.Fatalf("expected missing command error")
	}

	_, errs = validateFields([]string{"command=/bin/echo"})
	if len(errs) == 0 {
		t.Fatalf("expected missing name error")
	}

	_, errs = validateFields([]string{"name=Bad_Name", "command=/bin/echo"})
	if len(errs) == 0 {
		t.Fatalf("expected invalid name error")
	}

	_, errs = validateFields([]string{"name=backup", "command=/bin/echo", "shell=maybe"})
	if len(errs) == 0 {
		t.Fatalf("expected invalid shell error")
	}

	_, errs = validateFields([]string{"name=backup", "command=/bin/echo", "umask=98"})
	if len(errs) == 0 {
		t.Fatalf("expected invalid umask error")
	}

	_, errs = validateFields([]string{"name=backup", "command=/bin/echo", "timeout=bad"})
	if len(errs) == 0 {
		t.Fatalf("expected invalid timeout error")
	}

	_, errs = validateFields([]string{"name=backup", "command=/bin/echo", "stdout=file:"})
	if len(errs) == 0 {
		t.Fatalf("expected invalid stdout file path error")
	}

	_, errs = validateFields([]string{"name=backup", "command=/bin/echo", "stderr=weird"})
	if len(errs) == 0 {
		t.Fatalf("expected invalid stderr error")
	}

	_, errs = validateFields([]string{"name=backup", "command=/bin/echo", "env=BAD"})
	if len(errs) == 0 {
		t.Fatalf("expected invalid env error")
	}

	_, errs = validateFields([]string{"name=backup", "command=/bin/echo", "unknown=v"})
	if len(errs) == 0 {
		t.Fatalf("expected unknown field error")
	}

	_, errs = validateFields([]string{"name=backup", "name=backup2", "command=/bin/echo"})
	if len(errs) == 0 {
		t.Fatalf("expected duplicate field error")
	}
}
