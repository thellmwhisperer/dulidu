package cli

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/thellmwhisperer/dulidu/internal/config"
	"github.com/thellmwhisperer/dulidu/internal/pipeline"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/analysis"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/epics"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/export"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/input"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/research"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/triage"
	agent "github.com/thellmwhisperer/dulidu/internal/run"
)

// runCmd is dulidu run: a new run folder in .dulidu/runs with the brief and its attachments in input/ and
// contexto.md, the input step, packaged from them, then triage.json, the triage step, read from contexto.md, and,
// when triage asks no questions, research.json, analysis.json and epics.json, each its step, then the export: spec.md,
// tickets/, plan.json and brief-diseno.md. The first step to fail stops the run. Triage's questions instead go to
// questions.json and are printed, and the run waits for answers: --resume <label> --answers <file> adds them to its
// contexto.md and goes on to research and the rest in the same run. It prints contexto.md's path.
func runCmd() *cobra.Command {
	var attach []string
	var resume, answers string
	c := &cobra.Command{Use: "run <brief.md> [--attach <path>...] | run --resume <label> --answers <answers.md>",
		Short: "Package a brief and its attachments into contexto.md and take it to spec, tickets, plan and design brief",
		Args: func(cmd *cobra.Command, args []string) error {
			if resume != "" {
				return cobra.NoArgs(cmd, args)
			}
			return cobra.ExactArgs(1)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var in pipeline.InputContext
			var err error
			if resume == "" {
				if in, err = input.Load(args[0], attach); err != nil {
					return err
				}
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
			var r pipeline.Run
			var dir string
			var a agent.Agent
			// ask runs a, the current step's agent, in dir.
			ask := func(prompt string) (string, error) { return a.Run(dir, prompt, func(string) error { return nil }) }
			// next makes a the named step's agent, set by s, and is the step as report.json records it.
			next := func(name string, s config.Step) pipeline.Step {
				step := c.Step(name)
				step.Harness, step.Effort = cmp.Or(step.Harness, "claude"), s.Effort
				a = agent.Agent{Harness: step.Harness, Model: s.Model, Effort: s.Effort, Timeout: limit}
				return step
			}
			if resume != "" {
				dir = filepath.Join(runs, resume)
				if r, err = pipeline.Resume(dir, answers); err != nil {
					return err
				}
			} else {
				if err := os.MkdirAll(runs, 0o755); err != nil {
					return err
				}
				now := time.Now()
				dir, err = os.MkdirTemp(runs, now.Format("20060102-150405-"))
				if err != nil {
					return err
				}
				step := c.Step("input")
				step.Harness, step.Status, step.Started = cmp.Or(step.Harness, "claude"), "running", now
				r = pipeline.Run{Report: pipeline.Report{Status: "running", Started: now, Steps: []pipeline.Step{step}}, Input: &in}
				if err := pipeline.Write(dir, r); err != nil {
					return err
				}
				a = agent.Agent{Harness: step.Harness, Model: c.Steps.Input.Model, Effort: c.Steps.Input.Effort, Timeout: limit}
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
				r.Report.Steps[0].Status, r.Report.Steps[0].Finished = "done", time.Now()
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
			}
			// Triage's missing_questions, if any, stop a new run until it is resumed.
			if resume == "" && len(r.Triage.MissingQuestions) > 0 {
				err := pipeline.Ask(dir, &r)
				var questions []byte
				if err == nil {
					questions, err = os.ReadFile(filepath.Join(dir, "questions.json"))
				}
				if err == nil {
					_, err = cmd.OutOrStdout().Write(questions)
				}
				if err != nil {
					return err
				}
			} else {
				// Stories are the epics step's second phase, on its agent.
				err := research.Run(dir, &r, next("research", c.Steps.Research), ask)
				if err == nil {
					err = analysis.Run(dir, &r, next("analysis", c.Steps.Analysis), ask)
				}
				if err == nil {
					err = epics.Run(dir, &r, next("epics", c.Steps.Epics), ask)
				}
				if err == nil {
					err = export.Write(dir, &r)
				}
				if err != nil {
					return err
				}
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), filepath.Join(dir, "contexto.md"))
			return err
		}}
	c.Flags().StringArrayVar(&attach, "attach", nil, "a file that comes with the brief: pdf, docx, md, txt, json, png, jpg, webp, heic, mp4, mov or webm")
	c.Flags().StringVar(&resume, "resume", "", "the label of a run waiting for answers, to go on with")
	c.Flags().StringVar(&answers, "answers", "", "the file with the answers to a resumed run's questions")
	return c
}
