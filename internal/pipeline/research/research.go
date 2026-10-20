// Package research asks business, users, market, technical and project in order and writes their answers, each with its
// sources, to research.json.
package research

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/prompts"
)

// templates is each dimension's prompt.
var templates = map[string]string{"business": "p03-research-business", "users": "p04-research-users",
	"market": "p05-research-market", "technical": "p06-research-technical", "project": "p07-research-project"}

// Run researches r.Context in dir, the run folder: it adds step to r's report, running, with the dimensions in its
// note, then the harness's web status, asks ask once per dimension in the prototype's order with that dimension's prompt,
// and writes r back. When every answer is a dimension with text and sources they are r.Research, in order, and the step passed; at
// the first that is not, no later one is asked, the step and r fail, the step "failed at research: <dimension>:
// <reason>", and that is Run's error.
func Run(dir string, r *pipeline.Run, step pipeline.Step, ask func(prompt string) (string, error)) error {
	names := []string{"business", "users", "market", "technical", "project"}
	step.Status, step.Started, step.Note = "running", time.Now(), "dimensions: "+strings.Join(names, ", ")
	if step.Harness == "claude" || step.Harness == "codex" { // their agents enable web search
		step.Note += "; web: on"
	} else {
		step.Note += "; web: off for " + step.Harness
	}
	r.Report.Status, r.Report.Steps = "running", append(r.Report.Steps, step)
	if err := pipeline.Write(dir, *r); err != nil {
		return err
	}
	if r.Logs == nil {
		r.Logs = map[string]string{}
	}
	research := pipeline.Research{Dimensions: []pipeline.Dimension{}}
	var err error
	for _, name := range names {
		var prompt, answer string
		prompt, err = prompts.Render(templates[name], map[string]any{"Input": r.Context})
		if err == nil {
			answer, err = ask(prompt)
			r.Logs["research"] += prompt + "\n\n" + answer + "\n\n"
		}
		var d pipeline.Dimension
		if err == nil && json.Unmarshal([]byte(answer), &d) != nil {
			err = errors.New("invalid output")
		}
		if err == nil {
			d.Name = name
			err = d.Validate()
		}
		if err != nil {
			err = fmt.Errorf("failed at research: %s: %w", name, err)
			break
		}
		research.Dimensions = append(research.Dimensions, d)
	}
	s := &r.Report.Steps[len(r.Report.Steps)-1]
	s.Finished = time.Now()
	if err != nil {
		s.Status, r.Report.Status = err.Error(), "failed"
		return errors.Join(err, pipeline.Write(dir, *r))
	}
	s.Status, r.Report.Status, r.Research = "passed", "done", &research
	return pipeline.Write(dir, *r)
}
