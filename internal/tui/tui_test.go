package tui

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
)

func TestFrameHasStepsLeftAndTheAgentsLastLinesRight(t *testing.T) {
	at := time.Date(2026, 10, 17, 1, 0, 0, 0, time.UTC)
	r := pipeline.Report{Status: "running", Steps: []pipeline.Step{
		{Name: "input", Status: "done", Started: at, Finished: at.Add(5 * time.Second)},
		{Name: "triage", Status: "running", Started: at.Add(5 * time.Second)},
	}}
	frame := Frame("20261017-010000-1", r, nil, []string{"old", "reading", "→ Read"}, 90, 3, at.Add(time.Minute))
	lines := strings.Split(frame, "\n")
	want := []string{"20261017-010000-1 running", "input done 0 min 05 s", "triage running 0 min 55 s"}
	if len(lines) != 3 {
		t.Fatalf("%d lines, want 3:\n%s", len(lines), frame)
	}
	for i, line := range lines {
		if len([]rune(line)) != 90 || !strings.HasPrefix(line, want[i]) {
			t.Errorf("line %d: %q, want 90 columns starting %q", i, line, want[i])
		}
	}
	if !strings.HasSuffix(strings.TrimRight(lines[1], " "), "│ reading") || !strings.HasSuffix(strings.TrimRight(lines[2], " "), "│ → Read") {
		t.Errorf("right panel is not the log's last lines:\n%s", frame)
	}
}

func TestFrameShowsTheQuestionsUnderTheSteps(t *testing.T) {
	at := time.Date(2026, 10, 17, 1, 0, 0, 0, time.UTC)
	r := pipeline.Report{Status: "waiting_answers", Steps: []pipeline.Step{
		{Name: "input", Status: "done", Started: at, Finished: at.Add(5 * time.Second)},
	}}
	frame := Frame("20261017-010000-1", r, []string{"Who pays?", "For how long?"}, nil, 90, 5, at.Add(time.Minute))
	lines := strings.Split(frame, "\n")
	want := []string{"20261017-010000-1 waiting_answers", "input done 0 min 05 s", "? Who pays?", "? For how long?"}
	if len(lines) != 5 {
		t.Fatalf("%d lines, want 5:\n%s", len(lines), frame)
	}
	for i, prefix := range want {
		if !strings.HasPrefix(lines[i], prefix) {
			t.Errorf("line %d: %q, want it to start %q", i, lines[i], prefix)
		}
	}
}

func TestTailReadsTheAgentsTextAndTools(t *testing.T) {
	dir := t.TempDir()
	out := strings.Join([]string{
		`{"type":"system","subtype":"init"}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"Reading the brief.\nThen triage."},{"type":"tool_use","name":"Read"}]}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","content":"..."}]}}`,
		`{"type":"item.completed","item":{"type":"agent_message","text":"codex says"}}`,
		`dulidu: failed at research`,
	}, "\n")
	if err := os.WriteFile(filepath.Join(dir, Output), []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	want := "Reading the brief.|Then triage.|→ Read|codex says|dulidu: failed at research"
	if got := strings.Join(tail(dir), "|"); got != want {
		t.Errorf("tail: %q, want %q", got, want)
	}
}

func TestFollowPrintsTheQuestionsARunWaitsFor(t *testing.T) {
	dir := t.TempDir()
	report := `{"status": "waiting_answers", "started": "2026-10-17T01:00:00Z", "steps": [
		{"name": "input", "status": "done", "model": "", "harness": "claude",
		 "started": "2026-10-17T01:00:00Z", "finished": "2026-10-17T01:00:05Z"},
		{"name": "triage", "status": "passed", "model": "", "harness": "claude",
		 "started": "2026-10-17T01:00:05Z", "finished": "2026-10-17T01:01:10Z"}]}`
	if err := os.WriteFile(filepath.Join(dir, "report.json"), []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	questions := "[\n  \"Who pays?\",\n  \"For how long?\"\n]\n"
	if err := os.WriteFile(filepath.Join(dir, "questions.json"), []byte(questions), 0o644); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	if err := Follow(dir, &b); err != nil {
		t.Fatalf("Follow: %v", err)
	}
	want := "input done 0 min 05 s\ntriage passed 1 min 05 s\nWho pays?\nFor how long?\n"
	if b.String() != want {
		t.Errorf("Follow of a run waiting for answers: %q, want %q", b.String(), want)
	}
}

func TestFollowEndsARunNothingHolds(t *testing.T) {
	dir := t.TempDir()
	report := `{"status": "running", "started": "2026-10-17T01:00:00Z", "steps": [
		{"name": "input", "status": "running", "model": "", "harness": "claude", "started": "2026-10-17T01:00:00Z"}]}`
	if err := os.WriteFile(filepath.Join(dir, "report.json"), []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	if err := Follow(dir, &b); err != nil || !strings.HasPrefix(b.String(), "input running ") {
		t.Errorf("Follow of a dead run: %v, %q", err, b.String())
	}
}

func TestFollowEndsWhenTheRunDiedBeforeItsReport(t *testing.T) {
	var b strings.Builder
	if err := Follow(t.TempDir(), &b); err == nil || b.Len() != 0 {
		t.Errorf("Follow of a run that died before its report: %v, %q", err, b.String())
	}
}

func TestWatchEndsWhenTheRunDiedBeforeItsReport(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// A reader at its end never presses q, so only the run's state can end the view.
	if err := Watch(ctx, t.TempDir(), io.Discard, strings.NewReader(""), func() (int, int) { return 80, 24 }); err == nil {
		t.Error("Watch of a run that died before its report: no error")
	}
}
