package tui

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
	"github.com/thellmwhisperer/dulidu/internal/tui/screen"
)

func TestFrameHasStepsLeftAndTheAgentsLastLinesRight(t *testing.T) {
	at := time.Date(2026, 10, 17, 1, 0, 0, 0, time.UTC)
	r := pipeline.Report{Status: "running", Steps: []pipeline.Step{
		{Name: "input", Status: "done", Started: at, Finished: at.Add(5 * time.Second)},
		{Name: "triage", Status: "running", Started: at.Add(5 * time.Second)},
	}}
	frame := Frame("20261017-010000-1", r, nil, []string{"old", "reading", "→ Read"}, 120, 40, at.Add(time.Minute))
	first, _, _ := strings.Cut(frame, "\n")
	if !strings.Contains(first, "Pipeline") || !strings.Contains(first, "Agent") {
		t.Fatalf("want the pipeline box and the Agent box side by side:\n%s", first)
	}
	for _, want := range []string{"╭", "✓", "Input", "0 min 05 s", "Triage", "0 min 55 s", "✎ reading", "→ Read", "detach"} {
		if !strings.Contains(frame, want) {
			t.Errorf("frame lacks %q:\n%s", want, frame)
		}
	}
}

func TestFrameShowsTheQuestionsUnderTheSteps(t *testing.T) {
	at := time.Date(2026, 10, 17, 1, 0, 0, 0, time.UTC)
	r := pipeline.Report{Status: "waiting_answers", Steps: []pipeline.Step{
		{Name: "input", Status: "done", Started: at, Finished: at.Add(5 * time.Second)},
	}}
	frame := Frame("20261017-010000-1", r, []string{"Who pays?", "For how long?"}, nil, 120, 40, at.Add(time.Minute))
	if strings.Contains(frame, "Agent") || !strings.Contains(frame, "Questions") || !strings.Contains(frame, "1. Who pays?") ||
		!strings.Contains(frame, "2. For how long?") || !strings.Contains(frame, "detach") {
		t.Fatalf("questions box:\n%s", frame)
	}
	if strings.Contains(frame, "--resume") || strings.Contains(frame, "--answers") {
		t.Fatalf("legacy run offers removed flags in its screen:\n%s", frame)
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
	want := "Reading the brief.|Then triage.|→ Read|codex says"
	if got := strings.Join(tail(dir), "|"); got != want {
		t.Errorf("tail: %q, want %q", got, want)
	}
	// Keep diagnostics on disk for the failed-step log, outside the agent prose.
	if got := strings.Join(rawLines(dir), "|"); !strings.Contains(got, "dulidu: failed at research") {
		t.Errorf("raw diagnostic was lost: %s", got)
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

func TestFrameShowsAFailedRunAndAStackedTerminal(t *testing.T) {
	at := time.Date(2026, 10, 17, 1, 0, 0, 0, time.UTC)
	failed := pipeline.Report{Status: "failed", Steps: []pipeline.Step{
		{Name: "research", Status: "failed at research: market: no sources", Started: at, Finished: at.Add(32 * time.Second)},
	}}
	frame := Frame("lumpa", failed, nil, []string{"last agent line"}, 120, 40, at.Add(time.Minute))
	if !strings.Contains(frame, "Findings") || !strings.Contains(frame, "failed at research: market: no sources") || !strings.Contains(frame, "Log") || !strings.Contains(frame, "last agent line") {
		t.Fatalf("failed run:\n%s", frame)
	}
	narrow := Frame("lumpa", pipeline.Report{Status: "running", Steps: []pipeline.Step{
		{Name: "triage", Status: "running", Started: at},
	}}, nil, []string{"reading"}, 80, 24, at.Add(time.Minute))
	pipe, agent := strings.Index(narrow, "Pipeline"), strings.Index(narrow, "Agent")
	first, _, _ := strings.Cut(narrow, "\n")
	if pipe < 0 || agent < pipe || strings.Contains(first, "Agent") {
		t.Fatalf("80x24 should stack the pipeline above the Agent box:\n%s", narrow)
	}
}

func TestScrollKeysAndTFolding(t *testing.T) {
	at := time.Date(2026, 10, 17, 1, 0, 0, 0, time.UTC)
	var log []string
	for i := range 40 {
		log = append(log, fmt.Sprintf("thought %02d", i))
	}
	r := pipeline.Report{Status: "running", Steps: []pipeline.Step{{Name: "triage", Status: "running", Started: at}}}
	m := screen.New("lumpa", 120, 24)
	m.Load(screen.Snap{Label: "lumpa", Report: r, Log: log, Now: at.Add(time.Minute)})
	if !strings.Contains(m.View(), "thought 39") {
		t.Fatalf("want the latest line:\n%s", m.View())
	}
	m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if strings.Contains(m.View(), "thought 39") {
		t.Fatal("up did not scroll the Agent box")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if !strings.Contains(m.View(), "thought 39") {
		t.Fatal("end did not follow the Agent box")
	}
	folded := []string{
		`{"says":"Writing tests."}`,
		`{"id":"t1","tool":"Edit","detail":"a.go","result":"passed"}`,
		`{"id":"t2","tool":"Bash","detail":"go test","result":"passed"}`,
	}
	m = screen.New("lumpa", 120, 40)
	m.Load(screen.Snap{Label: "lumpa", Report: r, Log: folded, Now: at.Add(time.Minute)})
	if !strings.Contains(m.View(), "2 tools: Edit, Bash") || strings.Contains(m.View(), "ok Edit") {
		t.Fatalf("tools should be folded:\n%s", m.View())
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	unfolded := m.View()
	if !strings.Contains(unfolded, "Edit a.go") || !strings.Contains(unfolded, "Bash go test") || !strings.Contains(unfolded, "ok") {
		t.Fatalf("t did not unfold:\n%s", unfolded)
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
