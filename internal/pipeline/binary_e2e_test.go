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
	// The test runner uses its own GOCOVERDIR. Keep binary coverage separate.
	coverage := os.Getenv("DULIDU_TEST_COVERDIR")
	if coverage != "" {
		t.Setenv("GOCOVERDIR", coverage)
	}
	builtOnce.Do(func() {
		dir, err := os.MkdirTemp("", "dulidu-bin-")
		if err != nil {
			buildErr = err
			return
		}
		built = filepath.Join(dir, "dulidu")
		args := []string{"build"}
		if coverage != "" {
			args = append(args, "-cover")
		}
		args = append(args, "-o", built, "../../cmd/dulidu")
		if out, err := exec.Command("go", args...).CombinedOutput(); err != nil {
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

// TestBinaryTicketsCarryTheirStory runs the built dulidu, piped, on stories US1 (a feature with two use cases and
// one invariant) and US2 (a fix with no invariants and one non-goal), and an analysis with three constraints: each
// ticket is filled from its own story, the two share no line but headings, empty lists and the rollback, and the
// stories prompt asks for the prototype's size.
func TestBinaryTicketsCarryTheirStory(t *testing.T) {
	bin := binary(t)
	repo := realTempDir(t)
	t.Setenv("HOME", t.TempDir())
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
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
	constraints := []string{"The kiln fires once a week, on Friday at 6 pm", "Bookings are free: members pay a monthly fee",
		"A member books at most two hours a week"}
	recorded := func(name string) string { return filepath.Join(binaryExample, "answers", name) }
	var answers []string
	for _, name := range []string{"02-triage.md", "03-research-business.md", "04-research-users.md", "05-research-market.md",
		"06-research-technical.md", "07-research-project.md", "08-analysis-personas.md", "09-analysis-functionalities.md",
		"10-analysis-flows.md"} {
		answers = append(answers, recorded(name))
	}
	answers = append(answers,
		answer("constraints.md", "- "+strings.Join(constraints, "\n- ")+"\n"),
		recorded("12-analysis-questions.md"), recorded("13-epics-outlines.md"),
		answer("stories-E1.md", "## US1 Book a free wheel for an hour\nType: feature\nFunctionalities: F1\n"+
			"As Ana I want to book a free wheel so that one waits for me\n"+
			"### Acceptance\n- Given wheel 2 free at 19:00, when Ana books it, then it is hers from 19:00 to 20:00\n"+
			"### First test\nWrite a test that books a free wheel at 19:00 and expects it held until 20:00.\n"+
			"### Use cases\n- Ana books wheel 2 at 19:00 on Tuesday and finds it waiting\n"+
			"- Luis books wheel 1 at 10:00 on Saturday before his class\n"+
			"### Invariants\n- "+constraints[1]+"\n"),
		answer("stories-E2.md", "## US2 Fire pieces dropped at 17:59 on Friday\nType: fix\nFunctionalities: F2\n"+
			"As Ana I want a piece dropped at 17:59 on Friday fired that week, not the next\n"+
			"### Acceptance\n- Given a piece dropped off on Friday at 17:59, when the kiln fires, then the piece is in that firing\n"+
			"### First test\nWrite a test that drops a piece off at 17:59 on Friday and expects it in that week's firing.\n"+
			"### Non-goals\n- Moving the firing day\n"))
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
	one := binaryRead(t, filepath.Join(dirs[0], "tickets", "1.md"))
	two := binaryRead(t, filepath.Join(dirs[0], "tickets", "2.md"))
	shared := map[string]bool{}
	for _, line := range strings.Split(one, "\n") {
		shared[line] = true
	}
	for _, line := range strings.Split(two, "\n") {
		if shared[line] && line != "" && line != "- None." && line != "Revert the PR." && !strings.HasPrefix(line, "#") {
			t.Errorf("tickets 1 and 2 share the line %q:\n%s\n%s", line, one, two)
		}
	}
	if !regexp.MustCompile(`(?m)^Type: fix$`).MatchString(two) {
		t.Errorf("ticket 2 is not a fix:\n%s", two)
	}
	section := func(ticket, name string) string {
		_, s, _ := strings.Cut(ticket, "\n## "+name+"\n")
		s, _, _ = strings.Cut(s, "\n## ")
		return s
	}
	if got, want := strings.TrimSpace(section(one, "Invariants")), "- "+constraints[1]; got != want {
		t.Errorf("ticket 1's invariants are %q, want %q", got, want)
	}
	if got, want := strings.TrimSpace(section(one, "Scope surface")), "- F1 Book a wheel: One hour, one member per wheel"; got != want {
		t.Errorf("ticket 1's scope is %q, want %q", got, want)
	}
	evidence := section(one, "Evidence (fix / ops)")
	for _, want := range []string{"- Ana books wheel 2 at 19:00 on Tuesday and finds it waiting\n",
		"- Luis books wheel 1 at 10:00 on Saturday before his class\n"} {
		if !strings.Contains(evidence, want) {
			t.Errorf("ticket 1's evidence lacks %q:\n%s", want, one)
		}
	}
	// The stories call for E1 is the fourteenth: triage, five research dimensions, five lists, the outlines.
	if prompt := binaryRead(t, filepath.Join(replay, "prompt-14")); !strings.Contains(prompt, "100-200 lines") {
		t.Errorf("the stories prompt does not ask for the story's size:\n%s", prompt)
	}
}

// The built binary must read the same forms with a whole-answer fence or introductory prose.
func TestBinaryReadsFencedAnswers(t *testing.T) {
	bin := binary(t)
	var plain pipeline.Run
	for _, wrapped := range []bool{false, true} {
		repo := realTempDir(t)
		t.Setenv("HOME", t.TempDir())
		if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
			t.Fatalf("git init: %v\n%s", err, out)
		}
		if err := os.WriteFile(filepath.Join(repo, "brief.md"), []byte(binaryRead(t, filepath.Join(binaryExample, "brief.md"))), 0o644); err != nil {
			t.Fatal(err)
		}
		answers, err := filepath.Glob(filepath.Join(binaryExample, "answers", "*"))
		if err != nil || len(answers) != 15 {
			t.Fatalf("recordings: %v, %v", answers, err)
		}
		answers = answers[1:]
		if wrapped {
			for i, path := range answers {
				body := binaryRead(t, path)
				switch name := filepath.Base(path); {
				case name == "13-epics-outlines.md":
					body = "Here are the epics:\n" + body
				case name == "02-triage.md":
					body = "Here is the classification.\n" + body
				case name == "14-epics-E1.md", strings.Contains(name, "research-"):
					body = "\n```markdown\n" + body + "```\n\n"
				}
				answers[i] = filepath.Join(t.TempDir(), filepath.Base(path))
				if err := os.WriteFile(answers[i], []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}
		fakeReplay(t, answers)
		for _, args := range [][]string{{"init"}, {"run", "brief.md"}} {
			cmd := exec.Command(bin, args...)
			cmd.Dir, cmd.Stdin = repo, strings.NewReader("")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("dulidu %v: %v\n%s", args, err, out)
			}
		}
		dirs, _ := filepath.Glob(filepath.Join(repo, ".dulidu", "runs", "*"))
		if len(dirs) != 1 {
			t.Fatalf("run folders: %v", dirs)
		}
		r, err := pipeline.Read(dirs[0])
		if err != nil || r.Report.Status != "done" || len(r.Epics) != 2 || len(r.Epics[0].Stories) != 2 {
			t.Fatalf("incomplete run: %+v, %v", r, err)
		}
		if !wrapped {
			plain = r
			continue
		}
		for name, pair := range map[string][2]any{
			"epics and stories": {r.Epics, plain.Epics}, "research prose": {r.Research, plain.Research},
			"triage": {r.Triage, plain.Triage}, "analysis": {r.Analysis, plain.Analysis},
		} {
			got, _ := json.Marshal(pair[0])
			want, _ := json.Marshal(pair[1])
			if string(got) != string(want) {
				t.Errorf("%s changed with wrapping:\n%s\nwant:\n%s", name, got, want)
			}
		}
	}
}

func TestBinaryDoctorChecksWhatARunNeeds(t *testing.T) {
	bin := binary(t)
	repo, path := realTempDir(t), t.TempDir()
	t.Setenv("HOME", t.TempDir())
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(git, filepath.Join(path, "git")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ffmpeg", "ffprobe", "whisper-cli"} {
		if err := os.WriteFile(filepath.Join(path, name), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", path)
	for _, installed := range []bool{false, true} {
		if installed {
			if err := os.WriteFile(filepath.Join(path, "pdftotext"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		cmd := exec.Command(bin, "doctor")
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if strings.Contains(string(out), "ok go:") || strings.Contains(string(out), "fail go:") {
			t.Errorf("doctor checks Go:\n%s", out)
		}
		if installed {
			if err != nil {
				t.Errorf("doctor with run tools: %v\n%s", err, out)
			}
		} else if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 || !strings.Contains(string(out),
			"fail pdftotext: not found in PATH: install it with brew install poppler, or apt install poppler-utils") {
			t.Errorf("doctor without pdftotext: %v\n%s", err, out)
		}
	}
}

func TestBinaryRejectsKeysNobodyReads(t *testing.T) {
	bin := binary(t)
	repo := realTempDir(t)
	t.Setenv("HOME", t.TempDir())
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	cmd := exec.Command(bin, "init")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("dulidu init: %v\n%s", err, out)
	}
	config := filepath.Join(repo, "dulidu.yaml")
	for _, key := range []string{"platform", "project_type", "ticket_size", "exclusions", "stories"} {
		if regexp.MustCompile(`(?m)^\s*` + key + `:`).MatchString(binaryRead(t, config)) {
			t.Errorf("init still writes %s", key)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "brief.md"), []byte("Book a wheel."), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"platform", "project_type", "ticket_size", "exclusions", "steps.stories"} {
		body := key + ": \"\"\n"
		if key == "steps.stories" {
			body = "steps:\n  stories: {}\n"
		}
		if err := os.WriteFile(config, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(bin, "run", "brief.md")
		cmd.Dir, cmd.Stdin = repo, strings.NewReader("")
		out, err := cmd.CombinedOutput()
		if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 ||
			!strings.Contains(string(out), "dulidu.yaml: "+key+" is no longer read; delete it") {
			t.Errorf("removed %s: %v\n%s", key, err, out)
		}
	}
}
