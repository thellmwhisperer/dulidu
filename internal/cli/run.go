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
	"github.com/thellmwhisperer/dulidu/internal/pipeline/input"
	agent "github.com/thellmwhisperer/dulidu/internal/run"
)

// runCmd is dulidu run: a new run folder in .dulidu/runs with the brief and its attachments in input/ and
// contexto.md, the input step, packaged from them. It prints contexto.md's path.
func runCmd() *cobra.Command {
	var attach []string
	c := &cobra.Command{Use: "run <brief.md> [--attach <path>...]", Short: "Package a brief and its attachments into contexto.md",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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
			if c.VideoFrameSeconds < 0 {
				return errors.New("video_frame_seconds: want a positive number of seconds")
			}
			runs := filepath.Join(root, ".dulidu", "runs")
			if err := os.MkdirAll(runs, 0o755); err != nil {
				return err
			}
			now := time.Now()
			dir, err := os.MkdirTemp(runs, now.Format("20060102-150405-"))
			if err != nil {
				return err
			}
			step := c.Step("input")
			step.Harness, step.Status, step.Started = cmp.Or(step.Harness, "claude"), "running", now
			r := pipeline.Run{Report: pipeline.Report{Status: "running", Started: now, Steps: []pipeline.Step{step}}, Input: &in}
			if err := pipeline.Write(dir, r); err != nil {
				return err
			}
			a := agent.Agent{Harness: step.Harness, Model: c.Steps.Input.Model, Effort: c.Steps.Input.Effort, Timeout: limit}
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
			r.Report.Status = "done"
			if err != nil {
				r.Report.Status = "failed"
			}
			r.Report.Steps[0].Status, r.Report.Steps[0].Finished = r.Report.Status, time.Now()
			if log.Len() > 0 {
				r.Logs = map[string]string{"input": log.String()}
			}
			if err := errors.Join(err, pipeline.Write(dir, r)); err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), filepath.Join(dir, "contexto.md"))
			return err
		}}
	c.Flags().StringArrayVar(&attach, "attach", nil, "a file that comes with the brief: pdf, docx, md, txt, json, png, jpg, webp, heic, mp4, mov or webm")
	return c
}
