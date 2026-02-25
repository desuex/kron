package main

import (
	"os"
	"path/filepath"
	"testing"
)

const updateSnapshotsEnv = "UPDATE_SNAPSHOTS"

const (
	snapArgFile   = "--file"
	snapArgFormat = "--format"
)

func TestExplainOutputSnapshots(t *testing.T) {
	cfg := writeTempKrontab(t, `
0 10 * * * @tz(UTC) @win(around,30m) @dist(skewLate,shape=2.5) @seed(daily,salt=msgs) @only(hours=10;dow=TUE) name=backup command=/usr/bin/backup
`)

	textOut, _ := captureOutput(t, func() {
		err := runExplain([]string{"backup", snapArgFile, cfg, "--at", "2026-02-24T10:00:00Z", snapArgFormat, "text"})
		if err != nil {
			t.Fatalf("runExplain text error: %v", err)
		}
	})
	assertSnapshot(t, "explain_text.txt", textOut)

	jsonOut, _ := captureOutput(t, func() {
		err := runExplain([]string{"backup", snapArgFile, cfg, "--at", "2026-02-24T10:00:00Z", snapArgFormat, "json"})
		if err != nil {
			t.Fatalf("runExplain json error: %v", err)
		}
	})
	assertSnapshot(t, "explain_json.json", jsonOut)
}

func TestNextOutputSnapshots(t *testing.T) {
	cfg := writeTempKrontab(t, `
*/30 * * * * @tz(UTC) @win(after,10m) @dist(uniform) @seed(stable,salt=ops) name=backup command=/usr/bin/backup
`)

	textOut, _ := captureOutput(t, func() {
		err := runNext([]string{"backup", snapArgFile, cfg, "--count", "3", "--at", "2026-02-24T10:07:00Z", snapArgFormat, "text"})
		if err != nil {
			t.Fatalf("runNext text error: %v", err)
		}
	})
	assertSnapshot(t, "next_text.txt", textOut)

	jsonOut, _ := captureOutput(t, func() {
		err := runNext([]string{"backup", snapArgFile, cfg, "--count", "3", "--at", "2026-02-24T10:07:00Z", snapArgFormat, "json"})
		if err != nil {
			t.Fatalf("runNext json error: %v", err)
		}
	})
	assertSnapshot(t, "next_json.json", jsonOut)
}

func assertSnapshot(t *testing.T, name, got string) {
	t.Helper()

	path := filepath.Join("testdata", "snapshots", name)
	if os.Getenv(updateSnapshotsEnv) == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir snapshots dir: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write snapshot %s: %v", name, err)
		}
	}

	wantBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read snapshot %s: %v (run with %s=1 to create)", name, err, updateSnapshotsEnv)
	}
	want := string(wantBytes)
	if got != want {
		t.Fatalf("snapshot mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}
