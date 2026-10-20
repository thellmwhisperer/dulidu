// Package epics plans the analysis in two phases: one call for the epic outlines and the dependencies between them,
// then one per epic for its stories. Code checks the caps, the dependencies and that a story delivers every must
// functionality; past a cap the first epics or stories are kept, and epics.json is written only when every check holds.
package epics

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/prompts"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/text"
)

func blank(s string) bool { return strings.TrimSpace(s) == "" }

// heading separates a block's id from its title.
func heading(b text.Block) (string, string) {
	parts := strings.Fields(b.Name)
	if len(parts) == 0 {
		return "", ""
	}
	return parts[0], strings.TrimSpace(strings.TrimPrefix(b.Name, parts[0]))
}

// references reads a comma-separated list; none is an explicitly empty list.
func references(value string) []string {
	if strings.EqualFold(strings.TrimSpace(value), "none") {
		return []string{}
	}
	ids := strings.Split(value, ",")
	for i := range ids {
		ids[i] = strings.TrimSpace(ids[i])
	}
	return ids
}

// description keeps prose, including colon-bearing lines, before the first sub-section, except the named field.
func description(b text.Block, field string) string {
	var lines []string
	for _, line := range b.RawLines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "### ") {
			break
		}
		key, _, ok := strings.Cut(line, ":")
		if !ok || !strings.EqualFold(key, field) {
			lines = append(lines, line)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// outlines reads phase one's blocks as epics without stories.
func outlines(answer string) (pipeline.Epics, error) {
	blocks := text.Parse(answer)[1:]
	if len(blocks) == 0 {
		return nil, errors.New("outlines: no epic blocks")
	}
	es := make(pipeline.Epics, len(blocks))
	for i, b := range blocks {
		id, title := heading(b)
		if blank(id) || blank(title) {
			return nil, fmt.Errorf("outlines: epic %s lacks id or title", id)
		}
		var dependencies []string
		if value := b.Field("Depends on"); value != "" {
			dependencies = references(value)
		}
		es[i] = pipeline.EpicOutline{ID: id, Title: title, Text: description(b, "Depends on"), DependsOn: dependencies}
	}
	return es, nil
}

// dropBadDependencies adds edges in outline order, recording each rejected edge in notes.
func dropBadDependencies(es pipeline.Epics, notes *[]string) error {
	dependencies := make([][]string, len(es))
	for i := range es {
		dependencies[i], es[i].DependsOn = es[i].DependsOn, es[i].DependsOn[:0:0]
	}
	if err := es.Validate(); err != nil { // Duplicate ids still fail before any repair.
		return err
	}
	// ponytail: validate each edge for these small epic graphs; use incremental reachability if they grow large.
	for i := range es {
		for _, id := range dependencies[i] {
			es[i].DependsOn = append(es[i].DependsOn, id)
			if err := es.Validate(); err != nil {
				es[i].DependsOn = es[i].DependsOn[:len(es[i].DependsOn)-1]
				note := "dropped dependency " + es[i].ID + " on " + id
				if strings.HasPrefix(err.Error(), "cycle:") {
					note += ": cycle"
				}
				*notes = append(*notes, note)
			}
		}
	}
	return nil
}

// Run plans r.Analysis, within r.Triage.Epics and, when positive, r.Triage.StoriesPerEpic, in dir, the run folder: it
// adds step to r's report, running, and writes r; then asks ask for the outlines and, once their dependencies hold, for
// each kept epic's stories. Past a cap it keeps the first, drops dependencies on cut epics and says so in the step's
// note. Unknown dependencies and edges closing a cycle are dropped with a note. Functionalities are F1, F2... in the
// analysis' order. When every check holds the epics with their stories are
// r.Epics and the step passed; at the first that fails nothing more is asked, r gets no epics, the step and r
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
	var notes []string
	es, err := plan(r, &notes, func(prompt string) (string, error) {
		answer, err := ask(prompt)
		r.Logs["epics"] += prompt + "\n\n" + answer + "\n\n"
		return answer, err
	})
	s := &r.Report.Steps[len(r.Report.Steps)-1]
	s.Finished, s.Note = time.Now(), strings.Join(notes, "; ")
	if err != nil {
		check, _, _ := strings.Cut(err.Error(), ":")
		s.Status, r.Report.Status = "failed at epics: "+check, "failed"
		return errors.Join(fmt.Errorf("failed at epics: %w", err), pipeline.Write(dir, *r))
	}
	s.Status, r.Report.Status = "passed", "done"
	r.Epics = es
	return pipeline.Write(dir, *r)
}

// plan is phase one, its checks, then phase two, adding each cut to notes; each error starts with the check or call
// that failed.
func plan(r *pipeline.Run, notes *[]string, ask func(prompt string) (string, error)) (pipeline.Epics, error) {
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
	data := map[string]any{"Input": r.Context, "Personas": list(r.Analysis.Personas), "Functionalities": list(all),
		"MustFunctionalities": list(mustLines), "Flows": list(r.Analysis.Flows), "Constraints": list(r.Analysis.Constraints),
		"Epics": r.Triage.Epics, "Stories": r.Triage.StoriesPerEpic}
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
		err = dropBadDependencies(es, notes)
	}
	if err != nil {
		return nil, err
	}
	es = trim(es, r.Triage.Epics, "epics", notes)
	for i := range es {
		es[i].DependsOn = slices.DeleteFunc(es[i].DependsOn, func(id string) bool {
			return !slices.ContainsFunc(es, func(e pipeline.EpicOutline) bool { return e.ID == id })
		})
		if es[i].Stories, err = askStories(es[i], data, ids, ask); err != nil {
			return nil, err
		}
		es[i].Stories = trim(es[i].Stories, r.Triage.StoriesPerEpic, "stories in "+es[i].ID, notes)
	}
	return es, uncovered(es, musts)
}
