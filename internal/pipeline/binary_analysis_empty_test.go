//go:build unix

package pipeline_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
)

func TestBinaryRunAcceptsEmptyAnalysisList(t *testing.T) {
	bin := binary(t)
	for name, heading := range map[string]string{
		"personas": "Personas", "functionalities": "Functionalities", "flows": "Flows",
		"constraints": "Constraints", "questions": "Open questions",
	} {
		t.Run(name, func(t *testing.T) {
			repo := realTempDir(t)
			t.Setenv("HOME", t.TempDir())
			if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
				t.Fatalf("git init: %v\n%s", err, out)
			}
			if err := os.WriteFile(filepath.Join(repo, "brief.md"), []byte(binaryRead(t, filepath.Join(binaryExample, "brief.md"))), 0o644); err != nil {
				t.Fatal(err)
			}
			empty := filepath.Join(t.TempDir(), "empty.md")
			if err := os.WriteFile(empty, []byte("\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			answers, err := filepath.Glob(filepath.Join(binaryExample, "answers", "*"))
			if err != nil {
				t.Fatal(err)
			}
			answers = answers[1:] // A text-only brief needs no input agent call.
			replaced := false
			for i, answer := range answers {
				if strings.HasSuffix(answer, "-analysis-"+name+".md") {
					answers[i], replaced = empty, true
				}
				if name == "functionalities" && strings.Contains(filepath.Base(answer), "-epics-E") {
					// Stories cannot refer to functionality IDs when analysis found none.
					lines := strings.Split(binaryRead(t, answer), "\n")
					for j, line := range lines {
						if strings.HasPrefix(line, "Functionalities:") {
							lines[j] = "Functionalities: none"
						}
					}
					answers[i] = filepath.Join(filepath.Dir(empty), filepath.Base(answer))
					if err := os.WriteFile(answers[i], []byte(strings.Join(lines, "\n")), 0o644); err != nil {
						t.Fatal(err)
					}
				}
			}
			if !replaced {
				t.Fatalf("no recorded %s answer", name)
			}
			replay := fakeReplay(t, answers)
			for _, args := range [][]string{{"init"}, {"run", "brief.md"}} {
				cmd := exec.Command(bin, args...)
				cmd.Dir = repo
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("dulidu %s: %v\n%s", strings.Join(args, " "), err, out)
				}
			}
			dirs, _ := filepath.Glob(filepath.Join(repo, ".dulidu", "runs", "*"))
			if len(dirs) != 1 {
				t.Fatalf("run folders: %v", dirs)
			}
			var stored map[string]json.RawMessage
			if err := json.Unmarshal([]byte(binaryRead(t, filepath.Join(dirs[0], "analysis.json"))), &stored); err != nil {
				t.Fatal(err)
			}
			if got := strings.TrimSpace(string(stored[name])); got != "[]" {
				t.Errorf("analysis.json %s = %s, want []", name, got)
			}
			spec := binaryRead(t, filepath.Join(dirs[0], "spec.md"))
			if !strings.Contains(spec, "## "+heading+"\n\n- None.\n\n") {
				t.Errorf("spec.md lacks an empty %s section:\n%s", heading, spec)
			}
			r, err := pipeline.Read(dirs[0])
			if err != nil {
				t.Fatal(err)
			}
			if r.Report.Status != "done" || r.Report.Steps[3].Name != "analysis" || r.Report.Steps[3].Status != "passed" ||
				binaryLines(filepath.Join(replay, "calls")) != len(answers) {
				t.Errorf("run did not finish after empty %s: %+v", name, r.Report)
			}
		})
	}
}
