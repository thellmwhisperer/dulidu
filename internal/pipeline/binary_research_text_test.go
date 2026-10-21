//go:build unix

package pipeline_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/prompts"
)

func TestBinaryResearchReadsPlainText(t *testing.T) {
	bin := binary(t)
	for _, missingSource := range []bool{false, true} {
		name := "prose and sources"
		if missingSource {
			name = "dimension without a source fails"
		}
		t.Run(name, func(t *testing.T) {
			repo, replies := realTempDir(t), t.TempDir()
			t.Setenv("HOME", t.TempDir())
			if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
				t.Fatalf("git init: %v\n%s", err, out)
			}
			if err := os.WriteFile(filepath.Join(repo, "brief.md"), []byte("A studio booking tool.\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			answers := []string{filepath.Join(binaryExample, "answers", "02-triage.md")}
			var want []pipeline.Dimension
			for _, dimension := range []string{"business", "users", "market", "technical", "project"} {
				d := pipeline.Dimension{Name: dimension, Text: "A finding about " + dimension + ".\nA second sentence.",
					Sources: []pipeline.Source{
						{URL: "https://example.com/" + dimension, Title: "Study of " + dimension, Supports: "The first claim."},
						{URL: "https://example.com/guide_(2026)", Title: "Another study", Supports: "The second claim: a detail."},
					}}
				answer := d.Text + "\n\n## Sources\n"
				for _, s := range d.Sources {
					answer += "- [" + s.Title + "](" + s.URL + "): " + s.Supports + "\n"
				}
				if missingSource && dimension == "users" {
					answer = d.Text + "\n\n## Sources\n"
				}
				path := filepath.Join(replies, dimension+".md")
				if err := os.WriteFile(path, []byte(answer), 0o644); err != nil {
					t.Fatal(err)
				}
				answers = append(answers, path)
				want = append(want, d)
			}
			for _, step := range []string{"analysis", "epics", "design"} {
				recorded, err := filepath.Glob(filepath.Join(binaryExample, "answers", "*-"+step+"*"))
				if err != nil || len(recorded) == 0 {
					t.Fatalf("%s answers: %v, %v", step, recorded, err)
				}
				answers = append(answers, recorded...)
			}
			replay := fakeReplay(t, answers)
			init := exec.Command(bin, "init")
			init.Dir = repo
			if out, err := init.CombinedOutput(); err != nil {
				t.Fatalf("dulidu init: %v\n%s", err, out)
			}
			cmd := exec.Command(bin, "run", "brief.md")
			cmd.Dir = repo
			out, runErr := cmd.CombinedOutput()
			if missingSource {
				if runErr == nil || !strings.Contains(string(out), "failed at research: users: no sources") {
					t.Fatalf("missing source: %v\n%s", runErr, out)
				}
			} else if runErr != nil {
				t.Fatalf("plain-text research should pass: %v\n%s", runErr, out)
			}
			dirs, _ := filepath.Glob(filepath.Join(repo, ".dulidu", "runs", "*"))
			if len(dirs) != 1 {
				t.Fatalf("run folders: %v", dirs)
			}
			r, err := pipeline.Read(dirs[0])
			if err != nil {
				t.Fatal(err)
			}
			if missingSource {
				if r.Report.Status != "failed" || len(r.Report.Steps) != 3 ||
					r.Report.Steps[2].Status != "failed at research: users: no sources" ||
					r.Research != nil || binaryLines(filepath.Join(replay, "calls")) != 3 {
					t.Fatalf("missing source must stop research: %+v", r)
				}
				if _, err := os.Stat(filepath.Join(dirs[0], "research.json")); !os.IsNotExist(err) {
					t.Fatalf("failed research wrote research.json: %v", err)
				}
				return
			}
			var stored pipeline.Research
			if err := json.Unmarshal([]byte(binaryRead(t, filepath.Join(dirs[0], "research.json"))), &stored); err != nil {
				t.Fatalf("research.json must remain JSON: %v", err)
			}
			if !reflect.DeepEqual(stored.Dimensions, want) {
				t.Errorf("research.json: %+v, want %+v", stored.Dimensions, want)
			}
			if r.Report.Status != "done" || r.Report.Steps[2].Status != "passed" ||
				binaryLines(filepath.Join(replay, "calls")) != len(answers) {
				t.Errorf("run did not finish after plain-text research: %+v", r.Report)
			}
			for _, tmpl := range []string{"p03-research-business", "p04-research-users",
				"p05-research-market", "p06-research-technical", "p07-research-project"} {
				prompt, err := prompts.Render(tmpl, map[string]any{"Input": r.Context})
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(strings.ToLower(prompt), "json") || !strings.Contains(prompt, "prose") ||
					!strings.Contains(prompt, "## Sources") || !strings.Contains(prompt, "- [") ||
					!strings.Contains(prompt, "claim") {
					t.Errorf("%s must ask for prose and a Sources list:\n%s", tmpl, prompt)
				}
			}
		})
	}
}
