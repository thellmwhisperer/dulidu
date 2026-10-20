// Package triage reads contexto.md once for the first planning step and writes a typed triage.json that code checks and caps.
package triage

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/prompts"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/text"
)

// capEpics is the epics each complexity plans at most, the reference prototype's MAX_EPICS.
var capEpics = map[string]int{"simple": 3, "moderate": 5, "complex": 8}

// Run triages r.Context in dir, the run folder: it adds step to r's report, running, asks ask once with the triage
// prompt and writes r back with the step passed. The answer is plain text, read by package text: a valid one is
// r.Triage, its epics capped by its complexity, the whole of that cap when it gives none; an agent error or an invalid answer is a moderate r.Triage, with
// "classification failed, moderate: <reason>" as its reasoning and the step's note. A positive maxEpics clamps the
// epics, the clamp in the note; maxStories is r.Triage.StoriesPerEpic. Run's error is only a failed write.
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
	lead := text.Parse(answer)[0]
	epics, epicsErr := strconv.Atoi(lead.Field("Epics"))
	if err == nil && epicsErr != nil && lead.Field("Epics") != "" {
		err = errors.New("invalid output")
	}
	t := pipeline.Triage{Complexity: lead.Field("Complexity"), Epics: epics, Reasoning: lead.Prose}
	if most := capEpics[t.Complexity]; t.Epics <= 0 || t.Epics > most {
		t.Epics = most
	}
	if err == nil && t.Validate() != nil {
		err = errors.New("invalid output")
	}
	var notes []string
	if err != nil {
		t = pipeline.Triage{Complexity: "moderate", Epics: capEpics["moderate"], Reasoning: "classification failed, moderate: " + err.Error()}
		notes = append(notes, t.Reasoning)
	}
	if maxEpics > 0 && t.Epics > maxEpics {
		notes = append(notes, fmt.Sprintf("clamped epics from %d to max_epics %d", t.Epics, maxEpics))
		t.Epics = maxEpics
	}
	t.StoriesPerEpic = maxStories
	s := &r.Report.Steps[len(r.Report.Steps)-1]
	s.Finished, s.Status, s.Note, r.Report.Status, r.Triage = time.Now(), "passed", strings.Join(notes, "; "), "done", &t
	return pipeline.Write(dir, *r)
}
