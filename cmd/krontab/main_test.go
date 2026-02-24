package main

import (
	"reflect"
	"testing"
)

func TestNormalizeExplainArgsJobFirst(t *testing.T) {
	job, args, err := normalizeExplainArgs([]string{
		"backup",
		"--file", "/tmp/jobs.kron",
		"--at", "2026-02-24T10:00:00Z",
	})
	if err != nil {
		t.Fatalf("normalizeExplainArgs error: %v", err)
	}
	if job != "backup" {
		t.Fatalf("job mismatch: got %q want %q", job, "backup")
	}
	want := []string{"--file", "/tmp/jobs.kron", "--at", "2026-02-24T10:00:00Z"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args mismatch: got %v want %v", args, want)
	}
}

func TestNormalizeExplainArgsFlagsFirst(t *testing.T) {
	job, args, err := normalizeExplainArgs([]string{
		"--file", "/tmp/jobs.kron",
		"--at", "2026-02-24T10:00:00Z",
		"backup",
	})
	if err != nil {
		t.Fatalf("normalizeExplainArgs error: %v", err)
	}
	if job != "backup" {
		t.Fatalf("job mismatch: got %q want %q", job, "backup")
	}
	want := []string{"--file", "/tmp/jobs.kron", "--at", "2026-02-24T10:00:00Z"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args mismatch: got %v want %v", args, want)
	}
}

func TestNormalizeExplainArgsRejectsMultipleJobs(t *testing.T) {
	_, _, err := normalizeExplainArgs([]string{"backup", "other", "--at", "2026-02-24T10:00:00Z"})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestNormalizeNextArgsJobFirst(t *testing.T) {
	job, args, err := normalizeNextArgs([]string{
		"backup",
		"--file", "/tmp/jobs.kron",
		"--count", "3",
	})
	if err != nil {
		t.Fatalf("normalizeNextArgs error: %v", err)
	}
	if job != "backup" {
		t.Fatalf("job mismatch: got %q want %q", job, "backup")
	}
	want := []string{"--file", "/tmp/jobs.kron", "--count", "3"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args mismatch: got %v want %v", args, want)
	}
}

func TestNormalizeNextArgsFlagsFirstAndErrors(t *testing.T) {
	job, args, err := normalizeNextArgs([]string{
		"--file", "/tmp/jobs.kron",
		"--count", "2",
		"backup",
	})
	if err != nil {
		t.Fatalf("normalizeNextArgs error: %v", err)
	}
	if job != "backup" {
		t.Fatalf("job mismatch: got %q want %q", job, "backup")
	}
	want := []string{"--file", "/tmp/jobs.kron", "--count", "2"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args mismatch: got %v want %v", args, want)
	}

	if _, _, err := normalizeNextArgs([]string{"--file"}); err == nil {
		t.Fatalf("expected missing flag argument error")
	}
	if _, _, err := normalizeNextArgs([]string{"backup", "other"}); err == nil {
		t.Fatalf("expected multiple job error")
	}
	if _, _, err := normalizeNextArgs([]string{"--file", "/tmp/jobs.kron"}); err == nil {
		t.Fatalf("expected missing job error")
	}
}

func TestNormalizeExplainArgsErrors(t *testing.T) {
	if _, _, err := normalizeExplainArgs([]string{"--at"}); err == nil {
		t.Fatalf("expected missing flag argument error")
	}
	if _, _, err := normalizeExplainArgs([]string{"backup", "other"}); err == nil {
		t.Fatalf("expected multiple job error")
	}
	if _, _, err := normalizeExplainArgs([]string{"--at", "2026-02-24T10:00:00Z"}); err == nil {
		t.Fatalf("expected missing job error")
	}
}
