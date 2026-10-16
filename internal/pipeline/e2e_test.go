//go:build unix

package pipeline_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/analysis"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/epics"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/export"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/input"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/research"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/triage"
)

// example is the recorded run: its brief, two attachments and, in answers/, every model answer in call order, each
// named <n>-<step>. A change to a step that breaks the recording updates it in the same PR.
const example = "testdata/run-example"

// TestEndToEndRecorded chains every step, as dulidu run will, on the example with a fake agent that replays the
// recorded answers: the run folder is complete and every step passed.
func TestEndToEndRecorded(t *testing.T) {
	for _, tool := range []string{"pdftotext", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed", tool)
		}
	}
	answers, _ := filepath.Glob(filepath.Join(example, "answers", "*"))
	used := 0
	// replay is the fake agent for step: each call is its next recorded answer, none left an error.
	replay := func(step string) func(string) (string, error) {
		mine, _ := filepath.Glob(filepath.Join(example, "answers", "*-"+step+"*"))
		return func(string) (string, error) {
			if len(mine) == 0 {
				return "", errors.New("no recorded answer left for " + step)
			}
			b, err := os.ReadFile(mine[0])
			mine, used = mine[1:], used+1
			return string(b), err
		}
	}
	step := func(name string) pipeline.Step { return pipeline.Step{Name: name, Harness: "claude"} }

	start := time.Now()
	in, err := input.Load(filepath.Join(example, "brief.md"), []string{filepath.Join(example, "one.pdf"), filepath.Join(example, "one.png")})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	s := step("input")
	s.Status, s.Started = "running", start
	r := pipeline.Run{Report: pipeline.Report{Status: "running", Started: start, Steps: []pipeline.Step{s}}, Input: &in}
	if err := pipeline.Write(dir, r); err != nil {
		t.Fatal(err)
	}
	if r.Context, err = input.Package(dir, in, replay("input")); err != nil {
		t.Fatal(err)
	}
	r.Report.Steps[0].Status, r.Report.Steps[0].Finished = "passed", time.Now()
	for _, run := range []func() error{
		func() error { return triage.Run(dir, &r, step("triage"), 0, 0, replay("triage")) },
		func() error { return research.Run(dir, &r, step("research"), replay("research")) },
		func() error { return analysis.Run(dir, &r, step("analysis"), replay("analysis")) },
		func() error { return epics.Run(dir, &r, step("epics"), replay("epics")) },
		func() error { return export.Write(dir, &r) },
	} {
		if err := run(); err != nil {
			t.Fatal(err)
		}
	}
	if took := time.Since(start); took > time.Second {
		t.Errorf("the run took %v, want under a second", took)
	}

	if used != len(answers) || used == 0 {
		t.Errorf("the agent replayed %d of %d recorded answers", used, len(answers))
	}
	for _, name := range []string{"report.json", "contexto.md", "triage.json", "research.json", "analysis.json", "epics.json",
		"spec.md", "plan.json", "brief-diseno.md", "tickets/1.md", "tickets/2.md", "tickets/3.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Error(err)
		}
	}
	got, err := pipeline.Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Report.Status != "done" || len(got.Report.Steps) != 5 {
		t.Errorf("report.json: %+v", got.Report)
	}
	for _, s := range got.Report.Steps {
		if s.Status != "passed" {
			t.Errorf("step %s: %s", s.Name, s.Status)
		}
	}
}
