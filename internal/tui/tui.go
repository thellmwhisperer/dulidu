// Package tui is the view of a run folder: a pipeline box and, beside or under it, what the agent is doing,
// drawn by runscreen live on a terminal, or one line per step on anything else. It reads report.json and the agent
// output; it never drives the run.
package tui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/thellmwhisperer/runscreen"
	"github.com/thellmwhisperer/runscreen/adapter/claude"
	"github.com/thellmwhisperer/runscreen/adapter/codex"
	"github.com/thellmwhisperer/runscreen/adapter/pi"
	"github.com/thellmwhisperer/runscreen/markdown"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
	"github.com/thellmwhisperer/dulidu/internal/run"
)

// Output is the run folder's agent output: every line its agents write, as they write it, then what dulidu itself
// prints. A live run holds a lock on it.
const Output = "agent.out"

// every is how often the view reads the run folder again.
var every = 250 * time.Millisecond

// grace is how long a view keeps reading a run folder that holds no live run and no report either, the window
// between the folder's creation and its run's lock; past it no report will ever be written.
const grace = 2 * time.Second

// Report is dir's report.json.
func Report(dir string) (pipeline.Report, error) {
	var r pipeline.Report
	b, err := os.ReadFile(filepath.Join(dir, "report.json"))
	if err == nil {
		err = json.Unmarshal(b, &r)
	}
	return r, err
}

// Questions is dir's questions.json, the questions a run waiting for answers asks.
func Questions(dir string) ([]string, error) {
	var q []string
	b, err := os.ReadFile(filepath.Join(dir, "questions.json"))
	if err == nil {
		err = json.Unmarshal(b, &q)
	}
	return q, err
}

// Line is s on one line: its name, its status and how long it took, or has taken by now while it runs.
func Line(s pipeline.Step, now time.Time) string {
	end := s.Finished
	if end.IsZero() {
		end = now
	}
	return s.Name + " " + s.Status + " " + run.Human(end.Sub(s.Started))
}

// running says whether a dulidu run still holds dir's agent output; a run that died holds nothing.
func running(dir string) bool {
	f, err := os.Open(filepath.Join(dir, Output))
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	return syscall.Flock(int(f.Fd()), syscall.LOCK_SH|syscall.LOCK_NB) != nil
}

// Follow writes one line per step of the run in dir to w as each finishes, then, when it stopped for answers,
// one line per question, and returns once the run has ended, every step written; a run that died before writing
// any report fails it, once grace has passed.
func Follow(dir string, w io.Writer) error {
	done := 0
	stuck := time.Time{} // since a run not running last left its report unreadable
	for ; ; time.Sleep(every) {
		ended := !running(dir) // before the report, so the report read after it is the last one
		r, err := Report(dir)
		if err != nil {
			if !ended {
				stuck = time.Time{}
				continue // not written yet, or being written
			}
			if stuck.IsZero() {
				stuck = time.Now() // the run may still be about to start and lock its output
				continue
			}
			if time.Since(stuck) < grace {
				continue
			}
			return err // nothing holds the run's output, so nothing will ever write its report
		}
		stuck = time.Time{}
		for ; done < len(r.Steps) && (ended || !r.Steps[done].Finished.IsZero()); done++ {
			if _, err := fmt.Fprintln(w, Line(r.Steps[done], time.Now())); err != nil {
				return err
			}
		}
		if ended {
			if r.Status == "waiting_answers" {
				questions, _ := Questions(dir)
				for _, q := range questions {
					if _, err := fmt.Fprintln(w, q); err != nil {
						return err
					}
				}
			}
			return nil
		}
	}
}

// Watch draws the run in dir on w with runscreen until q, ctx ends, or an observed live run ends. An already
// finished run stays open for inspection. A run with no report ends the view once grace has passed, and fails it.
// Leaving the view never stops the run.
func Watch(ctx context.Context, dir string, w io.Writer, keys io.Reader, size func() (cols, rows int)) error {
	m := runscreen.New(size())
	m.SetMarkdown(markdown.Inline)
	ctx, stop := context.WithCancel(ctx)
	// bubbletea ends the screen itself on SIGINT and SIGTERM, and a quit sent on the same signal would leave its
	// signal handler stuck: the screen is quit from here only when neither came.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	ended := make(chan error, 1)
	err := runscreen.Show(m, keys, w, func(send func(tea.Msg)) {
		ended <- feed(ctx, dir, send)
		signal.Stop(signals) // once it returns, a signal that ended ctx is in signals
		if len(signals) == 0 {
			send(tea.QuitMsg{})
		}
	})
	stop()
	if errors.Is(err, tea.ErrInterrupted) {
		err = nil // a signal ends the view as q does, and the run goes on
	}
	return errors.Join(err, <-ended)
}

// feed sends the screen the run in dir, every while, until ctx ends or the run gives no more: its run ended, or
// it never wrote a report, its error.
func feed(ctx context.Context, dir string, send func(tea.Msg)) error {
	send(pipelineOf(filepath.Base(dir), pipeline.Report{})) // its steps, all pending, until its report says more
	v := &view{dir: dir}
	for {
		msgs, err := v.read()
		for _, msg := range msgs {
			send(msg)
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(every):
		}
	}
}

// view is what the screen was sent of the run in dir: how far into its agent output it read, the agent it last
// named, and the lines of the output that are no event.
type view struct {
	dir   string
	seen  int // bytes of the agent output read
	agent runscreen.Agent
	log   []string  // dulidu's own lines and the agents' stderr, shown once the run failed
	alive bool      // a live run was seen
	stuck time.Time // since a run not running last left its report unreadable
}

// read is what the screen is sent once dir is read again: its pipeline, the agent of its latest step, the events
// of the agent's new lines and the boxes under them. A live run that ended gives io.EOF with its last messages; a
// run that never wrote a report gives its error, once grace has passed.
func (v *view) read() ([]tea.Msg, error) {
	alive := running(v.dir)
	ended := v.alive && !alive
	v.alive = v.alive || alive
	r, err := Report(v.dir)
	if err != nil {
		if alive {
			v.stuck = time.Time{}
		} else if v.stuck.IsZero() {
			v.stuck = time.Now() // the run may still be about to start and lock its output
		} else if time.Since(v.stuck) >= grace {
			return nil, err // nothing holds the run's output, so nothing will ever write its report
		}
		return nil, nil
	}
	v.stuck = time.Time{}
	msgs := []tea.Msg{pipelineOf(filepath.Base(v.dir), r)}
	if a := agentOf(r); a != v.agent {
		v.agent = a
		msgs = append(msgs, a)
	}
	msgs = append(msgs, v.lines(!alive)...)
	var questions []string
	if r.Status == "waiting_answers" {
		questions, _ = Questions(v.dir)
	}
	msgs = append(msgs, boxesOf(r, questions, v.log))
	if ended {
		return msgs, io.EOF
	}
	return msgs, nil
}

// readers is runscreen's adapter of each CLI it reads, by dulidu's name for its harness.
var readers = map[string]func([]byte) []runscreen.Event{"claude": claude.Read, "codex": codex.Read, "pi": pi.Read}

// lines is the events of the agent output's lines written since the last read, read by the adapter of the agent
// that runs; whole lines only, unless all. A line that is no JSON goes to the log instead.
func (v *view) lines(all bool) []tea.Msg {
	b, _ := os.ReadFile(filepath.Join(v.dir, Output)) // none is no line yet
	fresh := b[min(v.seen, len(b)):]
	if !all {
		fresh = fresh[:bytes.LastIndexByte(fresh, '\n')+1] // the last line may be half written
	}
	v.seen += len(fresh)
	read := readers[v.agent.Harness]
	var msgs []tea.Msg
	for _, line := range strings.Split(string(fresh), "\n") {
		switch line = strings.TrimSpace(line); {
		case line == "":
		case !json.Valid([]byte(line)):
			v.log = append(v.log, line)
		case read != nil:
			for _, e := range read([]byte(line)) {
				msgs = append(msgs, e)
			}
		}
	}
	return msgs
}

// agentOf is the agent of r's latest step that names one: its harness, model and effort.
func agentOf(r pipeline.Report) (a runscreen.Agent) {
	for _, s := range r.Steps {
		if s.Harness != "" {
			a = runscreen.Agent{Harness: s.Harness, Model: s.Model, Effort: s.Effort}
		}
	}
	return a
}

// stepOrder is dulidu's pipeline, top to bottom.
var stepOrder = []string{"input", "triage", "research", "analysis", "epics", "design", "export"}

// pipelineOf is the pipeline box of the run label whose report is r: a row for each step of dulidu's, pending
// until r has it, with its state, its times and its note, and the research step's dimensions under it.
func pipelineOf(label string, r pipeline.Report) runscreen.Pipeline {
	by := map[string]pipeline.Step{}
	for _, s := range r.Steps {
		by[s.Name] = s
	}
	p := runscreen.Pipeline{Title: label, State: stateOf(r.Status)}
	for _, name := range stepOrder {
		s := by[name]
		row := runscreen.Step{Name: title(name), State: stateOf(s.Status), Start: s.Started,
			End: s.Finished, Note: s.Note}
		names := dimensions(s.Note)
		for _, d := range names {
			row.Steps = append(row.Steps, runscreen.Step{Name: d, State: dimension(s.Status, d, names)})
		}
		p.Steps = append(p.Steps, row)
	}
	return p
}

// stateOf is the state of a run or step whose status report.json says is status; pending until it says one.
func stateOf(status string) runscreen.State {
	switch {
	case status == "running":
		return runscreen.Running
	case status == "done" || status == "passed":
		return runscreen.Passed
	case strings.HasPrefix(status, "waiting"):
		return runscreen.Waiting
	case strings.HasPrefix(status, "failed"):
		return runscreen.Failed
	}
	return runscreen.Pending
}

// dimensions is the dimensions the research step's note "dimensions: a, b; web: on" names, none for another note.
func dimensions(note string) []string {
	_, names, _ := strings.Cut(note, "dimensions:")
	names, _, _ = strings.Cut(names, ";")
	var out []string
	for _, name := range strings.Split(names, ",") {
		if name = strings.TrimSpace(name); name != "" {
			out = append(out, name)
		}
	}
	return out
}

// dimension is the state of the research dimension name, one of names, in the research step whose status is
// status: those of a passed step passed; of a step "failed at research: <dimension>: <reason>", those before it
// passed, it failed and those after it skipped; pending while it runs. The report has no clock of each dimension.
func dimension(status, name string, names []string) runscreen.State {
	switch state := stateOf(status); {
	case state == runscreen.Passed:
		return runscreen.Passed
	case state != runscreen.Failed:
		return runscreen.Pending
	}
	rest, _ := strings.CutPrefix(status, "failed at research: ")
	failed, _, _ := strings.Cut(rest, ":")
	switch at, end := slices.Index(names, name), slices.Index(names, failed); {
	case at < end:
		return runscreen.Passed
	case at == end:
		return runscreen.Failed
	}
	return runscreen.Skipped
}

// boxesOf is the boxes under the Agent box of the run whose report is r: the questions it waits for, the findings
// of its failed steps and, once it failed, the log, its lines that are no event. A box with no row is left out.
func boxesOf(r pipeline.Report, questions, log []string) runscreen.Boxes {
	var asked, findings, logged []runscreen.Row
	for i, q := range questions {
		asked = append(asked, runscreen.Row{Text: fmt.Sprintf("%d. %s", i+1, q)})
	}
	for _, s := range r.Steps {
		if stateOf(s.Status) == runscreen.Failed {
			findings = append(findings, runscreen.Row{Text: title(s.Name) + ": " + s.Status})
		}
	}
	if stateOf(r.Status) == runscreen.Failed {
		for _, line := range log {
			logged = append(logged, runscreen.Row{Text: line})
		}
	}
	boxes := runscreen.Boxes{{Title: "Questions", Rows: asked}, {Title: "Findings", Rows: findings},
		{Title: "Log", Rows: logged}}
	return slices.DeleteFunc(boxes, func(b runscreen.Box) bool { return len(b.Rows) == 0 })
}

// title is a step's name as its row shows it: Triage for triage.
func title(name string) string { return strings.ToUpper(name[:1]) + name[1:] }
