package cli

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
)

// runEvents writes NDJSON directly to the caller, without changing the run folder.
// Agent data retains the harness's JSON; non-JSON diagnostics become strings.
// Agent.Run serializes log writes and joins them before a step can finish.
type runEvents struct {
	out  *json.Encoder
	step string
	err  error
}

func (e *runEvents) emit(kind, step string, data any) {
	if e.out != nil && e.err == nil {
		e.err = e.out.Encode(struct {
			Type string    `json:"type"`
			Step string    `json:"step"`
			TS   time.Time `json:"ts"`
			Data any       `json:"data"`
		}{kind, step, time.Now(), data})
	}
}

func (e *runEvents) start(step string) {
	e.step = step
	e.emit("step_start", step, nil)
}

func (e *runEvents) finish(r pipeline.Run) {
	step := r.Report.Steps[len(r.Report.Steps)-1]
	e.emit("step_result", step.Name, step)
	if step.Name == "analysis" && r.Analysis != nil {
		for _, question := range r.Analysis.Questions {
			e.emit("question", step.Name, question)
		}
	}
}

// Write receives one complete agent line at a time from Agent.Run's log writer.
func (e *runEvents) Write(p []byte) (int, error) {
	var data any = strings.TrimSuffix(string(p), "\n")
	if json.Valid(p) {
		data = json.RawMessage(p)
	}
	e.emit("agent", e.step, data)
	return len(p), e.err
}
