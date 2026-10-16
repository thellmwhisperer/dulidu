// Package analysis takes contexto.md and research.json apart in five calls, one per list, each fed the lists before
// it, and writes the five to analysis.json.
package analysis

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/prompts"
)

type persona struct {
	Name, Role        string
	Goals, PainPoints []string
}

type functionality struct{ Name, Description, Priority string }

type flow struct {
	Name  string
	Steps []string
}

// lists is each list in the order it is asked: its prompt, the key later prompts read it by and how its answer
// becomes lines.
var lists = []struct {
	name, template, key string
	parse               func(answer string) ([]string, error)
}{
	{"personas", "p09-analysis-personas", "Personas", func(a string) ([]string, error) {
		return each(a, func(p persona) (string, error) {
			s := p.Name + " (" + p.Role + ")"
			if len(p.Goals) > 0 {
				s += ". Goals: " + strings.Join(p.Goals, "; ")
			}
			if len(p.PainPoints) > 0 {
				s += ". Pain points: " + strings.Join(p.PainPoints, "; ")
			}
			return s, errors.Join(
				need(!blank(p.Name) && !blank(p.Role), "a persona lacks name or role"),
				need(!slices.ContainsFunc(p.Goals, blank), "a persona has a blank goal"),
				need(!slices.ContainsFunc(p.PainPoints, blank), "a persona has a blank pain point"))
		})
	}},
	{"functionalities", "p10-analysis-functionalities", "Functionalities", func(a string) ([]string, error) {
		return each(a, func(f functionality) (string, error) {
			return "[" + f.Priority + "] " + f.Name + ": " + f.Description, errors.Join(
				need(!blank(f.Name) && !blank(f.Description), "a functionality lacks name or description"),
				need(slices.Contains([]string{"must", "should", "could"}, f.Priority),
					"functionality "+f.Name+": priority not must, should or could"))
		})
	}},
	{"flows", "p11-analysis-flows", "Flows", func(a string) ([]string, error) {
		return each(a, func(f flow) (string, error) {
			return f.Name + ": " + strings.Join(f.Steps, " → "),
				need(!blank(f.Name) && len(f.Steps) > 0 && !slices.ContainsFunc(f.Steps, blank), "a flow lacks name or steps")
		})
	}},
	{"constraints", "p12-analysis-constraints", "Constraints", text},
	{"questions", "p13-analysis-questions", "Questions", text},
}

func blank(s string) bool { return strings.TrimSpace(s) == "" }

// need is nil when ok holds, else an error saying msg.
func need(ok bool, msg string) error {
	if ok {
		return nil
	}
	return errors.New(msg)
}

func text(answer string) ([]string, error) {
	return each(answer, func(s string) (string, error) { return s, need(!blank(s), "a blank item") })
}

// each is answer, a JSON array with at least one T, each made a line by line, which says what is wrong with it.
func each[T any](answer string, line func(T) (string, error)) ([]string, error) {
	var items []T
	if json.Unmarshal([]byte(answer), &items) != nil {
		return nil, errors.New("invalid output")
	}
	if len(items) == 0 {
		return nil, errors.New("empty")
	}
	lines := make([]string, len(items))
	for i, item := range items {
		var err error
		if lines[i], err = line(item); err != nil {
			return nil, err
		}
	}
	return lines, nil
}

// Run analyses r.Context and r.Research in dir, the run folder: it adds step to r's report, running, and writes r;
// then asks ask once per list, in lists' order, with its prompt on contexto.md, research.json and the lists before it.
// When every answer is a list of at least one valid item the five are r.Analysis and the step passed; at the first
// that is not, no later one is asked, r gets no analysis, the step and r fail, the step "failed at analysis: <list>",
// and Run's error says that and why.
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
	got := map[string][]string{}
	var failed string
	var err error
	for _, l := range lists {
		var prompt, answer string
		prompt, err = prompts.Render(l.template, data)
		if err == nil {
			answer, err = ask(prompt)
			r.Logs["analysis"] += prompt + "\n\n" + answer + "\n\n"
		}
		if err == nil {
			got[l.name], err = l.parse(answer)
		}
		if err != nil {
			failed = "failed at analysis: " + l.name
			err = fmt.Errorf("%s: %w", failed, err)
			break
		}
		data[l.key] = "- " + strings.Join(got[l.name], "\n- ")
	}
	s := &r.Report.Steps[len(r.Report.Steps)-1]
	s.Finished = time.Now()
	if err != nil {
		s.Status, r.Report.Status = failed, "failed"
		return errors.Join(err, pipeline.Write(dir, *r))
	}
	s.Status, r.Report.Status = "passed", "done"
	r.Analysis = &pipeline.Analysis{Personas: got["personas"], Functionalities: got["functionalities"],
		Flows: got["flows"], Constraints: got["constraints"], Questions: got["questions"]}
	return pipeline.Write(dir, *r)
}
