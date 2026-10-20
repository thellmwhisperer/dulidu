// Package export turns a run's research, analysis and epics into spec.md, tickets/<n>.md in the ten-point contract,
// plan.json and brief-diseno.md, by code alone: the same run gives the same bytes.
package export

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
)

// Write exports r, whose analysis and epics passed, into dir, its run folder: r gets its spec, plan and design brief
// and is written, then each story is tickets/<n>.md, counting from 1 in epic order, then story order.
func Write(dir string, r *pipeline.Run) error {
	if r.Analysis == nil || r.Epics == nil {
		return errors.New("export: no analysis or epics")
	}
	nums := numbers(r.Epics)
	abs, err := filepath.Abs(dir) // <repo>/.dulidu/runs/<label>
	if err != nil {
		return err
	}
	project := filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(abs))))
	r.Spec, r.Plan, r.DesignBrief = spec(*r, nums, project), plan(r.Epics, nums), design(*r.Analysis)
	if err := pipeline.Write(dir, *r); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dir, "tickets"), 0o755); err != nil {
		return err
	}
	ts := tickets(*r, nums)
	for n, t := range ts {
		if err := os.WriteFile(filepath.Join(dir, "tickets", fmt.Sprintf("%d.md", n+1)), []byte(t), 0o644); err != nil {
			return err
		}
	}
	entries, err := os.ReadDir(filepath.Join(dir, "tickets"))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		n, err := strconv.Atoi(strings.TrimSuffix(entry.Name(), ".md"))
		if err == nil && !entry.IsDir() && n > 0 && fmt.Sprintf("%d.md", n) == entry.Name() && n > len(ts) {
			if err := os.Remove(filepath.Join(dir, "tickets", entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

// doc is a Markdown document: parts, each ending in a newline, with a blank line between them.
func doc(parts ...string) string { return strings.Join(parts, "\n") }

// list is items as a Markdown list, "- None." when there are none.
func list(items []string) string {
	if len(items) == 0 {
		return "- None.\n"
	}
	return "- " + strings.Join(items, "\n- ") + "\n"
}

// functionalities is a's functionalities, each led by its id.
func functionalities(a pipeline.Analysis) []string {
	fs := make([]string, len(a.Functionalities))
	for i, f := range a.Functionalities {
		fs[i] = f.ID + " " + f.String()
	}
	return fs
}

// sections is the title spec.md gives each research dimension, as the prototype's report.
var sections = map[string]string{"business": "Business Context", "users": "Users & Personas",
	"market": "Market & Competition", "technical": "Technical Landscape", "project": "Project Scope"}

// spec is spec.md, as the prototype's report: the project's name, research with its sources, the analysis' personas,
// functionality table, flows, constraints and questions, and the epics with their tickets.
func spec(r pipeline.Run, nums map[string][]int, project string) string {
	a := r.Analysis
	parts := []string{"# " + project + "\n", "## Research\n"}
	var dims []pipeline.Dimension
	if r.Research != nil {
		dims = r.Research.Dimensions
	}
	if len(dims) == 0 {
		parts = append(parts, list(nil))
	}
	for _, d := range dims {
		sources := make([]string, len(d.Sources))
		for i, s := range d.Sources {
			sources[i] = fmt.Sprintf("[%s](%s): %s", s.Title, s.URL, s.Supports)
		}
		parts = append(parts, "### "+cmp.Or(sections[d.Name], d.Name)+"\n", strings.TrimSpace(d.Text)+"\n", "Sources:\n", list(sources))
	}
	parts = append(parts, "## Analysis\n", "### Personas\n")
	if len(a.Personas) == 0 {
		parts = append(parts, list(nil))
	}
	for _, p := range a.Personas {
		parts = append(parts, "#### "+p.Name+": "+p.Role+"\n", "**Goals:**\n", list(p.Goals), "**Pain Points:**\n", list(p.PainPoints))
	}
	table := "| ID | Priority | Name | Description |\n|----|----------|------|-------------|\n"
	for _, f := range a.Functionalities {
		table += "| " + f.ID + " | " + f.Priority + " | " + f.Name + " | " + f.Description + " |\n"
	}
	if len(a.Functionalities) == 0 {
		table = list(nil)
	}
	parts = append(parts, "### Functionalities\n", table, "### User Flows\n")
	if len(a.Flows) == 0 {
		parts = append(parts, list(nil))
	}
	for _, fl := range a.Flows {
		parts = append(parts, "#### "+fl.Name+"\n", strings.Join(fl.Steps, " → ")+"\n")
	}
	parts = append(parts, "### Constraints\n", list(a.Constraints), "### Open Questions\n", list(a.Questions), "## Epics\n")
	for _, e := range r.Epics {
		deps := "none"
		if len(e.DependsOn) > 0 {
			deps = strings.Join(e.DependsOn, ", ")
		}
		stories := make([]string, len(e.Stories))
		for i, s := range e.Stories {
			stories[i] = fmt.Sprintf("#%d %s %s", nums[e.ID][i], s.ID, s.Title)
		}
		parts = append(parts, "### "+e.ID+" "+e.Title+"\n")
		if strings.TrimSpace(e.Text) != "" {
			parts = append(parts, strings.TrimSpace(e.Text)+"\n")
		}
		parts = append(parts, "Depends on: "+deps+".\n", list(stories))
	}
	return doc(parts...)
}
