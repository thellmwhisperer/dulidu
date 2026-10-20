//go:build unix

package pipeline_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
)

func TestBinaryRunAcceptsSparseStories(t *testing.T) {
	bin := binary(t)
	for _, tc := range []struct {
		name, answer string
		invalid      bool
	}{
		{"sparse and unknown functionality", "## US1 Book a wheel\n\n## US2 Refuse a booking\nFunctionalities: F1, F9\n", false},
		{"missing title", "## US1\n", true},
		{"missing id and title", "## \n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := realTempDir(t)
			t.Setenv("HOME", t.TempDir())
			if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
				t.Fatalf("git init: %v\n%s", err, out)
			}
			if err := os.WriteFile(filepath.Join(repo, "brief.md"), []byte(binaryRead(t, filepath.Join(binaryExample, "brief.md"))), 0o644); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "stories.md")
			if err := os.WriteFile(path, []byte(tc.answer), 0o644); err != nil {
				t.Fatal(err)
			}
			answers, err := filepath.Glob(filepath.Join(binaryExample, "answers", "*"))
			if err != nil {
				t.Fatal(err)
			}
			answers = answers[1:]
			for i, answer := range answers {
				if filepath.Base(answer) == "14-epics-E1.md" {
					answers[i] = path
				}
			}
			replay := fakeReplay(t, answers)
			cmd := exec.Command(bin, "init")
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("dulidu init: %v\n%s", err, out)
			}
			cmd = exec.Command(bin, "run", "brief.md")
			cmd.Dir = repo
			out, runErr := cmd.CombinedOutput()
			dirs, _ := filepath.Glob(filepath.Join(repo, ".dulidu", "runs", "*"))
			if len(dirs) != 1 {
				t.Fatalf("run folders: %v", dirs)
			}
			r, err := pipeline.Read(dirs[0])
			if err != nil {
				t.Fatal(err)
			}
			if tc.invalid {
				if runErr == nil || !strings.Contains(string(out), "failed at epics: stories:") || r.Report.Status != "failed" || r.Epics != nil {
					t.Fatalf("invalid story must fail: %v\n%s\n%+v", runErr, out, r.Report)
				}
				return
			}
			if runErr != nil {
				t.Fatalf("dulidu run: %v\n%s", runErr, out)
			}
			if r.Report.Status != "done" || len(r.Epics) != 2 || binaryLines(filepath.Join(replay, "calls")) != len(answers) {
				t.Fatalf("incomplete run: %+v, epics: %+v", r.Report, r.Epics)
			}
			want := []pipeline.Story{
				{ID: "US1", Epic: "E1", Title: "Book a wheel"},
				{ID: "US2", Epic: "E1", Title: "Refuse a booking", Functionalities: []string{"F1"}},
			}
			if !reflect.DeepEqual(r.Epics[0].Stories, want) {
				t.Fatalf("epics.json stories: %+v, want %+v", r.Epics[0].Stories, want)
			}
			for i, story := range want {
				ticket := binaryRead(t, filepath.Join(dirs[0], "tickets", fmt.Sprintf("%d.md", i+1)))
				if !strings.HasPrefix(ticket, "# "+story.Title+"\n") || !strings.Contains(ticket, "Story: "+story.ID+".") || strings.Contains(ticket, "F9") {
					t.Errorf("exported ticket: %s", ticket)
				}
			}
			note := ""
			for _, step := range r.Report.Steps {
				if step.Name == "epics" {
					if step.Status != "passed" {
						t.Errorf("epics status: %s", step.Status)
					}
					note = step.Note
				}
			}
			if !slices.Contains(strings.Split(note, "; "), "dropped functionality F9 from US2 in E1") {
				t.Errorf("epics note: %q", note)
			}
			// Sparse stories must also survive the JSON ticket writer's validation.
			r.Tickets = map[string]pipeline.Story{"US1": want[0], "US2": want[1]}
			if err := pipeline.Write(dirs[0], r); err != nil {
				t.Fatalf("write sparse tickets: %v", err)
			}
		})
	}
}
