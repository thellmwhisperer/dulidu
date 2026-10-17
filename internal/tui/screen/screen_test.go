package screen

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
)

func plain(s string) string { return ansi.Strip(s) }

func runningReport(at time.Time) pipeline.Report {
	return pipeline.Report{Status: "running", Steps: []pipeline.Step{
		{Name: "input", Status: "done", Started: at, Finished: at.Add(5 * time.Second), Note: "packaged"},
		{Name: "triage", Status: "running", Started: at.Add(5 * time.Second)},
	}}
}

func render(label string, r pipeline.Report, questions, log []string, cols, rows int, now time.Time) string {
	return Frame(Snap{Label: label, Report: r, Questions: questions, Log: log, Now: now}, cols, rows)
}

func TestRunningRunAt120HasTwoBoxesIconsAndDurations(t *testing.T) {
	at := time.Date(2026, 10, 17, 1, 0, 0, 0, time.UTC)
	frame := render("20261017-010000-1", runningReport(at), nil, []string{"reading the brief"}, 120, 40, at.Add(time.Minute))
	s := plain(frame)
	lines := strings.Split(s, "\n")
	if len(lines) > 40 {
		t.Fatalf("120x40 used %d rows:\n%s", len(lines), s)
	}
	for _, line := range strings.Split(frame, "\n") {
		if ansi.StringWidth(line) > 120 {
			t.Fatalf("line wider than 120: %q", plain(line))
		}
	}
	first := lines[0]
	if !strings.Contains(first, "Pipeline") || !strings.Contains(first, "Agent") {
		t.Fatalf("want both boxes on the first row:\n%s", first)
	}
	for _, want := range []string{"╭", "╮", "╰", "╯", "20261017-010000-1", "running", "✓", "Input", "0 min 05 s",
		"done", "Triage", "0 min 55 s", "○", "Research", "pending", "✎ reading the brief", "q detach", "t tools"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q:\n%s", want, s)
		}
	}
	if !strings.ContainsAny(frame, "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏") {
		t.Errorf("missing a running icon:\n%s", s)
	}
	if strings.Contains(s, "…") {
		t.Errorf("a short screen was cut:\n%s", s)
	}
}

func TestWaitingAnswersShowsEveryQuestionComplete(t *testing.T) {
	at := time.Date(2026, 10, 17, 1, 0, 0, 0, time.UTC)
	long := "Who pays for the kiln, the clay and the long weekend workshop that runs past the usual closing time?"
	r := pipeline.Report{Status: "waiting_answers", Steps: []pipeline.Step{
		{Name: "input", Status: "done", Started: at, Finished: at.Add(5 * time.Second)},
		{Name: "triage", Status: "passed", Started: at, Finished: at.Add(time.Minute)},
	}}
	frame := render("lumpa", r, []string{"Who pays?", long}, nil, 120, 40, at.Add(2*time.Minute))
	s := plain(frame)
	if strings.Contains(s, "Agent") || !strings.Contains(s, "Questions") {
		t.Fatalf("want a Questions box in place of the Agent box:\n%s", s)
	}
	if !strings.Contains(s, "1. Who pays?") || !strings.Contains(s, "Who pays for the kiln, the clay and the long weekend workshop that runs") ||
		!strings.Contains(s, "past the usual closing time?") || strings.Contains(s, "…") {
		t.Fatalf("a question was cut:\n%s", s)
	}
	if !strings.Contains(s, "dulidu run --resume lumpa --answers answers.md") || !strings.Contains(s, "q detach") {
		t.Fatalf("footer:\n%s", s)
	}
	if strings.Contains(s, "t tools") {
		t.Fatalf("t tools without the Agent box:\n%s", s)
	}
}

func TestFailedRunShowsTheFindingsReason(t *testing.T) {
	at := time.Date(2026, 10, 17, 1, 0, 0, 0, time.UTC)
	r := pipeline.Report{Status: "failed", Steps: []pipeline.Step{
		{Name: "input", Status: "done", Started: at, Finished: at.Add(time.Second)},
		{Name: "research", Status: "failed at research: market: no sources", Started: at, Finished: at.Add(32 * time.Second),
			Note: "dimensions: market, users"},
	}}
	frame := render("lumpa", r, nil, []string{"last agent line"}, 120, 40, at.Add(time.Minute))
	s := plain(frame)
	if i, j := strings.Index(s, "Findings"), strings.Index(s, "Log"); i < 0 || j < i || !strings.Contains(s, "Research") || !strings.Contains(s, "failed at research: market: no sources") {
		t.Fatalf("findings above the log:\n%s", s)
	}
	if !strings.Contains(s, "last agent line") {
		t.Fatalf("log under findings:\n%s", s)
	}
	if strings.Contains(s, "Agent") {
		t.Fatalf("agent box on a failed run:\n%s", s)
	}
	if !strings.Contains(s, "✗") || !strings.Contains(s, "0 min 32 s") || !strings.Contains(s, "market") || !strings.Contains(s, "users") {
		t.Fatalf("failed research row:\n%s", s)
	}
}

func TestDoneAndExportKeepEarlierProseInTheAgentBox(t *testing.T) {
	at := time.Date(2026, 10, 17, 1, 0, 0, 0, time.UTC)
	var log []string
	for i := range 40 {
		log = append(log, "earlier "+itoa(i))
	}
	done := pipeline.Report{Status: "done", Steps: []pipeline.Step{
		{Name: "epics", Status: "passed", Started: at, Finished: at.Add(time.Second)},
		{Name: "export", Status: "done", Started: at, Finished: at.Add(2 * time.Second)},
	}}
	m := New("lumpa", 120, 24)
	m.Load(Snap{Label: "lumpa", Report: done, Log: log, Now: at.Add(time.Minute)})
	s := plain(m.View())
	if !strings.Contains(s, "Agent") || !strings.Contains(s, "earlier 39") || !strings.Contains(s, "t tools") || strings.Contains(s, "Findings") {
		t.Fatalf("done run dropped the Agent box:\n%s", s)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyHome})
	if !strings.Contains(plain(m.View()), "earlier 0") || strings.Contains(plain(m.View()), "earlier 39") {
		t.Fatalf("earlier prose is not reachable by scrolling up:\n%s", plain(m.View()))
	}
	exporting := pipeline.Report{Status: "running", Steps: []pipeline.Step{
		{Name: "epics", Status: "passed", Started: at, Finished: at.Add(time.Second)},
		{Name: "export", Status: "running", Started: at.Add(time.Second)},
	}}
	frame := plain(render("lumpa", exporting, nil, []string{"earlier prose"}, 120, 40, at.Add(time.Minute)))
	if !strings.Contains(frame, "Agent") || !strings.Contains(frame, "earlier prose") || strings.Contains(frame, "Questions") || strings.Contains(frame, "Findings") {
		t.Fatalf("export hid the Agent box:\n%s", frame)
	}
}

func TestQuestionWrapKeepsAWordThatFits(t *testing.T) {
	at := time.Date(2026, 10, 17, 1, 0, 0, 0, time.UTC)
	word := strings.Repeat("k", 70)
	r := pipeline.Report{Status: "waiting_answers", Steps: []pipeline.Step{
		{Name: "input", Status: "done", Started: at, Finished: at.Add(time.Second)},
	}}
	s := plain(render("lumpa", r, []string{"pay " + word}, nil, 120, 40, at.Add(time.Minute)))
	if !strings.Contains(s, word) || strings.Contains(s, "\u2026") {
		t.Fatalf("a word that fits the box was split or cut:\n%s", s)
	}
}

func TestNarrowTerminalStacksThePipelineOnTop(t *testing.T) {
	at := time.Date(2026, 10, 17, 1, 0, 0, 0, time.UTC)
	frame := render("lumpa", runningReport(at), nil, []string{"reading"}, 80, 24, at.Add(time.Minute))
	s := plain(frame)
	lines := strings.Split(s, "\n")
	if len(lines) > 24 {
		t.Fatalf("80x24 used %d rows:\n%s", len(lines), s)
	}
	pipe, agent := -1, -1
	for i, line := range lines {
		if strings.Contains(line, "Pipeline") && pipe < 0 {
			pipe = i
		}
		if strings.Contains(line, "Agent") && agent < 0 {
			agent = i
		}
		if strings.Contains(line, "Pipeline") && strings.Contains(line, "Agent") {
			t.Fatalf("boxes share a row under 100 columns: %q", line)
		}
	}
	if pipe < 0 || agent <= pipe {
		t.Fatalf("want the pipeline box above the Agent box:\n%s", s)
	}
	wide := plain(render("lumpa", runningReport(at), nil, []string{"reading"}, 100, 40, at.Add(time.Minute)))
	first, _, _ := strings.Cut(wide, "\n")
	if !strings.Contains(first, "Pipeline") || !strings.Contains(first, "Agent") {
		t.Fatalf("at 100 columns, want the boxes side by side: %q", first)
	}
}

func TestScrollKeysHoldAndFollow(t *testing.T) {
	at := time.Date(2026, 10, 17, 1, 0, 0, 0, time.UTC)
	var log []string
	for i := range 40 {
		log = append(log, "thought "+itoa(i))
	}
	m := New("lumpa", 120, 24)
	m.Load(Snap{Label: "lumpa", Report: runningReport(at), Log: log, Now: at.Add(time.Minute)})
	s := plain(m.View())
	if !strings.Contains(s, "thought 39") {
		t.Fatalf("want the latest line followed:\n%s", s)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m.Update(tea.KeyMsg{Type: tea.KeyUp})
	held := plain(m.View())
	if strings.Contains(held, "thought 39") {
		t.Fatalf("up did not leave the bottom:\n%s", held)
	}
	m.Load(Snap{Label: "lumpa", Report: runningReport(at), Log: append(log, "thought 40"), Now: at.Add(time.Minute)})
	if plain(m.View()) != held {
		t.Fatalf("new output moved a held box:\n%s", plain(m.View()))
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if !strings.Contains(plain(m.View()), "thought 40") {
		t.Fatalf("end did not follow again:\n%s", plain(m.View()))
	}
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !strings.Contains(plain(m.View()), "Pipeline*") {
		t.Fatalf("tab did not focus the pipeline:\n%s", plain(m.View()))
	}
}

func TestTFoldsAndUnfoldsToolCalls(t *testing.T) {
	at := time.Date(2026, 10, 17, 1, 0, 0, 0, time.UTC)
	log := []string{
		`{"thinking":"Plan the fix."}`,
		`{"says":"Writing the red tests first."}`,
		`{"id":"t1","tool":"Edit","detail":"model_test.go","result":"passed"}`,
		`{"id":"t2","tool":"Bash","detail":"go test ./...","result":"passed"}`,
		`{"id":"t3","tool":"Bash","detail":"go vet ./...","result":"passed"}`,
		`{"says":"Red on behaviour. Now the fix."}`,
		`{"id":"t4","tool":"Bash","detail":"go build ./...","result":"failed","summary":"exit status 1"}`,
		`{"id":"t5","tool":"Read","detail":"view.go","result":"running"}`,
	}
	m := New("lumpa", 120, 40)
	m.Load(Snap{Label: "lumpa", Report: runningReport(at), Log: log, Now: at.Add(time.Minute)})
	s := plain(m.View())
	for _, want := range []string{"∴ Plan the fix.", "✎ Writing the red tests first.", "· 3 tools: Edit, Bash ×2",
		"✎ Red on behaviour. Now the fix.", "✗ failed Bash go build ./...", "1 tool: Read · view.go"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "ok Bash go test") {
		t.Fatalf("folded calls are on their own rows:\n%s", s)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	s = plain(m.View())
	if !strings.Contains(s, "ok Edit model_test.go") || !strings.Contains(s, "ok Bash go test ./...") || !strings.Contains(s, "failed Bash go build ./... · exit status 1") {
		t.Fatalf("t did not unfold:\n%s", s)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if strings.Contains(plain(m.View()), "ok Bash go test") {
		t.Fatalf("a second t did not fold:\n%s", plain(m.View()))
	}
}

func TestMarkdownWrapsInsideTheAgentBox(t *testing.T) {
	at := time.Date(2026, 10, 17, 1, 0, 0, 0, time.UTC)
	words := strings.Repeat("lorem ipsum ", 16) + "omega"
	s := plain(render("lumpa", runningReport(at), nil, []string{words}, 120, 40, at.Add(time.Minute)))
	if strings.Contains(s, "…") || !strings.Contains(s, "✎ lorem") || !strings.Contains(s, "omega") {
		t.Fatalf("long prose was cut:\n%s", s)
	}
	raw := render("lumpa", runningReport(at), nil, []string{"**bold** and _italic_"}, 120, 40, at.Add(time.Minute))
	if !strings.Contains(raw, "\x1b[1m") || !strings.Contains(plain(raw), "bold") || !strings.Contains(plain(raw), "italic") {
		t.Fatalf("markdown styles missing:\n%s", plain(raw))
	}
	if strings.Contains(raw, "\x1b[38;") || strings.Contains(raw, "\x1b[48;") {
		t.Fatalf("a colour outside ANSI roles:\n%q", raw)
	}
}

func TestResearchSubRowsAndClaudeTools(t *testing.T) {
	at := time.Date(2026, 10, 17, 1, 0, 0, 0, time.UTC)
	r := pipeline.Report{Status: "running", Steps: []pipeline.Step{
		{Name: "input", Status: "done", Started: at, Finished: at.Add(time.Second)},
		{Name: "triage", Status: "passed", Started: at, Finished: at.Add(2 * time.Second)},
		{Name: "research", Status: "running", Started: at.Add(2 * time.Second), Note: "dimensions: market, users"},
	}}
	log := []string{
		`{"type":"assistant","message":{"content":[{"type":"thinking","thinking":"Look up the market."},{"type":"text","text":"Reading the brief."},{"type":"tool_use","id":"t1","name":"Read","input":{"file_path":"brief.md"}}]}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t1","content":"Bikes.","is_error":false}]}}`,
	}
	s := plain(render("lumpa", r, nil, log, 120, 40, at.Add(time.Minute)))
	if !strings.Contains(s, "market") || !strings.Contains(s, "users") || !strings.Contains(s, "∴ Look up the market.") ||
		!strings.Contains(s, "✎ Reading the brief.") || !strings.Contains(s, "1 tool: Read") {
		t.Fatalf("research rows or agent lines:\n%s", s)
	}
}

func TestReadKeysDecodesTheFooterKeys(t *testing.T) {
	out := make(chan tea.Msg, 8)
	go readKeys(strings.NewReader("t\x1b[A\x1b[6~\x1b[<64;8;3M\t"), out)
	var got []string
	for msg := range out {
		switch m := msg.(type) {
		case tea.KeyMsg:
			got = append(got, m.String())
		case tea.MouseMsg:
			if m.Button != tea.MouseButtonWheelUp || m.X != 7 || m.Y != 2 {
				t.Fatalf("wheel %+v", m)
			}
			got = append(got, "wheel")
		}
	}
	want := []string{"t", "up", "pgdown", "wheel", "tab"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("keys %q, want %q", got, want)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [4]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
