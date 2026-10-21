package cli

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/thellmwhisperer/dulidu/internal/config"
	"github.com/thellmwhisperer/dulidu/internal/pipeline"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/analysis"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/design"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/epics"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/export"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/input"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/research"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/triage"
	agent "github.com/thellmwhisperer/dulidu/internal/run"
	"github.com/thellmwhisperer/dulidu/internal/tui"
)

// runCmd is dulidu run: a new run folder in .dulidu/runs with the brief and its attachments in input/ and
// contexto.md, the input step, packaged from them, then triage.json, the triage step, read from contexto.md, then
// research.json, analysis.json, epics.json and screens.json, each its step, then the export: spec.md, tickets/, plan.json and
// brief-diseno.md. The first step to fail stops the run. Without --follow it prints contexto.md's path; on a terminal
// it runs in the background and attaches to it. With --follow it stays in the foreground and streams JSON events.
func runCmd() *cobra.Command {
	var attach []string
	var follow bool
	c := &cobra.Command{Use: "run <brief.md> [--attach <path>...]",
		Short: "Package a brief and its attachments into contexto.md and take it to spec, tickets, plan and design brief",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (runErr error) {
			in, err := input.Load(args[0], attach)
			if err != nil {
				return err
			}
			root, err := toplevel()
			if err != nil {
				return err
			}
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			c, _, err := config.Load(root, home)
			if err == nil {
				err = c.Complete()
			}
			if err != nil {
				return err
			}
			limit, err := time.ParseDuration(cmp.Or(c.AgentTimeout, "0s"))
			if err != nil {
				return err
			}
			runs := filepath.Join(root, ".dulidu", "runs")
			var dir string
			var a agent.Agent
			var r pipeline.Run
			var agentLog io.Writer
			events := runEvents{}
			if follow {
				events.out = json.NewEncoder(cmd.OutOrStdout())
			}
			// ask runs a, the current step's agent, in dir.
			ask := func(prompt string) (string, error) { return a.Run(dir, prompt, func(string) error { return nil }) }
			background := os.Getenv(runEnv)
			live := os.Stderr // a background run's stdout and stderr are its agent output, locked already
			// next makes a the named step's agent, set by s, and is the step as report.json records it.
			next := func(name string, s config.Step) pipeline.Step {
				events.finish(r)
				events.start(name)
				step := c.Step(name)
				step.Harness, step.Effort = cmp.Or(step.Harness, "claude"), s.Effort
				a = agent.Agent{Harness: step.Harness, Model: s.Model, Effort: s.Effort, Timeout: limit, Log: agentLog,
					Web: name == "research"}
				return step
			}
			if dir = background; dir == "" {
				if err := os.MkdirAll(runs, 0o755); err != nil {
					return err
				}
				if dir, err = os.MkdirTemp(runs, time.Now().Format("20060102-150405-")); err != nil {
					return err
				}
			}
			if !follow && background == "" && terminal(cmd.OutOrStdout()) {
				return detach(cmd, dir)
			}
			if background == "" {
				if live, err = lock(dir); err != nil {
					return err
				}
				defer func() { _ = live.Close() }()
			}
			agentLog = live
			if follow {
				agentLog = io.MultiWriter(live, &events)
			}
			now := time.Now()
			step := c.Step("input")
			step.Harness, step.Effort, step.Status, step.Started = cmp.Or(step.Harness, "claude"), c.Steps.Input.Effort, "running", now
			r = pipeline.Run{Report: pipeline.Report{Status: "running", Started: now, Steps: []pipeline.Step{step}}, Input: &in}
			events.start("input")
			defer func() {
				events.finish(r)
				status, code := r.Report.Status, 0
				if runErr != nil {
					status, code = "failed", 1
				}
				events.emit("run_end", "run", struct {
					Status   string `json:"status"`
					ExitCode int    `json:"exit_code"`
				}{status, code})
				runErr = errors.Join(runErr, events.err)
			}()
			if err := pipeline.Write(dir, r); err != nil {
				return err
			}
			a = agent.Agent{Harness: step.Harness, Model: c.Steps.Input.Model, Effort: c.Steps.Input.Effort, Timeout: limit, Log: agentLog}
			var log strings.Builder
			seconds := cmp.Or(c.VideoFrameSeconds, input.FrameSeconds)
			r.Context, err = input.PackageEvery(dir, in, seconds, func(prompt string) (string, error) {
				final, err := a.Run(dir, prompt, func(final string) error {
					if strings.TrimSpace(final) == "" {
						return errors.New("no description in its final message")
					}
					return nil
				})
				log.WriteString(prompt + "\n\n" + final + "\n\n")
				return final, err
			})
			r.Report.Steps[0].Status, r.Report.Steps[0].Finished = "passed", time.Now()
			if err != nil {
				r.Report.Status, r.Report.Steps[0].Status = "failed", "failed"
			}
			if log.Len() > 0 {
				r.Logs = map[string]string{"input": log.String()}
			}
			if err := errors.Join(err, pipeline.Write(dir, r)); err != nil {
				return err
			}
			if err := triage.Run(dir, &r, next("triage", c.Steps.Triage), c.MaxEpics, c.MaxStoriesPerEpic, ask); err != nil {
				return err
			}
			// Stories are the epics step's second phase, on its agent.
			err = research.Run(dir, &r, next("research", c.Steps.Research), ask)
			if err == nil {
				err = analysis.Run(dir, &r, next("analysis", c.Steps.Analysis), ask)
			}
			if err == nil {
				err = epics.Run(dir, &r, next("epics", c.Steps.Epics), ask)
			}
			if err == nil {
				err = design.Run(dir, &r, next("design", c.Steps.Design), ask)
			}
			if err == nil {
				events.finish(r)
				events.start("export")
				r.Report.Status = "running"
				r.Report.Steps = append(r.Report.Steps, pipeline.Step{Name: "export", Status: "running", Started: time.Now()})
				err = export.Write(dir, &r)
				step := &r.Report.Steps[len(r.Report.Steps)-1]
				step.Finished = time.Now()
				if err != nil {
					step.Status, r.Report.Status = "failed at export: "+err.Error(), "failed"
					return errors.Join(fmt.Errorf("failed at export: %w", err), pipeline.Write(dir, r))
				}
				step.Status, r.Report.Status = "passed", "done"
				err = pipeline.Write(dir, r)
			}
			if err != nil {
				return err
			}
			if follow {
				return nil
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), filepath.Join(dir, "contexto.md"))
			return err
		}}
	c.Flags().BoolVar(&follow, "follow", false, "Stream every run event as JSON on stdout until the run ends")
	c.Flags().StringArrayVar(&attach, "attach", nil, "a file that comes with the brief: pdf, docx, md, txt, json, png, jpg, webp, heic, mp4, mov or webm")
	return c
}

// runEnv is set, to its run folder, in the dulidu that runs a run in the background.
const runEnv = "DULIDU_RUN"

// detach runs this same dulidu run in dir in the background, in its own session, its stdout and stderr its agent
// output, locked before it starts, and attaches to it; once the view ends, a failed run fails this command too.
func detach(cmd *cobra.Command, dir string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	out, err := lock(dir)
	if err != nil {
		return err
	}
	child := exec.Command(self, os.Args[1:]...)
	child.Env = append(os.Environ(), runEnv+"="+dir)
	child.Stdout, child.Stderr = out, out
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	err = child.Start()
	_ = out.Close() // the child holds the lock now
	if err != nil {
		return err
	}
	_ = child.Process.Release()
	if err := attach(cmd, dir); err != nil {
		return err
	}
	if r, err := tui.Report(dir); err == nil && r.Status == "failed" {
		return fmt.Errorf("run %s failed", filepath.Base(dir))
	}
	return nil
}

// lock opens dir's agent output to append to and locks it, for attach to see the run alive while it is open; a run
// already running in dir fails it.
func lock(dir string) (*os.File, error) {
	f, err := os.OpenFile(filepath.Join(dir, tui.Output), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("run %s is running", filepath.Base(dir))
	}
	return f, nil
}
