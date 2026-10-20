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
// fails. Each call adds a line to $FAKE_REPLAY/calls.
const replayClaude = `#!/bin/sh
cat > /dev/null
echo >> "$FAKE_REPLAY/calls"
n=$(wc -l < "$FAKE_REPLAY/calls")
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
