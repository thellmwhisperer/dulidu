//go:build unix

package pipeline_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
)

var (
	builtOnce sync.Once
	built     string
	buildErr  error
)

const binaryExample = "testdata/run-example"

func binaryRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func binaryLines(path string) int {
	b, _ := os.ReadFile(path)
	return strings.Count(string(b), "\n")
}

// binary is ./cmd/dulidu built with go build, once per test process.
func binary(t *testing.T) string {
	t.Helper()
	builtOnce.Do(func() {
		dir, err := os.MkdirTemp("", "dulidu-bin-")
		if err != nil {
			buildErr = err
			return
		}
		built = filepath.Join(dir, "dulidu")
		if out, err := exec.Command("go", "build", "-o", built, "../../cmd/dulidu").CombinedOutput(); err != nil {
			buildErr = fmt.Errorf("go build: %v\n%s", err, out)
		}
	})
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	return built
}

// replayClaude is a claude whose nth call answers with $FAKE_REPLAY/<n>, a stream-json result line; with none it
// fails. Each call adds a line to $FAKE_REPLAY/calls and saves its prompt in $FAKE_REPLAY/prompt-<n>.
const replayClaude = `#!/bin/sh
echo >> "$FAKE_REPLAY/calls"
n=$(wc -l < "$FAKE_REPLAY/calls")
cat > "$FAKE_REPLAY/prompt-$((n))"
exec cat "$FAKE_REPLAY/$((n))"
`

// fakeReplay puts replayClaude first on PATH, answering in order with the recorded answers named, and returns the
// folder whose calls file counts its calls.
func fakeReplay(t *testing.T, answers []string) string {
	t.Helper()
	bin, replay := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(replayClaude), 0o755); err != nil {
		t.Fatal(err)
	}
	for i, path := range answers {
		line, _ := json.Marshal(map[string]string{"type": "result", "result": binaryRead(t, path)})
		if err := os.WriteFile(filepath.Join(replay, strconv.Itoa(i+1)), append(line, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_REPLAY", replay)
	return replay
}

// TestBinaryRunRecorded runs the built dulidu on the example's brief, piped, with a claude that replays the recorded
// answers: the run is done and every step passed, including the input step, as dulidu status prints in the README.
func TestBinaryRunRecorded(t *testing.T) {
	bin := binary(t)
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(repo, "brief.md"), []byte(binaryRead(t, filepath.Join(binaryExample, "brief.md"))), 0o644); err != nil {
		t.Fatal(err)
	}
	// The brief alone has no image, so the input step asks nothing.
	answers, _ := filepath.Glob(filepath.Join(binaryExample, "answers", "*"))
	answers = answers[1:]
	replay := fakeReplay(t, answers)
	t.Setenv("HOME", t.TempDir())
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(bin, args...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("dulidu %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return string(out)
	}
	run("init")
	run("run", "brief.md")

	if n := binaryLines(filepath.Join(replay, "calls")); n != len(answers) {
		t.Errorf("claude was called %d times, want %d", n, len(answers))
	}
	dirs, _ := filepath.Glob(filepath.Join(repo, ".dulidu", "runs", "*"))
	if len(dirs) != 1 {
		t.Fatalf("run folders: %v", dirs)
	}
	r, err := pipeline.Read(dirs[0])
	if err != nil {
		t.Fatal(err)
	}
	if r.Report.Status != "done" {
		t.Errorf("report.json status %q, want done", r.Report.Status)
	}
	want := filepath.Base(dirs[0]) + ` done
input passed 0 min 00 s
triage passed 0 min 00 s
research passed 0 min 00 s
analysis passed 0 min 00 s
epics passed 0 min 00 s
export passed 0 min 00 s
`
	if got := regexp.MustCompile(`\d+ min \d\d s`).ReplaceAllString(run("status"), "0 min 00 s"); got != want {
		t.Errorf("dulidu status:\n%s\nwant:\n%s", got, want)
	}
}

// TestBinarySpecLikeThePrototype runs the built dulidu, piped, in a repository folder named teamplanner, with
// functionalities F7 and F9, two personas with goals and pain points, a flow of three steps and a story naming F9:
// spec.md reads like the prototype's report, titled with the folder's name, and the open-questions call gets the
// brief and the research as named paragraphs, without the lists asked before it.
func TestBinarySpecLikeThePrototype(t *testing.T) {
	bin := binary(t)
	repo := filepath.Join(realTempDir(t), "teamplanner")
	t.Setenv("HOME", t.TempDir())
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(repo, "brief.md"), []byte(binaryRead(t, filepath.Join(binaryExample, "brief.md"))), 0o644); err != nil {
		t.Fatal(err)
	}
	scratch := t.TempDir()
	answer := func(name, body string) string {
		path := filepath.Join(scratch, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	recorded := func(name string) string { return filepath.Join(binaryExample, "answers", name) }
	var answers []string
	for _, name := range []string{"02-triage.md", "03-research-business.md", "04-research-users.md", "05-research-market.md",
		"06-research-technical.md", "07-research-project.md", "08-analysis-personas.md"} {
		answers = append(answers, recorded(name))
	}
	answers = append(answers,
		answer("functionalities.md", "## F7 Book a wheel\nPriority: must\nOne hour, one member per wheel\n\n"+
			"## F9 Drop off for the kiln\nPriority: must\nPieces in before Friday 6 pm make that week's firing\n"),
		answer("flows.md", "## FL1 Booking\n1. Pick a free hour\n2. Pick a wheel\n3. Confirm\n"),
		recorded("11-analysis-constraints.md"), recorded("12-analysis-questions.md"), recorded("13-epics-outlines.md"),
		answer("stories-E1.md", strings.Replace(binaryRead(t, recorded("14-epics-E1.md")), "Functionalities: F1", "Functionalities: F9", 1)),
		answer("stories-E2.md", strings.Replace(binaryRead(t, recorded("15-epics-E2.md")), "Functionalities: F2", "Functionalities: F7", 1)))
	replay := fakeReplay(t, answers)
	for _, args := range [][]string{{"init"}, {"run", "brief.md"}} {
		cmd := exec.Command(bin, args...)
		cmd.Dir, cmd.Stdin = repo, strings.NewReader("")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("dulidu %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	dirs, _ := filepath.Glob(filepath.Join(repo, ".dulidu", "runs", "*"))
	if len(dirs) != 1 {
		t.Fatalf("run folders: %v", dirs)
	}
	spec := binaryRead(t, filepath.Join(dirs[0], "spec.md"))
	if !strings.HasPrefix(spec, "# teamplanner\n") {
		t.Errorf("spec.md does not start with the project's name:\n%s", spec)
	}
	for _, want := range []string{"\n### Business Context\n", "\n### Users & Personas\n", "\n### Market & Competition\n",
		"\n### Technical Landscape\n", "\n### Project Scope\n", "\n#### Ana: member\n", "\n**Goals:**\n",
		"\n**Pain Points:**\n", "\n| ID | Priority | Name | Description |\n", "\n| F9 | must | Drop off for the kiln |",
		"Pick a free hour → Pick a wheel → Confirm", "\n### Constraints\n", "\n### Open Questions\n", "\n## Epics\n"} {
		if !strings.Contains(spec, want) {
			t.Errorf("spec.md lacks %q:\n%s", want, spec)
		}
	}
	ticket := binaryRead(t, filepath.Join(dirs[0], "tickets", "1.md"))
	_, scope, _ := strings.Cut(ticket, "## Scope surface\n")
	scope, _, _ = strings.Cut(scope, "## ")
	if !strings.Contains(scope, "F9") {
		t.Errorf("ticket 1's scope does not name F9:\n%s", ticket)
	}
	// The open-questions call is the eleventh: triage, five research dimensions, then the five lists.
	questions := binaryRead(t, filepath.Join(replay, "prompt-11"))
	if !strings.Contains(questions, "## Research Context\nBusiness: ") || !strings.Contains(questions, "\nTechnical: ") ||
		regexp.MustCompile(`\b(Ana|Luis)\b`).MatchString(questions) {
		t.Errorf("the open-questions prompt is not the brief and the named research alone:\n%s", questions)
	}
}
