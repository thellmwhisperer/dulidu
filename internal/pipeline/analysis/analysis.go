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

// personaLists is a persona's "### " sections by lower-cased name, each with the label its line gives it.
var personaLists = map[string]string{"goals": "Goals", "pain points": "Pain points"}

// lists is each list in the order it is asked: its prompt, the key later prompts read it by and how its answer's
// blocks become lines.
var lists = []struct {
	name, template, key string
	parse               func(blocks []text.Block) ([]string, error)
}{
	{"personas", "p09-analysis-personas", "Personas", func(bs []text.Block) ([]string, error) {
		return each(bs[1:], func(b text.Block) (string, error) {
			s := b.Name + " (" + b.Field("role") + ")"
			for _, sub := range b.Sections {
				if label := personaLists[strings.ToLower(sub.Name)]; label != "" && len(sub.Items) > 0 {
					s += ". " + label + ": " + strings.Join(sub.Items, "; ")
				}
			}
			return s, need(!blank(b.Name) && !blank(b.Field("role")), "a persona lacks name or role")
		})
	}},
	{"functionalities", "p10-analysis-functionalities", "Functionalities", func(bs []text.Block) ([]string, error) {
		return each(bs[1:], func(b text.Block) (string, error) {
			name, priority := id.ReplaceAllString(b.Name, ""), b.Field("priority")
			var lines []string
			for _, line := range b.RawLines {
				line = strings.TrimSpace(line)
				key, _, field := strings.Cut(line, ":")
				if line != "" && !(field && strings.EqualFold(key, "priority")) {
					lines = append(lines, line)
				}
			}
			description := strings.Join(lines, " ")
			return "[" + priority + "] " + name + ": " + description, errors.Join(
				need(!blank(name) && !blank(description), "a functionality lacks name or description"),
				need(slices.Contains([]string{"must", "should", "could"}, priority),
					"functionality "+name+": priority not must, should or could"))
		})
	}},
	{"flows", "p11-analysis-flows", "Flows", func(bs []text.Block) ([]string, error) {
		return each(bs[1:], func(b text.Block) (string, error) {
			name := id.ReplaceAllString(b.Name, "")
			return name + ": " + strings.Join(b.Items, " → "), need(!blank(name) && len(b.Items) > 0, "a flow lacks name or steps")
		})
	}},
	{"constraints", "p12-analysis-constraints", "Constraints", items},
	{"questions", "p13-analysis-questions", "Questions", items},
}

func blank(s string) bool { return strings.TrimSpace(s) == "" }

// need is nil when ok holds, else an error saying msg.
func need(ok bool, msg string) error {
	if ok {
		return nil
	}
	return errors.New(msg)
}

// items is the lead block's "- item" list, at least one.
func items(bs []text.Block) ([]string, error) {
	if len(bs[0].Items) == 0 {
		return nil, errors.New("empty")
	}
	return bs[0].Items, nil
}

// each is blocks, at least one, each made a line by line, which says what is wrong with it.
func each(blocks []text.Block, line func(text.Block) (string, error)) ([]string, error) {
	if len(blocks) == 0 {
		return nil, errors.New("empty")
	}
	lines := make([]string, len(blocks))
	for i, b := range blocks {
		var err error
		if lines[i], err = line(b); err != nil {
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
			got[l.name], err = l.parse(text.Parse(answer))
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
