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

// Triage is the first read of the brief: what it is about and the dimensions to research.
type Triage struct {
	Summary    string   `json:"summary"`
	Dimensions []string `json:"dimensions"`
}

// Research is what was found, one entry per dimension.
type Research struct {
	Dimensions []Dimension `json:"dimensions"`
}

// Dimension is one researched dimension: what was found and where.
type Dimension struct {
	Name    string   `json:"name"`
	Text    string   `json:"text"`
	Sources []string `json:"sources"`
}

// Analysis is the brief taken apart.
type Analysis struct {
	Personas        []string `json:"personas"`
	Functionalities []string `json:"functionalities"`
	Flows           []string `json:"flows"`
	Constraints     []string `json:"constraints"`
	Questions       []string `json:"questions"`
}

// EpicOutline is one epic before its stories.
type EpicOutline struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

// Epics is epics.json: every epic, in order.
type Epics []EpicOutline

// Story is one ticket of an epic.
type Story struct {
	Epic       string   `json:"epic"`
	Title      string   `json:"title"`
	Text       string   `json:"text"`
	Acceptance []string `json:"acceptance"`
}

// Report is report.json: how the run and each of its steps went.
type Report struct {
	Status  string    `json:"status"`
	Started time.Time `json:"started"`
	Steps   []Step    `json:"steps"`
}

// Step is one step of a run; Finished is zero while it runs.
type Step struct {
	Name     string    `json:"name"`
	Status   string    `json:"status"`
	Model    string    `json:"model"`
	Harness  string    `json:"harness"`
	Started  time.Time `json:"started"`
	Finished time.Time `json:"finished,omitzero"`
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
