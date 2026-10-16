package pipeline

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// briefFile is the brief's file in input/, beside the files given with it.
const briefFile = "brief.md"

// Write puts r in dir, the run folder .dulidu/runs/<label>/, after checking it: report.json and every
// artefact r has, each in its file. It leaves alone the files r has no artefact for.
func Write(dir string, r Run) error {
	if err := r.Validate(); err != nil {
		return err
	}
	files := map[string][]byte{}
	put := func(name string, v any) {
		b, _ := json.MarshalIndent(v, "", "  ") // the types hold nothing json cannot marshal
		files[name] = append(b, '\n')
	}
	text := func(name, s string) {
		if s != "" {
			files[name] = []byte(s)
		}
	}
	put("report.json", r.Report)
	if r.Input != nil {
		files[filepath.Join("input", briefFile)] = []byte(r.Input.Brief.Text)
		for name, s := range r.Input.Files {
			files[filepath.Join("input", name)] = []byte(s)
		}
	}
	text("contexto.md", r.Context)
	if r.Triage != nil {
		put("triage.json", r.Triage)
	}
	if r.Research != nil {
		put("research.json", r.Research)
	}
	if r.Analysis != nil {
		put("analysis.json", r.Analysis)
	}
	if r.Epics != nil {
		put("epics.json", r.Epics)
	}
	for id, s := range r.Tickets {
		put(filepath.Join("tickets", id+".json"), s)
	}
	text("spec.md", r.Spec)
	text("plan.json", string(r.Plan))
	text("brief-diseno.md", r.DesignBrief)
	for step, s := range r.Logs {
		files[filepath.Join("agent", step+".log")] = []byte(s)
	}
	for name, b := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, b, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Read is the run in dir as Write left it. Only report.json must be there.
func Read(dir string) (Run, error) {
	var r Run
	err := decode(filepath.Join(dir, "report.json"), &r.Report)
	for name, v := range map[string]any{"triage.json": &r.Triage, "research.json": &r.Research,
		"analysis.json": &r.Analysis, "epics.json": &r.Epics} {
		if e := decode(filepath.Join(dir, name), v); !errors.Is(e, fs.ErrNotExist) {
			err = errors.Join(err, e)
		}
	}
	for name, s := range map[string]*string{"contexto.md": &r.Context, "spec.md": &r.Spec,
		"brief-diseno.md": &r.DesignBrief} {
		b, e := os.ReadFile(filepath.Join(dir, name))
		if !errors.Is(e, fs.ErrNotExist) {
			err = errors.Join(err, e)
		}
		*s = string(b)
	}
	plan, e := os.ReadFile(filepath.Join(dir, "plan.json"))
	if !errors.Is(e, fs.ErrNotExist) {
		err = errors.Join(err, e)
	}
	r.Plan = plan

	input, e := folder(dir, "input", "")
	err = errors.Join(err, e)
	if input != nil {
		r.Input = &InputContext{Brief: Brief{Text: input[briefFile]}}
		delete(input, briefFile)
		if len(input) > 0 {
			r.Input.Files = input
		}
	}
	tickets, e := folder(dir, "tickets", ".json")
	err = errors.Join(err, e)
	for id, s := range tickets {
		var story Story
		if e := json.Unmarshal([]byte(s), &story); e != nil {
			err = errors.Join(err, fmt.Errorf("tickets/%s.json: %w", id, e))
		}
		if r.Tickets == nil {
			r.Tickets = map[string]Story{}
		}
		r.Tickets[id] = story
	}
	r.Logs, e = folder(dir, "agent", ".log")
	return r, errors.Join(err, e)
}

// decode reads path's JSON into v.
func decode(path string, v any) error {
	b, err := os.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(b, v)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return nil
}

// folder is the files in dir/sub ending in ext, by name without ext; nil when there are none.
func folder(dir, sub, ext string) (map[string]string, error) {
	entries, err := os.ReadDir(filepath.Join(dir, sub))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	var files map[string]string
	for _, entry := range entries {
		name, ok := strings.CutSuffix(entry.Name(), ext)
		if !ok || entry.IsDir() {
			continue
		}
		b, e := os.ReadFile(filepath.Join(dir, sub, entry.Name()))
		err = errors.Join(err, e)
		if files == nil {
			files = map[string]string{}
		}
		files[name] = string(b)
	}
	return files, err
}
