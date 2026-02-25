package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const stateVersion = "1"

type StateStore interface {
	Load(identity string) (JobState, error)
	Save(state JobState) error
}

type FileStateStore struct {
	Dir string
}

func (s FileStateStore) Load(identity string) (JobState, error) {
	if identity == "" {
		return JobState{}, errors.New("identity is required")
	}
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return JobState{}, fmt.Errorf("create state dir: %w", err)
	}

	path := s.statePath(identity)
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return JobState{
				Version:  stateVersion,
				Identity: identity,
			}, nil
		}
		return JobState{}, fmt.Errorf("read state: %w", err)
	}

	var out JobState
	if err := json.Unmarshal(raw, &out); err != nil {
		return JobState{}, fmt.Errorf("decode state: %w", err)
	}
	if out.Identity == "" {
		out.Identity = identity
	}
	if out.Version == "" {
		out.Version = stateVersion
	}
	return out, nil
}

func (s FileStateStore) Save(state JobState) error {
	if state.Identity == "" {
		return errors.New("identity is required")
	}
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	if state.Version == "" {
		state.Version = stateVersion
	}
	path := s.statePath(state.Identity)
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-state-*.json")
	if err != nil {
		return fmt.Errorf("create temp state file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(state); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("encode state: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("fsync temp state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp state: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename state file: %w", err)
	}
	return nil
}

func (s FileStateStore) statePath(identity string) string {
	sum := sha256.Sum256([]byte(identity))
	return filepath.Join(s.Dir, hex.EncodeToString(sum[:])+".json")
}
