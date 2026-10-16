package pipeline

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"strings"
)

// need is nil when ok holds, else an error saying msg.
func need(ok bool, msg string) error {
	if ok {
		return nil
	}
	return errors.New(msg)
}

func blank(s string) bool { return strings.TrimSpace(s) == "" }

// filled is a list with at least one item and no blank one.
func filled(list []string) bool { return len(list) > 0 && !slices.ContainsFunc(list, blank) }

// plain is a name that stays one file inside its folder.
func plain(name string) bool { return filepath.Base(name) == name && filepath.IsLocal(name) }

func distinctNames[V any](items map[string]V, reserved ...string) bool {
	seen := append([]string(nil), reserved...)
	for name := range items {
		if slices.ContainsFunc(seen, func(other string) bool { return strings.EqualFold(name, other) }) {
			return false
		}
		seen = append(seen, name)
	}
	return true
}

func (b Brief) Validate() error { return need(!blank(b.Text), "brief: no text") }

func (c InputContext) Validate() error {
	errs := []error{c.Brief.Validate(), need(distinctNames(c.Files, briefFile), "input: colliding names")}
	for name := range c.Files {
		errs = append(errs, need(plain(name) && name != briefFile, "input: bad file name "+name))
	}
	return errors.Join(errs...)
}

func (t Triage) Validate() error {
	errs := []error{need(slices.Contains([]string{"simple", "moderate", "complex"}, t.Complexity), "triage: bad complexity"),
		need(t.Caps.Epics > 0 && t.Caps.StoriesPerEpic > 0, "triage: caps not positive"),
		need(slices.Contains([]string{"new_product", "existing_codebase", "migration", "other"}, t.ProjectType),
			"triage: bad project type"),
		need(t.ResearchDimensions != nil && t.ContextPresent != nil && t.MissingQuestions != nil,
			"triage: missing list"),
		need(!slices.ContainsFunc(t.ContextPresent, blank) && !slices.ContainsFunc(t.MissingQuestions, blank),
			"triage: a blank item")}
	for _, d := range t.ResearchDimensions {
		errs = append(errs, need(slices.Contains([]string{"business", "users", "market", "technical", "project"}, d),
			"triage: bad research dimension "+d))
	}
	return errors.Join(errs...)
}

func (r Research) Validate() error {
	errs := []error{need(len(r.Dimensions) > 0, "research: no dimensions")}
	for _, d := range r.Dimensions {
		errs = append(errs, need(!blank(d.Name), "research: a dimension has no name"),
			need(!blank(d.Text), "research "+d.Name+": no text"),
			need(filled(d.Sources), "research "+d.Name+": no sources"))
	}
	return errors.Join(errs...)
}

func (a Analysis) Validate() error {
	return errors.Join(need(filled(a.Personas), "analysis: no personas"),
		need(filled(a.Functionalities), "analysis: no functionalities"),
		need(filled(a.Flows), "analysis: no flows"))
}

func (e EpicOutline) Validate() error {
	return errors.Join(need(!blank(e.ID), "epic: no id"), need(!blank(e.Title), "epic "+e.ID+": no title"))
}

func (es Epics) Validate() error {
	errs := []error{need(len(es) > 0, "epics: none")}
	for _, e := range es {
		errs = append(errs, e.Validate())
	}
	return errors.Join(errs...)
}

func (s Story) Validate() error {
	return errors.Join(need(!blank(s.Epic), "story: no epic"), need(!blank(s.Title), "story: no title"),
		need(filled(s.Acceptance), "story "+s.Title+": no acceptance"))
}

func (r Report) Validate() error {
	errs := []error{need(!blank(r.Status), "report: no status"), need(!r.Started.IsZero(), "report: no start")}
	for _, s := range r.Steps {
		errs = append(errs, need(plain(s.Name), "report: bad step name "+s.Name),
			need(!blank(s.Status), "report "+s.Name+": no status"),
			need(!blank(s.Harness), "report "+s.Name+": no harness"),
			need(!s.Started.IsZero(), "report "+s.Name+": no start"),
			need(s.Finished.IsZero() || !s.Finished.Before(s.Started), "report "+s.Name+": finished before start"))
	}
	return errors.Join(errs...)
}

// Validate checks every artefact the run has.
func (r Run) Validate() error {
	errs := []error{r.Report.Validate()}
	if r.Input != nil {
		errs = append(errs, r.Input.Validate())
	}
	if r.Triage != nil {
		errs = append(errs, r.Triage.Validate())
	}
	if r.Research != nil {
		errs = append(errs, r.Research.Validate())
	}
	if r.Analysis != nil {
		errs = append(errs, r.Analysis.Validate())
	}
	if r.Epics != nil {
		errs = append(errs, r.Epics.Validate())
	}
	for id, s := range r.Tickets {
		errs = append(errs, need(plain(id), "tickets: bad id "+id), s.Validate())
	}
	errs = append(errs, need(distinctNames(r.Tickets), "tickets: colliding ids"))
	for step := range r.Logs {
		errs = append(errs, need(plain(step), "agent: bad step "+step))
	}
	errs = append(errs, need(distinctNames(r.Logs), "agent: colliding steps"))
	if r.Plan != nil {
		errs = append(errs, need(json.Valid(r.Plan), "plan: not JSON"))
	}
	return errors.Join(errs...)
}
