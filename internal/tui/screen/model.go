// Package screen draws a run folder: a pipeline box and, beside or under it, what the agent is doing.
// It reads a report, questions and agent lines; it never drives the run.
package screen

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
)

// Snap is one read of a run folder.
type Snap struct {
	Label     string
	Report    pipeline.Report
	Questions []string
	Log       []string
	Now       time.Time
}

// row is a pipeline row: a step, or a research dimension under the step before.
type row struct {
	step, label, state, note string
	check                    bool
	start                    time.Time
	took                     time.Duration
	hasDuration              bool
}

// activity is one thing the agent said, thought or called.
type activity struct {
	id, tool, detail, result, summary, thinking, says string
}

// finding is one row of the Findings box.
type finding struct {
	severity, ref, text string
}

// Model is the screen: the report so far, drawn at the terminal's size.
type Model struct {
	label, state                  string
	rows                          []*row
	agent                         []*activity
	output                        []string
	questions                     []string
	findings                      []finding
	width, height, spin           int
	now                           time.Time
	pipeline, history             pane
	pipelineFocus                 bool
	pipelineBounds, historyBounds bounds
	historyHeight                 int
	unfold                        bool
	open                          map[string]bool
	toggles                       []string
	mode, runningStep             string
}

type bounds struct{ x, y, width, height int }

type paneLine struct{ key, parent, text, toggle string }

type pane struct {
	top, last, h, part int
	anchor, parent     string
	held               bool
}

func (p *pane) show(lines []paneLine, h int) []string {
	oldAnchor, oldParent, oldPart, oldTop := p.anchor, p.parent, p.part, p.top
	p.h, p.last = h, max(0, len(lines)-h)
	if p.held && oldAnchor != "" {
		p.top = keyedLine(lines, oldAnchor, oldPart)
		if p.top < 0 && oldParent != "" {
			p.top = keyedLine(lines, oldParent, 0)
		}
		if p.top < 0 {
			p.top = min(oldTop, p.last)
		}
	}
	if !p.held || p.top >= p.last {
		p.top, p.held = p.last, false
	}
	if p.top < len(lines) {
		p.anchor, p.parent = lines[p.top].key, lines[p.top].parent
		p.part = countKey(lines, p.top, lines[p.top].key)
	}
	out := make([]string, 0, min(h, len(lines)-p.top))
	for _, line := range lines[p.top:min(len(lines), p.top+h)] {
		out = append(out, line.text)
	}
	return out
}

func keyedLine(lines []paneLine, key string, part int) int {
	first, count := -1, 0
	for i, line := range lines {
		if line.key == key {
			if first < 0 {
				first = i
			}
			count++
		}
	}
	if first < 0 {
		return -1
	}
	return first + min(part, count-1)
}

func countKey(lines []paneLine, at int, key string) int {
	n := 0
	for i := at - 1; i >= 0 && lines[i].key == key; i-- {
		n++
	}
	return n
}

func (p *pane) scroll(n int) {
	p.top = min(max(0, p.top+n), p.last)
	p.held = p.top < p.last
	p.anchor, p.parent, p.part = "", "", 0
}

// focused is the pane keys scroll: the agent or questions box while it shows, unless the user moved to the pipeline.
func (m *Model) focused() *pane {
	if m.pipelineFocus || !m.rightScrolls() {
		return &m.pipeline
	}
	return &m.history
}

// New is an empty screen of the given size.
func New(label string, width, height int) *Model {
	return &Model{label: label, state: "waiting", width: width, height: height, now: time.Now()}
}

// Load replaces what the screen shows from snap, keeping a held scroll position.
func (m *Model) Load(s Snap) {
	if s.Label != "" {
		m.label = s.Label
	}
	if !s.Now.IsZero() {
		m.now = s.Now
	}
	m.apply(s.Report, s.Questions)
	m.agent = activities(s.Log)
	m.output = readableLines(s.Log)
}

func (m *Model) apply(r pipeline.Report, questions []string) {
	m.state = r.Status
	if m.state == "" {
		m.state = "waiting"
	}
	m.questions = questions
	m.findings = findingsOf(r)
	by := map[string]pipeline.Step{}
	for _, s := range r.Steps {
		by[s.Name] = s
	}
	var rows []*row
	var running string
	for _, name := range stepOrder {
		s, ok := by[name]
		got := stepRow(name, s, ok, m.now)
		if !got.check && got.state == "running" {
			running = got.step
		}
		rows = append(rows, got)
		if name != "research" {
			continue
		}
		for _, d := range dimensions(s.Note) {
			rows = append(rows, &row{step: "research", label: d, state: dimState(s, d, dimensions(s.Note)), check: true})
		}
	}
	if running != "" && running != m.runningStep {
		m.history = pane{}
		m.pipelineFocus = false
		m.runningStep = running
	}
	if running == "" {
		m.runningStep = ""
	}
	m.rows = rows
	m.noteMode()
}

// stepOrder is the pipeline, top to bottom.
var stepOrder = []string{"input", "triage", "research", "analysis", "epics", "export"}

var stepLabel = map[string]string{
	"input": "Input", "triage": "Triage", "research": "Research",
	"analysis": "Analysis", "epics": "Epics", "export": "Export",
}

func stepRow(name string, s pipeline.Step, ok bool, now time.Time) *row {
	r := &row{step: name, label: stepLabel[name], state: "pending"}
	if !ok {
		return r
	}
	r.note = s.Note
	r.state = rowState(s.Status)
	if !s.Started.IsZero() {
		r.start = s.Started
		end := s.Finished
		if end.IsZero() {
			end = now
		}
		r.took, r.hasDuration = end.Sub(s.Started), true
	}
	return r
}

func rowState(status string) string {
	switch {
	case status == "running":
		return "running"
	case status == "done" || status == "passed":
		return status
	case status == "waiting" || strings.HasPrefix(status, "waiting"):
		return "waiting"
	case status == "skipped" || status == "not run":
		return status
	case status == "failed" || strings.HasPrefix(status, "failed"):
		return "failed"
	case status == "":
		return "pending"
	default:
		return status
	}
}

// dimensions is the research step's note "dimensions: a, b", empty for none.
func dimensions(note string) []string {
	const p = "dimensions:"
	i := strings.Index(note, p)
	if i < 0 {
		return nil
	}
	rest := strings.TrimSpace(note[i+len(p):])
	if rest == "" || rest == "none" {
		return nil
	}
	var out []string
	for _, name := range strings.Split(rest, ",") {
		if name = strings.TrimSpace(name); name != "" && name != "none" {
			out = append(out, name)
		}
	}
	return out
}

// dimState is a dimension's state from the research step. A running step has no per-dimension
// progress in the report, so those rows stay pending until the step ends.
// ponytail: no per-dimension clock in report.json; show live progress only if the report grows a field for it.
func dimState(s pipeline.Step, name string, names []string) string {
	switch {
	case s.Status == "passed":
		return "passed"
	case s.Status == "failed" || strings.HasPrefix(s.Status, "failed"):
		failed := failedDimension(s.Status)
		if failed == "" {
			return "failed"
		}
		if name == failed {
			return "failed"
		}
		if indexOf(names, name) < indexOf(names, failed) {
			return "passed"
		}
		return "not run"
	default:
		return "pending"
	}
}

func failedDimension(status string) string {
	// "failed at research: market: no sources"
	rest, ok := strings.CutPrefix(status, "failed at research: ")
	if !ok {
		return ""
	}
	name, _, _ := strings.Cut(rest, ":")
	return strings.TrimSpace(name)
}

func indexOf(ss []string, s string) int {
	for i, v := range ss {
		if v == s {
			return i
		}
	}
	return -1
}

func findingsOf(r pipeline.Report) []finding {
	var out []finding
	for _, s := range r.Steps {
		if s.Status != "failed" && !strings.HasPrefix(s.Status, "failed") {
			continue
		}
		ref := stepLabel[s.Name]
		if ref == "" {
			ref = s.Name
		}
		out = append(out, finding{severity: "error", ref: ref, text: s.Status})
	}
	if len(out) == 0 && (r.Status == "failed" || strings.HasPrefix(r.Status, "failed")) {
		out = append(out, finding{severity: "error", ref: "run", text: r.Status})
	}
	return out
}

func (m *Model) noteMode() {
	mode := "pipeline"
	switch {
	case m.questionsShow():
		mode = "questions"
	case m.agentShows():
		mode = "agent"
	case len(m.findings) > 0:
		mode = "findings"
	}
	if mode == m.mode {
		return
	}
	m.history = pane{}
	m.mode = mode
	if mode == "questions" {
		m.history.held = true // stay on the first question; scrolling down reaches the rest
	}
	if mode == "agent" || mode == "questions" {
		m.pipelineFocus = false
	}
}

func (m *Model) questionsShow() bool { return m.state == "waiting_answers" }

func (m *Model) failed() bool {
	return m.state == "failed" || strings.HasPrefix(m.state, "failed")
}

func (m *Model) agentShows() bool {
	return !m.questionsShow() && len(m.findings) == 0
}

func (m *Model) showLog() bool { return m.failed() && len(m.output) > 0 }

func (m *Model) rightScrolls() bool { return m.agentShows() || m.questionsShow() }

func (m *Model) sidebar() bool {
	return m.agentShows() || m.questionsShow() || len(m.findings) > 0 || m.showLog()
}

// Init implements tea.Model. The screen is drawn by the caller, not a program.
func (m *Model) Init() tea.Cmd { return nil }

// Update applies a key or a mouse event.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		m.key(msg.String())
	case tea.MouseMsg:
		m.View() // bounds for the click
		m.wheel(msg)
	}
	return m, nil
}

func (m *Model) key(k string) {
	p := m.focused()
	switch k {
	case "t":
		m.unfold, m.open = !m.unfold, nil
	case "tab":
		if m.rightScrolls() {
			m.pipelineFocus = !m.pipelineFocus
		}
	case "up":
		p.scroll(-1)
	case "down":
		p.scroll(1)
	case "pgup":
		p.scroll(-max(1, p.h-1))
	case "pgdown":
		p.scroll(max(1, p.h-1))
	case "home":
		p.scroll(-p.top)
	case "end":
		p.scroll(p.last)
	}
}

func (m *Model) wheel(msg tea.MouseMsg) {
	var p *pane
	if m.pipelineBounds.contains(msg.X, msg.Y) {
		p = &m.pipeline
	} else if m.historyBounds.contains(msg.X, msg.Y) && m.rightScrolls() {
		p = &m.history
	} else {
		return
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		p.scroll(-3)
	case tea.MouseButtonWheelDown:
		p.scroll(3)
	case tea.MouseButtonLeft:
		row := msg.Y - m.historyBounds.y - 1
		if p == &m.history && msg.Action == tea.MouseActionPress && row >= 0 && row < len(m.toggles) && m.toggles[row] != "" {
			if m.open == nil {
				m.open = map[string]bool{}
			}
			m.open[m.toggles[row]] = !m.unfolded(m.toggles[row])
		}
	}
}

func (m *Model) unfolded(key string) bool {
	if open, ok := m.open[key]; ok {
		return open
	}
	return m.unfold
}

func (b bounds) contains(x, y int) bool {
	return x >= b.x && x < b.x+b.width && y >= b.y && y < b.y+b.height
}

// Frame is the screen of snap at cols by rows.
func Frame(s Snap, cols, rows int) string {
	m := New(s.Label, cols, rows)
	m.Load(s)
	return m.View()
}
