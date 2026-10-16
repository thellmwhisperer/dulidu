// Package export turns a run's research, analysis and epics into spec.md, tickets/<n>.md in ATM's contract,
// plan.json and brief-diseno.md, by code alone: the same run gives the same bytes.
package export

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	r.Spec, r.Plan, r.DesignBrief = spec(*r, nums), plan(r.Epics, nums), design(*r.Analysis)
	if err := pipeline.Write(dir, *r); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dir, "tickets"), 0o755); err != nil {
		return err
	}
	for n, t := range tickets(*r, nums) {
		if err := os.WriteFile(filepath.Join(dir, "tickets", fmt.Sprintf("%d.md", n+1)), []byte(t), 0o644); err != nil {
			return err
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

// functionalities is a's functionalities with their ids, F1, F2... in order, as the epics step numbers them.
func functionalities(a pipeline.Analysis) []string {
	fs := make([]string, len(a.Functionalities))
	for i, f := range a.Functionalities {
		fs[i] = fmt.Sprintf("F%d %s", i+1, f)
	}
	return fs
}

// spec is spec.md: research with its sources, the analysis' lists and the epics with their tickets.
func spec(r pipeline.Run, nums map[string][]int) string {
	a := r.Analysis
	parts := []string{"# Spec\n", "## Research\n"}
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
		parts = append(parts, "### "+d.Name+"\n", strings.TrimSpace(d.Text)+"\n", "Sources:\n", list(sources))
	}
	parts = append(parts, "## Personas\n", list(a.Personas), "## Functionalities\n", list(functionalities(*a)),
		"## Flows\n", list(a.Flows), "## Constraints\n", list(a.Constraints), "## Open questions\n", list(a.Questions),
		"## Epics\n")
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
