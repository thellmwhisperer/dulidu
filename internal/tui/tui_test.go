package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/thellmwhisperer/runscreen"
	"github.com/thellmwhisperer/runscreen/markdown"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
)

// runAt is a new run folder: r its report.json, questions its questions.json when there are any, and out the lines
// of its agent output.
func runAt(t *testing.T, r pipeline.Report, questions []string, out ...string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "lumpa")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(r)
	files := map[string][]byte{"report.json": b, Output: []byte(strings.Join(out, "\n") + "\n")}
	if questions != nil {
		files["questions.json"], _ = json.Marshal(questions)
	}
	for name, b := range files {
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// screen is runscreen's screen at cols by rows sent what Watch's first read of the run folder dir sends it.
func screen(t *testing.T, dir string, cols, rows int) (*runscreen.Model, string) {
	t.Helper()
	m := runscreen.New(cols, rows)
	m.SetMarkdown(markdown.Inline)
	msgs, err := (&view{dir: dir}).read()
	if err != nil {
		t.Fatal(err)
	}
	for _, msg := range msgs {
		m.Update(msg)
	}
	return m, ansi.Strip(m.View())
}

// says is a claude stream-json line of the agent saying text.
func says(text string) string {
	b, _ := json.Marshal(map[string]any{"type": "assistant", "message": map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}}}})
	return string(b)
}

// uses is a claude stream-json line of the agent calling tool with input, by id.
func uses(id, tool string, input map[string]string) string {
	b, _ := json.Marshal(map[string]any{"type": "assistant", "message": map[string]any{
		"content": []map[string]any{{"type": "tool_use", "id": id, "name": tool, "input": input}}}})
	return string(b)
}

// result is a claude stream-json line of the result of tool call id.
func result(id, content string) string {
	b, _ := json.Marshal(map[string]any{"type": "user", "message": map[string]any{
		"content": []map[string]any{{"type": "tool_result", "tool_use_id": id, "content": content}}}})
	return string(b)
}

func TestFrameHasStepsLeftAndTheAgentsLastLinesRight(t *testing.T) {
	at := time.Now().Add(-time.Minute)
	r := pipeline.Report{Status: "running", Steps: []pipeline.Step{
		{Name: "input", Status: "done", Started: at, Finished: at.Add(5 * time.Second)},
		{Name: "triage", Status: "running", Harness: "claude", Started: at.Add(5 * time.Second)},
	}}
	_, frame := screen(t, runAt(t, r, nil, says("old"), says("reading"), uses("t1", "Read", map[string]string{
		"file_path": "brief.md"})), 120, 40)
	first, _, _ := strings.Cut(frame, "\n")
	if !strings.Contains(first, "Pipeline") || !strings.Contains(first, "Agent · claude") {
		t.Fatalf("want the pipeline box and the Agent box side by side:\n%s", first)
	}
	for _, want := range []string{"╭", "lumpa", "✓", "Input", "5.0 s", "Triage", "✎ reading", "1 tool: Read · brief.md", "quit"} {
		if !strings.Contains(frame, want) {
			t.Errorf("frame lacks %q:\n%s", want, frame)
		}
	}
	if !regexp.MustCompile(`Triage  55\.\d s`).MatchString(frame) {
		t.Errorf("the running step lacks its time so far:\n%s", frame)
	}
}

func TestFrameShowsTheQuestionsUnderTheSteps(t *testing.T) {
	at := time.Now().Add(-time.Minute)
	r := pipeline.Report{Status: "waiting_answers", Steps: []pipeline.Step{
		{Name: "input", Status: "done", Started: at, Finished: at.Add(5 * time.Second)},
	}}
	_, frame := screen(t, runAt(t, r, []string{"Who pays?", "For how long?"}), 120, 40)
	if strings.Contains(frame, "Agent") || !strings.Contains(frame, "Questions") || !strings.Contains(frame, "1. Who pays?") ||
		!strings.Contains(frame, "2. For how long?") || !strings.Contains(frame, "⏸ waiting") || !strings.Contains(frame, "quit") {
		t.Fatalf("questions box:\n%s", frame)
	}
	if strings.Contains(frame, "--resume") || strings.Contains(frame, "--answers") {
		t.Fatalf("legacy run offers removed flags in its screen:\n%s", frame)
	}
}

func TestWatchReadsTheAgentsTextAndToolsThroughTheAdapterOfItsCLI(t *testing.T) {
	steps := func(status, harness string) pipeline.Report {
		return pipeline.Report{Status: status, Steps: []pipeline.Step{{Name: "triage", Status: status, Harness: harness,
			Started: time.Now()}}}
	}
	dir := runAt(t, steps("running", "claude"),
		nil, `{"type":"system","subtype":"init"}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"Reading the brief.\nThen triage."},{"type":"tool_use","id":"r1","name":"Read"}]}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"r1","content":"..."}]}}`,
		`dulidu: failed at research`)
	m := runscreen.New(120, 40)
	v := &view{dir: dir}
	msgs, err := v.read()
	if err != nil {
		t.Fatal(err)
	}
	for _, msg := range msgs {
		m.Update(msg)
	}
	frame := ansi.Strip(m.View())
	for _, want := range []string{"Agent · claude", "Reading the brief.", "Then triage.", "1 tool: Read"} {
		if !strings.Contains(frame, want) {
			t.Errorf("frame lacks %q: %s", want, frame)
		}
	}
	if strings.Contains(frame, "dulidu: failed at research") {
		t.Errorf("diagnostic in Agent: %s", frame)
	}
	if got := strings.Join(v.log, "|"); !strings.Contains(got, "dulidu: failed at research") {
		t.Errorf("raw diagnostic was lost: %s", got)
	}
	b, _ := json.Marshal(steps("failed", "claude"))
	if err := os.WriteFile(filepath.Join(dir, "report.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	if msgs, err = v.read(); err != nil {
		t.Fatal(err)
	}
	for _, msg := range msgs {
		m.Update(msg)
	}
	if frame = ansi.Strip(m.View()); !strings.Contains(frame, "Log") || !strings.Contains(frame, "dulidu: failed at research") {
		t.Errorf("failed step lost its diagnostic: %s", frame)
	}
	for harness, line := range map[string]string{
		"codex": `{"type":"item.completed","item":{"type":"agent_message","text":"codex says"}}`,
		"pi":    `{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"pi says"}]}}`,
	} {
		if _, frame := screen(t, runAt(t, steps("running", harness), nil, line), 120, 40); !strings.Contains(frame, "Agent · "+harness) ||
			!strings.Contains(frame, harness+" says") {
			t.Errorf("%s's agent in Agent:\n%s", harness, frame)
		}
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
	at := time.Now().Add(-time.Minute)
	failed := pipeline.Report{Status: "failed", Steps: []pipeline.Step{
		{Name: "research", Status: "failed at research: market: no sources", Harness: "claude", Started: at,
			Finished: at.Add(32 * time.Second), Note: "dimensions: business, market, users; web: on"},
	}}
	_, frame := screen(t, runAt(t, failed, nil, "last agent line"), 120, 40)
	if !strings.Contains(frame, "Findings") || !strings.Contains(frame, "Research: failed at research: market: no sources") ||
		!strings.Contains(frame, "Log") || !strings.Contains(frame, "last agent line") {
		t.Fatalf("failed run:\n%s", frame)
	}
	for _, want := range []string{"✓ business", "✗ market", "– users"} {
		if !strings.Contains(frame, want) {
			t.Errorf("failed research lacks %q:\n%s", want, frame)
		}
	}
	_, narrow := screen(t, runAt(t, pipeline.Report{Status: "running", Steps: []pipeline.Step{
		{Name: "triage", Status: "running", Harness: "claude", Started: at},
	}}, nil, says("reading")), 80, 24)
	pipe, agent := strings.Index(narrow, "Pipeline"), strings.Index(narrow, "Agent")
	first, _, _ := strings.Cut(narrow, "\n")
	if pipe < 0 || agent < pipe || strings.Contains(first, "Agent") {
		t.Fatalf("80x24 should stack the pipeline above the Agent box:\n%s", narrow)
	}
}

func TestScrollKeysAndTFolding(t *testing.T) {
	at := time.Now().Add(-time.Minute)
	var log []string
	for i := range 40 {
		log = append(log, says(fmt.Sprintf("thought %02d", i)))
	}
	r := pipeline.Report{Status: "running", Steps: []pipeline.Step{{Name: "triage", Status: "running", Harness: "claude",
		Started: at}}}
	m, frame := screen(t, runAt(t, r, nil, log...), 120, 24)
	if !strings.Contains(frame, "thought 39") {
		t.Fatalf("want the latest line:\n%s", frame)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if strings.Contains(m.View(), "thought 39") {
		t.Fatal("up did not scroll the Agent box")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if !strings.Contains(m.View(), "thought 39") {
		t.Fatal("end did not follow the Agent box")
	}
	m, frame = screen(t, runAt(t, r, nil, says("Writing tests."),
		uses("t1", "Edit", map[string]string{"file_path": "a.go"}), result("t1", "edited"),
		uses("t2", "Bash", map[string]string{"command": "go test"}), result("t2", "passed")), 120, 40)
	if !strings.Contains(frame, "2 tools: Edit, Bash") || strings.Contains(frame, "ok Edit") {
		t.Fatalf("tools should be folded:\n%s", frame)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	unfolded := ansi.Strip(m.View())
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
