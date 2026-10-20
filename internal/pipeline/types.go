// Package pipeline holds the artefacts that travel between steps and the run folder that keeps them.
package pipeline

import (
	"encoding/json"
	"time"
)

// Brief is what a run is asked, in the words it was asked.
type Brief struct {
	Text string `json:"text"`
}

// InputContext is what a run is given: its brief and the files that came with it, by name.
type InputContext struct {
	Brief Brief
	Files map[string]string
}

// Triage is triage.json, the first read of contexto.md: how complex the project is, the epics that caps, why, and the
// dimensions to research. StoriesPerEpic is max_stories_per_epic, 0 for none, for the epics step: not triage's to pick,
// it is not in triage.json.
type Triage struct {
	Complexity         string   `json:"complexity"` // simple, moderate or complex
	Epics              int      `json:"epics"`
	Reasoning          string   `json:"reasoning"`
	ResearchDimensions []string `json:"research_dimensions"` // of business, users, market, technical, project
	StoriesPerEpic     int      `json:"-"`
}

// Research is research.json: one entry per dimension triage switched on, none when it switched on none.
type Research struct {
	Dimensions []Dimension `json:"dimensions"`
}

// Dimension is one researched dimension: what was found and where.
type Dimension struct {
	Name    string   `json:"name"`
	Text    string   `json:"text"`
	Sources []Source `json:"sources"`
}

// Source is where a dimension's text comes from, as the model cites it: unfetched and unchecked.
type Source struct {
	URL      string `json:"url"`
	Title    string `json:"title"`
	Supports string `json:"supports"` // the claim of the text it backs
}

// Analysis is the brief taken apart.
type Analysis struct {
	Personas        []string `json:"personas"`
	Functionalities []string `json:"functionalities"`
	Flows           []string `json:"flows"`
	Constraints     []string `json:"constraints"`
	Questions       []string `json:"questions"`
}

// EpicOutline is one epic: its outline, the epics it depends on by id, and its stories once they are asked.
type EpicOutline struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Text      string   `json:"text"`
	DependsOn []string `json:"depends_on"`
	Stories   []Story  `json:"stories"`
}

// Epics is epics.json: every epic, in order.
type Epics []EpicOutline

// Story is one ticket of an epic. Functionalities are the ids of the analysis functionalities it delivers.
type Story struct {
	ID              string   `json:"id"`
	Epic            string   `json:"epic"`
	Title           string   `json:"title"`
	Text            string   `json:"text"`
	Acceptance      []string `json:"acceptance"`
	FirstTest       string   `json:"first_test"`
	Functionalities []string `json:"functionalities"`
}

// Report is report.json: how the run and each of its steps went.
type Report struct {
	Status  string    `json:"status"`
	Started time.Time `json:"started"`
	Steps   []Step    `json:"steps"`
}

// Step is one step of a run; Finished is zero while it runs. Note says what code changed in its output, as a clamp,
// or what the step covered, as research's dimensions.
type Step struct {
	Name     string    `json:"name"`
	Status   string    `json:"status"`
	Model    string    `json:"model,omitempty"`
	Harness  string    `json:"harness,omitempty"`
	Effort   string    `json:"effort,omitempty"`
	Started  time.Time `json:"started"`
	Finished time.Time `json:"finished,omitzero"`
	Note     string    `json:"note,omitempty"`
}

// Run is a run folder in memory. Every artefact but the report is nil or empty until its step makes it,
// and absent from the folder while it is.
type Run struct {
	Report      Report
	Input       *InputContext     // input/: brief.md and the files beside it
	Context     string            // contexto.md
	Triage      *Triage           // triage.json
	Research    *Research         // research.json
	Analysis    *Analysis         // analysis.json
	Epics       Epics             // epics.json
	Tickets     map[string]Story  // tickets/<id>.json
	Spec        string            // spec.md
	Plan        json.RawMessage   // plan.json, byte for byte
	DesignBrief string            // brief-diseno.md
	Logs        map[string]string // agent/<step>.log
}
