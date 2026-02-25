package daemon

import (
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
