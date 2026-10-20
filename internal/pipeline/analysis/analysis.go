// Package analysis takes contexto.md and research.json apart in five calls, one per list, each fed the lists before
// it, and writes the five to analysis.json.
package analysis

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/prompts"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/text"
)

// id is the "F1 " or "FL1 " a functionality's or flow's heading starts with.
var id = regexp.MustCompile(`^[A-Z]{1,2}\d+\b[.:]?\s*`)

// idOf is b's heading split into the id it starts with, possibly none, and the name after it.
func idOf(b text.Block) (string, string) {
	lead := id.FindString(b.Name)
	return strings.TrimRight(lead, ".: \t"), strings.TrimPrefix(b.Name, lead)
}

// lists is each list in the order it is asked: its prompt, the key later prompts read it by and how its answer's
// blocks go into a, as the lines later prompts read, adding to notes what code changed.
var lists = []struct {
	name, template, key string
	parse               func(a *pipeline.Analysis, blocks []text.Block, notes *[]string) ([]string, error)
}{
	{"personas", "p09-analysis-personas", "Personas", func(a *pipeline.Analysis, bs []text.Block, _ *[]string) ([]string, error) {
		var err error
		a.Personas, err = each(bs[1:], func(_ int, b text.Block) (pipeline.Persona, error) {
			p := pipeline.Persona{Name: b.Name, Role: b.Field("role"), Goals: []string{}, PainPoints: []string{}}
			for _, sub := range b.Sections {
				switch strings.ToLower(sub.Name) {
				case "goals":
					p.Goals = append(p.Goals, sub.Items...)
				case "pain points":
					p.PainPoints = append(p.PainPoints, sub.Items...)
				}
			}
			return p, need(!blank(p.Name) && !blank(p.Role), "a persona lacks name or role")
		})
		return pipeline.Lines(a.Personas), err
	}},
	{"functionalities", "p10-analysis-functionalities", "Functionalities", func(a *pipeline.Analysis, bs []text.Block, notes *[]string) ([]string, error) {
		used := map[string]bool{}
		for _, b := range bs[1:] {
			id, _ := idOf(b)
			used[id] = true
		}
		var err error
		a.Functionalities, err = each(bs[1:], func(i int, b text.Block) (pipeline.Functionality, error) {
			f := pipeline.Functionality{Priority: b.Field("priority")}
			f.ID, f.Name = idOf(b)
			var lines []string
			for _, line := range b.RawLines {
				line = strings.TrimSpace(line)
				key, _, field := strings.Cut(line, ":")
				if line != "" && !(field && strings.EqualFold(key, "priority")) {
					lines = append(lines, line)
				}
			}
			f.Description = strings.Join(lines, " ")
			if f.ID == "" {
				for n := i + 1; f.ID == "" || used[f.ID]; n++ {
					f.ID = fmt.Sprintf("F%d", n)
				}
				used[f.ID] = true
				*notes = append(*notes, "functionality "+f.Name+" had no id: "+f.ID)
			}
			return f, errors.Join(
				need(!blank(f.Name) && !blank(f.Description), "a functionality lacks name or description"),
				need(slices.Contains([]string{"must", "should", "could"}, f.Priority),
					"functionality "+f.Name+": priority not must, should or could"))
		})
		return pipeline.Lines(a.Functionalities), err
	}},
	{"flows", "p11-analysis-flows", "Flows", func(a *pipeline.Analysis, bs []text.Block, _ *[]string) ([]string, error) {
		var err error
		a.Flows, err = each(bs[1:], func(_ int, b text.Block) (pipeline.Flow, error) {
			f := pipeline.Flow{Steps: b.Items}
			f.ID, f.Name = idOf(b)
			return f, need(!blank(f.Name) && len(f.Steps) > 0, "a flow lacks name or steps")
		})
		return pipeline.Lines(a.Flows), err
	}},
	{"constraints", "p12-analysis-constraints", "Constraints", func(a *pipeline.Analysis, bs []text.Block, _ *[]string) ([]string, error) {
		a.Constraints = items(bs)
		return a.Constraints, nil
	}},
	{"questions", "p13-analysis-questions", "Questions", func(a *pipeline.Analysis, bs []text.Block, _ *[]string) ([]string, error) {
		a.Questions = items(bs)
		return a.Questions, nil
	}},
}

func blank(s string) bool { return strings.TrimSpace(s) == "" }

// need is nil when ok holds, else an error saying msg.
func need(ok bool, msg string) error {
	if ok {
		return nil
	}
	return errors.New(msg)
}

// items is the lead block's "- item" list, possibly empty.
func items(bs []text.Block) []string {
	return append([]string{}, bs[0].Items...)
}

// each is blocks, possibly empty, each made an item by item, given its position, which says what is wrong with it.
func each[T any](blocks []text.Block, item func(int, text.Block) (T, error)) ([]T, error) {
	items := make([]T, len(blocks))
	for i, b := range blocks {
		var err error
		if items[i], err = item(i, b); err != nil {
			return nil, err
		}
	}
	return items, nil
}

// Run analyses r.Context and r.Research in dir, the run folder: it adds step to r's report, running, and writes r;
// then asks ask once per list, in lists' order, with its prompt on contexto.md, research.json and the lists before it.
// When parsing every answer yields valid items, possibly none, the five are r.Analysis and the step passed. At the first
// parsing or agent error, no later list is asked, r gets no analysis, the step and r fail, and Run's error names the list.
// A functionality without an id gets the first unused F<n> from its position onward; the step's note says so.
func Run(dir string, r *pipeline.Run, step pipeline.Step, ask func(prompt string) (string, error)) error {
	step.Status, step.Started = "running", time.Now()
	r.Report.Status, r.Report.Steps = "running", append(r.Report.Steps, step)
	if err := pipeline.Write(dir, *r); err != nil {
		return err
	}
	if r.Logs == nil {
		r.Logs = map[string]string{}
	}
	research, _ := json.MarshalIndent(r.Research, "", "  ") // as Write puts research.json
	data := map[string]any{"Input": r.Context, "Research": string(research)}
	var a pipeline.Analysis
	var notes []string
	var failed string
	var err error
	for _, l := range lists {
		var prompt, answer string
		var lines []string
		prompt, err = prompts.Render(l.template, data)
		if err == nil {
			answer, err = ask(prompt)
			r.Logs["analysis"] += prompt + "\n\n" + answer + "\n\n"
		}
		if err == nil {
			lines, err = l.parse(&a, text.Parse(answer), &notes)
		}
		if err != nil {
			failed = "failed at analysis: " + l.name
			err = fmt.Errorf("%s: %w", failed, err)
			break
		}
		data[l.key] = "- " + strings.Join(lines, "\n- ")
	}
	s := &r.Report.Steps[len(r.Report.Steps)-1]
	s.Finished, s.Note = time.Now(), strings.Join(notes, "; ")
	if err != nil {
		s.Status, r.Report.Status = failed, "failed"
		return errors.Join(err, pipeline.Write(dir, *r))
	}
	s.Status, r.Report.Status = "passed", "done"
	r.Analysis = &a
	return pipeline.Write(dir, *r)
}
