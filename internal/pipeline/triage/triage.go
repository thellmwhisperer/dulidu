// Package triage reads contexto.md once for the first planning step and writes a typed triage.json that code checks and caps.
package triage

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/prompts"
)

// Run triages r.Context in dir, the run folder: it adds step to r's report, running, asks ask once with the triage
// prompt and writes r back. A valid answer is r.Triage, its caps clamped to maxEpics and maxStories when positive,
// each clamp in the step's note, and the step passed; else the step and r fail, the step "failed at triage: <reason>",
// and that is Run's error.
func Run(dir string, r *pipeline.Run, step pipeline.Step, maxEpics, maxStories int, ask func(prompt string) (string, error)) error {
	prompt, err := prompts.Render("triage", map[string]any{"Context": r.Context})
	if err != nil {
		return err
	}
	step.Status, step.Started = "running", time.Now()
	r.Report.Steps = append(r.Report.Steps, step)
	if err := pipeline.Write(dir, *r); err != nil {
		return err
	}
	answer, err := ask(prompt)
	if r.Logs == nil {
		r.Logs = map[string]string{}
	}
	r.Logs["triage"] = prompt + "\n\n" + answer + "\n\n"
	var t pipeline.Triage
	if err == nil && (json.Unmarshal([]byte(answer), &t) != nil || t.Validate() != nil) {
		err = errors.New("invalid output")
	}
	s := &r.Report.Steps[len(r.Report.Steps)-1]
	s.Finished = time.Now()
	if err != nil {
		err = fmt.Errorf("failed at triage: %w", err)
		s.Status, r.Report.Status = err.Error(), "failed"
		return errors.Join(err, pipeline.Write(dir, *r))
	}
	var notes []string
	for _, c := range []struct {
		caps     *int
		max      int
		cap, key string
	}{{&t.Caps.Epics, maxEpics, "epics", "max_epics"}, {&t.Caps.StoriesPerEpic, maxStories, "stories_per_epic", "max_stories_per_epic"}} {
		if c.max > 0 && *c.caps > c.max {
			notes = append(notes, fmt.Sprintf("clamped caps.%s from %d to %s %d", c.cap, *c.caps, c.key, c.max))
			*c.caps = c.max
		}
	}
	s.Status, s.Note, r.Report.Status, r.Triage = "passed", strings.Join(notes, "; "), "done", &t
	return pipeline.Write(dir, *r)
}
