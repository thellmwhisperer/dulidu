// Package tui is the view of a run folder: a pipeline box and, beside or under it, what the agent is doing,
// live on a terminal, or one line per step on anything else. It reads report.json and the agent output; it never
// drives the run.
package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
	"github.com/thellmwhisperer/dulidu/internal/run"
	"github.com/thellmwhisperer/dulidu/internal/tui/screen"
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

// Watch draws the run in dir on w until q, ctx ends, or an observed live run ends.
// An already finished run stays open for inspection. A run with no report ends
// the view once grace has passed. Leaving the view never stops the run.
func Watch(ctx context.Context, dir string, w io.Writer, keys io.Reader, size func() (cols, rows int)) error {
	return screen.Poll(ctx, w, keys, size, watchLoad(dir))
}

// watchLoad is dir's report, questions and agent prose. It returns the final frame
// with io.EOF when a live run ends. Missing reports fail after grace; writes are skipped.
func watchLoad(dir string) func(time.Time) (screen.Snap, error) {
	var last screen.Snap
	have := false
	wasAlive := false
	stuck := time.Time{} // since a run not running last left its report unreadable
	return func(now time.Time) (screen.Snap, error) {
		alive := running(dir)
		ended := wasAlive && !alive
		wasAlive = wasAlive || alive
		r, err := Report(dir)
		if err != nil {
			if alive {
				stuck = time.Time{}
			} else if stuck.IsZero() {
				stuck = time.Now() // the run may still be about to start and lock its output
			} else if time.Since(stuck) >= grace {
				return screen.Snap{}, err // nothing holds the run's output, so nothing will ever write its report
			}
			if have {
				last.Now = now
				return last, nil
			}
			return screen.Snap{Label: filepath.Base(dir), Now: now}, nil
		}
		stuck = time.Time{}
		var questions []string
		if r.Status == "waiting_answers" {
			questions, _ = Questions(dir)
		}
		log := rawLines(dir)
		if r.Status != "failed" {
			log = slices.DeleteFunc(log, func(line string) bool { return !json.Valid([]byte(line)) })
		}
		last = screen.Snap{Label: filepath.Base(dir), Report: r, Questions: questions, Log: log, Now: now}
		have = true
		if ended {
			return last, io.EOF
		}
		return last, nil
	}
}

// Frame is the screen of cols by rows: a pipeline box and, beside or under it, the agent, the questions the run
// waits for, or the findings of a failed step.
func Frame(label string, r pipeline.Report, questions []string, log []string, cols, rows int, now time.Time) string {
	return screen.Frame(screen.Snap{Label: label, Report: r, Questions: questions, Log: log, Now: now}, cols, rows)
}

// rawLines is every non-empty line of dir's agent output, so the screen can keep them and scroll.
func rawLines(dir string) []string {
	b, err := os.ReadFile(filepath.Join(dir, Output))
	if err != nil {
		return nil
	}
	var lines []string
	for _, line := range strings.Split(string(b), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
