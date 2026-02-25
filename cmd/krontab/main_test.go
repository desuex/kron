package main

import (
	"reflect"
	"testing"
)

const (
	mainArgFile         = "--file"
	mainArgCount        = "--count"
	mainJobsPath        = "/tmp/jobs.kron"
	mainAt20260224T1000 = "2026-02-24T10:00:00Z"
	errJobMismatch      = "job mismatch: got %q want %q"
	errArgsMismatch     = "args mismatch: got %v want %v"
)

func TestNormalizeExplainArgsJobFirst(t *testing.T) {
	job, args, err := normalizeExplainArgs([]string{
		"backup",
		mainArgFile, mainJobsPath,
		"--at", mainAt20260224T1000,
	})
	if err != nil {
		t.Fatalf("normalizeExplainArgs error: %v", err)
	}
	if job != "backup" {
		t.Fatalf(errJobMismatch, job, "backup")
	}
	want := []string{mainArgFile, mainJobsPath, "--at", mainAt20260224T1000}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf(errArgsMismatch, args, want)
	}
}

func TestNormalizeExplainArgsFlagsFirst(t *testing.T) {
	job, args, err := normalizeExplainArgs([]string{
		mainArgFile, mainJobsPath,
		"--at", mainAt20260224T1000,
		"backup",
	})
	if err != nil {
		t.Fatalf("normalizeExplainArgs error: %v", err)
	}
	if job != "backup" {
		t.Fatalf(errJobMismatch, job, "backup")
	}
	want := []string{mainArgFile, mainJobsPath, "--at", mainAt20260224T1000}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf(errArgsMismatch, args, want)
	}
}

func TestNormalizeExplainArgsRejectsMultipleJobs(t *testing.T) {
	_, _, err := normalizeExplainArgs([]string{"backup", "other", "--at", mainAt20260224T1000})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestNormalizeNextArgsJobFirst(t *testing.T) {
	job, args, err := normalizeNextArgs([]string{
		"backup",
		mainArgFile, mainJobsPath,
		mainArgCount, "3",
	})
	if err != nil {
		t.Fatalf("normalizeNextArgs error: %v", err)
	}
	if job != "backup" {
		t.Fatalf(errJobMismatch, job, "backup")
	}
	want := []string{mainArgFile, mainJobsPath, mainArgCount, "3"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf(errArgsMismatch, args, want)
	}
}

func TestNormalizeNextArgsFlagsFirstAndErrors(t *testing.T) {
	job, args, err := normalizeNextArgs([]string{
		mainArgFile, mainJobsPath,
		mainArgCount, "2",
		"backup",
	})
	if err != nil {
		t.Fatalf("normalizeNextArgs error: %v", err)
	}
	if job != "backup" {
		t.Fatalf(errJobMismatch, job, "backup")
	}
	want := []string{mainArgFile, mainJobsPath, mainArgCount, "2"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf(errArgsMismatch, args, want)
	}

	if _, _, err := normalizeNextArgs([]string{mainArgFile}); err == nil {
		t.Fatalf("expected missing flag argument error")
	}
	if _, _, err := normalizeNextArgs([]string{"backup", "other"}); err == nil {
		t.Fatalf("expected multiple job error")
	}
	if _, _, err := normalizeNextArgs([]string{mainArgFile, mainJobsPath}); err == nil {
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
	if _, _, err := normalizeExplainArgs([]string{"--at", mainAt20260224T1000}); err == nil {
		t.Fatalf("expected missing job error")
	}
}
