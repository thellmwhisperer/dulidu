// Package design draws the product's screens in two phases: one call for the screens and their purpose, from the
// analysis' personas, functionalities and flows, then one per screen for its wireframe, components, navigation, states
// and accessibility. It writes screens.json.
package design

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/prompts"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/text"
)

// Run draws r.Analysis' screens in dir, the run folder: it adds step to r's report, running, and writes r; then asks
// ask for the screens and, for each, its skeleton, logging each call in r.Logs, which the earlier steps made. When
// every answer holds the screens are r.Screens and the step passed, noted "no screens" when there are none; at the
// first call or answer that fails nothing more is asked, the step and r fail, the step "failed at design: <screen or
// screens>", and Run's error says that and what broke it.
func Run(dir string, r *pipeline.Run, step pipeline.Step, ask func(prompt string) (string, error)) error {
	step.Status, step.Started = "running", time.Now()
	r.Report.Status, r.Report.Steps = "running", append(r.Report.Steps, step)
	if err := pipeline.Write(dir, *r); err != nil {
		return err
	}
	screens, err := draw(*r.Analysis, func(prompt string) (string, error) {
		answer, err := ask(prompt)
		r.Logs["design"] += prompt + "\n\n" + answer + "\n\n"
		return answer, err
	})
	s := &r.Report.Steps[len(r.Report.Steps)-1]
	s.Finished = time.Now()
	if err != nil {
		check, _, _ := strings.Cut(err.Error(), ":")
		s.Status, r.Report.Status = "failed at design: "+check, "failed"
		return errors.Join(fmt.Errorf("failed at design: %w", err), pipeline.Write(dir, *r))
	}
	if len(screens) == 0 {
		s.Note = "no screens"
	}
	s.Status, r.Report.Status, r.Screens = "passed", "done", screens
	return pipeline.Write(dir, *r)
}

// prose is b's lines as written, fields and items included.
func prose(b text.Block) string { return strings.TrimSpace(strings.Join(b.RawLines, "\n")) }

// draw is phase one, then phase two per screen; each error starts with the screen, or "screens", that failed.
func draw(a pipeline.Analysis, ask func(prompt string) (string, error)) ([]pipeline.Screen, error) {
	list := func(lines []string) string { return "- " + strings.Join(lines, "\n- ") }
	data := map[string]any{"Personas": list(pipeline.Lines(a.Personas)),
		"Functionalities": list(pipeline.Lines(a.Functionalities)), "Flows": list(pipeline.Lines(a.Flows))}
	prompt, err := prompts.Render("p17-design-screens", data)
	var answer string
	if err == nil {
		answer, err = ask(prompt)
	}
	if err != nil {
		return nil, fmt.Errorf("screens: %w", err)
	}
	screens, names := []pipeline.Screen{}, []string{}
	for _, b := range text.Parse(answer)[1:] {
		screens = append(screens, pipeline.Screen{Name: b.Name, Purpose: prose(b)})
		names = append(names, b.Name+": "+prose(b))
	}
	data["Screens"] = list(names)
	for i := range screens {
		data["Screen"] = screens[i]
		prompt, err := prompts.Render("p18-design-screen", data)
		if err == nil {
			answer, err = ask(prompt)
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", screens[i].Name, err)
		}
		blocks := text.Parse(answer)[1:]
		if len(blocks) == 0 {
			return nil, fmt.Errorf("%s: no screen block", screens[i].Name)
		}
		for _, section := range blocks[0].Sections {
			switch s := &screens[i]; strings.ToLower(section.Name) {
			case "wireframe":
				s.Wireframe = text.Unfence(prose(section))
			case "components":
				s.Components = section.Items
			case "navigation":
				s.Navigation = prose(section)
			case "states":
				s.States = section.Items
			case "accessibility":
				s.Accessibility = prose(section)
			}
		}
	}
	return screens, nil
}
