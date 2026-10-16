// Package epics plans the analysis in two phases: one call for the epic outlines and the dependencies between them,
// then one per epic for its stories. Code checks the caps, the dependencies and that a story delivers every must
// functionality; nothing is cut to fit, and epics.json is written only when every check holds.
package epics

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/prompts"
)

type outline struct {
	ID, Title, Description string
	DependsOn              []string `json:"depends_on"`
}

func blank(s string) bool { return strings.TrimSpace(s) == "" }

// outlines is answer, phase one's, as epics without stories, or why it is not a list of epics with id and title.
func outlines(answer string) (pipeline.Epics, error) {
	var got []outline
	if json.Unmarshal([]byte(answer), &got) != nil || len(got) == 0 {
		return nil, errors.New("invalid output: outlines")
	}
	es := make(pipeline.Epics, len(got))
	for i, o := range got {
		if blank(o.ID) || blank(o.Title) {
			return nil, errors.New("invalid output: an epic lacks id or title")
		}
		es[i] = pipeline.EpicOutline{ID: o.ID, Title: o.Title, Text: o.Description, DependsOn: o.DependsOn}
	}
	return es, nil
}

// Run plans r.Analysis, within r.Triage.Caps, in dir, the run folder: it adds step to r's report, running, and
// writes r; then asks ask for the outlines and, once they keep the cap and their dependencies, for each epic's
// stories. Functionalities are F1, F2... in the analysis' order. When every check holds the epics with their stories
// are r.Epics and the step passed; at the first that fails nothing more is asked, r gets no epics, the step and r
// fail, the step "failed at epics: <check>", and Run's error says that and what broke it.
func Run(dir string, r *pipeline.Run, step pipeline.Step, ask func(prompt string) (string, error)) error {
	step.Status, step.Started = "running", time.Now()
	r.Report.Status, r.Report.Steps = "running", append(r.Report.Steps, step)
	if err := pipeline.Write(dir, *r); err != nil {
		return err
	}
	if r.Logs == nil {
		r.Logs = map[string]string{}
	}
	es, err := plan(r, func(prompt string) (string, error) {
		answer, err := ask(prompt)
		r.Logs["epics"] += prompt + "\n\n" + answer + "\n\n"
		return answer, err
	})
	s := &r.Report.Steps[len(r.Report.Steps)-1]
	s.Finished = time.Now()
	if err != nil {
		check, _, _ := strings.Cut(err.Error(), ":")
		s.Status, r.Report.Status = "failed at epics: "+check, "failed"
		return errors.Join(fmt.Errorf("failed at epics: %w", err), pipeline.Write(dir, *r))
	}
	s.Status, r.Report.Status = "passed", "done"
	r.Epics = es
	return pipeline.Write(dir, *r)
}

// plan is phase one, its checks, then phase two; each error starts with the check or call that failed.
func plan(r *pipeline.Run, ask func(prompt string) (string, error)) (pipeline.Epics, error) {
	list := func(lines []string) string { return "- " + strings.Join(lines, "\n- ") }
	var all, ids, musts, mustLines []string
	for i, f := range r.Analysis.Functionalities {
		id := fmt.Sprintf("F%d", i+1)
		all = append(all, id+" "+f)
		ids = append(ids, id)
		if rest, ok := strings.CutPrefix(f, "[must] "); ok {
			musts, mustLines = append(musts, id), append(mustLines, id+" "+rest)
		}
	}
	caps := r.Triage.Caps
	data := map[string]any{"Input": r.Context, "Personas": list(r.Analysis.Personas), "Functionalities": list(all),
		"MustFunctionalities": list(mustLines), "Flows": list(r.Analysis.Flows), "Constraints": list(r.Analysis.Constraints),
		"Epics": caps.Epics, "Stories": caps.StoriesPerEpic}
	prompt, err := prompts.Render("p15-epics-outlines", data)
	var answer string
	if err == nil {
		answer, err = ask(prompt)
	}
	if err != nil {
		return nil, fmt.Errorf("outlines: %w", err)
	}
	es, err := outlines(answer)
	if err == nil {
		err = overCap(len(es), caps.Epics, "epics")
	}
	if err == nil {
		err = es.Validate() // unknown dependency or cycle: the outlines are well formed
	}
	if err != nil {
		return nil, err
	}
	for i := range es {
		if es[i].Stories, err = askStories(es[i], data, ids, ask); err == nil {
			err = overCap(len(es[i].Stories), caps.StoriesPerEpic, "stories in "+es[i].ID)
		}
		if err != nil {
			return nil, err
		}
	}
	return es, uncovered(es, musts)
}
