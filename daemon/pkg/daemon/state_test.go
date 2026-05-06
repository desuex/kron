package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileStateStoreLoadDefaultAndSaveRoundTrip(t *testing.T) {
	store := FileStateStore{Dir: t.TempDir()}
	const identity = "/etc/krond.d/jobs.kron:backup"

	initial, err := store.Load(identity)
	if err != nil {
		t.Fatalf("Load default error: %v", err)
	}
	if initial.Identity != identity || initial.Version == "" {
		t.Fatalf("default state mismatch: %+v", initial)
	}

	initial.LastHandledPeriodID = "2026-03-01T00:00:00Z"
	initial.LastOutcome = OutcomeExecuted
	initial.LastChosenTime = "2026-03-01T00:10:00Z"
	initial.LastNominalTime = "2026-03-01T00:00:00Z"
	initial.ActiveExecution = &ActiveExecutionState{
		PeriodID:    "2026-03-01T00:01:00Z",
		PID:         4242,
		StartedAt:   "2026-03-01T00:01:05Z",
		ChosenTime:  "2026-03-01T00:01:00Z",
		NominalTime: "2026-03-01T00:01:00Z",
	}

	if err := store.Save(initial); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	loaded, err := store.Load(identity)
	if err != nil {
		t.Fatalf("Load roundtrip error: %v", err)
	}
	if loaded.LastHandledPeriodID != initial.LastHandledPeriodID ||
		loaded.LastOutcome != initial.LastOutcome ||
		loaded.LastChosenTime != initial.LastChosenTime ||
		loaded.LastNominalTime != initial.LastNominalTime {
		t.Fatalf("roundtrip mismatch: got %+v want %+v", loaded, initial)
	}
	if loaded.ActiveExecution == nil || loaded.ActiveExecution.PeriodID != initial.ActiveExecution.PeriodID {
		t.Fatalf("active execution mismatch: got %+v want %+v", loaded.ActiveExecution, initial.ActiveExecution)
	}
}

func TestFileStateStoreValidatesIdentity(t *testing.T) {
	store := FileStateStore{Dir: t.TempDir()}
	_, err := store.Load("")
	if err == nil || !strings.Contains(err.Error(), "identity is required") {
		t.Fatalf("expected identity error on load, got %v", err)
	}
	err = store.Save(JobState{})
	if err == nil || !strings.Contains(err.Error(), "identity is required") {
		t.Fatalf("expected identity error on save, got %v", err)
	}
}

func TestFileStateStoreLoadDecodeError(t *testing.T) {
	store := FileStateStore{Dir: t.TempDir()}
	const identity = "/etc/krond.d/jobs.kron:broken"

	path := store.statePath(identity)
	if err := os.WriteFile(path, []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("write invalid state: %v", err)
	}

	got, err := store.Load(identity)
	if err != nil {
		t.Fatalf("expected corrupt state recovery, got %v", err)
	}
	if got.Identity != identity || got.Version != stateVersion {
		t.Fatalf("expected default state after corrupt load, got %+v", got)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected corrupt state file to move aside, stat err=%v", err)
	}
	matches, err := filepath.Glob(path + ".corrupt.*")
	if err != nil {
		t.Fatalf("glob corrupt files: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one quarantined state file, got %v", matches)
	}
}

func TestFileStateStoreLoadCorruptStateRenameFailure(t *testing.T) {
	store := FileStateStore{Dir: t.TempDir()}
	const identity = "/etc/krond.d/jobs.kron:broken-rename"

	path := store.statePath(identity)
	if err := os.WriteFile(path, []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("write invalid state: %v", err)
	}

	oldRename := stateRenameFile
	oldNow := stateCorruptSuffixNanos
	t.Cleanup(func() {
		stateRenameFile = oldRename
		stateCorruptSuffixNanos = oldNow
	})

	stateCorruptSuffixNanos = func() int64 { return 123 }
	stateRenameFile = func(_, _ string) error { return os.ErrPermission }

	_, err := store.Load(identity)
	if err == nil || !strings.Contains(err.Error(), "quarantine corrupt state") {
		t.Fatalf("expected corrupt quarantine error, got %v", err)
	}
}

func TestFileStateStoreLoadBackfillsIdentityAndVersion(t *testing.T) {
	store := FileStateStore{Dir: t.TempDir()}
	const identity = "/etc/krond.d/jobs.kron:legacy"

	path := store.statePath(identity)
	raw := `{"lastOutcome":"executed","lastHandledPeriodId":"2026-03-01T00:00:00Z"}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write legacy state: %v", err)
	}

	got, err := store.Load(identity)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if got.Identity != identity {
		t.Fatalf("expected identity backfill %q, got %q", identity, got.Identity)
	}
	if got.Version != stateVersion {
		t.Fatalf("expected version backfill %q, got %q", stateVersion, got.Version)
	}
}

func TestFileStateStoreMkdirErrors(t *testing.T) {
	tmp := t.TempDir()
	fileDir := filepath.Join(tmp, "not-a-dir")
	if err := os.WriteFile(fileDir, []byte("x"), 0o644); err != nil {
		t.Fatalf("write sentinel file: %v", err)
	}

	store := FileStateStore{Dir: fileDir}
	_, err := store.Load("/etc/krond.d/jobs.kron:load")
	if err == nil || !strings.Contains(err.Error(), "create state dir") {
		t.Fatalf("expected create state dir error on load, got %v", err)
	}

	err = store.Save(JobState{Identity: "/etc/krond.d/jobs.kron:save"})
	if err == nil || !strings.Contains(err.Error(), "create state dir") {
		t.Fatalf("expected create state dir error on save, got %v", err)
	}
}

func TestFileStateStoreSaveSetsDefaultVersion(t *testing.T) {
	store := FileStateStore{Dir: t.TempDir()}
	const identity = "/etc/krond.d/jobs.kron:noversion"

	if err := store.Save(JobState{Identity: identity}); err != nil {
		t.Fatalf("Save error: %v", err)
	}
	got, err := store.Load(identity)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if got.Version != stateVersion {
		t.Fatalf("expected default version %q, got %q", stateVersion, got.Version)
	}
}

func TestFileStateStoreSaveRenameError(t *testing.T) {
	store := FileStateStore{Dir: t.TempDir()}
	const identity = "/etc/krond.d/jobs.kron:rename-fail"

	target := store.statePath(identity)
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("mkdir target path: %v", err)
	}

	err := store.Save(JobState{Identity: identity, Version: stateVersion})
	if err == nil || !strings.Contains(err.Error(), "rename state file") {
		t.Fatalf("expected rename state file error, got %v", err)
	}
}

func TestFileStateStoreLoadReadError(t *testing.T) {
	store := FileStateStore{Dir: t.TempDir()}
	const identity = "/etc/krond.d/jobs.kron:read-error"

	path := store.statePath(identity)
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("mkdir state path: %v", err)
	}

	_, err := store.Load(identity)
	if err == nil || !strings.Contains(err.Error(), "read state") {
		t.Fatalf("expected read state error, got %v", err)
	}
}
