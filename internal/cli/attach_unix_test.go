//go:build unix

package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"
)

// reportAt writes report as the report.json of the run label in the repository dir and returns the run folder.
func reportAt(t *testing.T, dir, label, report string) string {
	t.Helper()
	run := filepath.Join(dir, ".dulidu", "runs", label)
	if err := os.MkdirAll(run, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(run, "report.json"), []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	return run
}

const (
	inputStep = `{"name": "input", "status": "done", "model": "", "harness": "claude",
		"started": "2026-10-17T01:00:00Z", "finished": "2026-10-17T01:00:05Z"}`
	triageRunning = `{"status": "done", "started": "2026-10-17T01:00:00Z", "steps": [` + inputStep + `,
		{"name": "triage", "status": "running", "model": "", "harness": "claude", "started": "2026-10-17T01:00:05Z"}]}`
	triagePassed = `{"status": "done", "started": "2026-10-17T01:00:00Z", "steps": [` + inputStep + `,
		{"name": "triage", "status": "passed", "model": "", "harness": "claude",
		 "started": "2026-10-17T01:00:05Z", "finished": "2026-10-17T01:01:10Z"}]}`
)

func TestAttachPipedPrintsOneLinePerStepUntilTheRunEnds(t *testing.T) {
	dir := repo(t)
	// A live run holds its agent output locked, and its report says done between two steps.
	run := reportAt(t, dir, "20261017-010000-1", triageRunning)
	live, err := os.OpenFile(filepath.Join(run, "agent.out"), os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(live.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	type result struct {
		code        int
		out, errOut string
	}
	got := make(chan result, 1)
	go func() {
		code, out, errOut := dulidu(t, "attach", "20261017-010000-1")
		got <- result{code, out, errOut}
	}()
	reportAt(t, dir, "20261017-010000-1", triagePassed)
	_ = live.Close() // the run ends
	select {
	case r := <-got:
		if want := "input done 0 min 05 s\ntriage passed 1 min 05 s\n"; r.code != 0 || r.out != want {
			t.Fatalf("dulidu attach | cat: exit %d\n%s%s\nwant:\n%s", r.code, r.out, r.errOut, want)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("dulidu attach did not end with the run")
	}
	if code, _, _ := dulidu(t, "attach", "no-such-run"); code != 1 {
		t.Errorf("dulidu attach of no run: exit %d, want 1", code)
	}
}

func TestStatusPrintsARunsSteps(t *testing.T) {
	dir := repo(t)
	reportAt(t, dir, "20261016-090000-1", `{"status": "failed", "started": "2026-10-16T09:00:00Z", "steps": [`+inputStep+`]}`)
	reportAt(t, dir, "20261017-010000-1", triagePassed)
	for args, want := range map[string]string{
		"status":                   "20261017-010000-1 done\ninput done 0 min 05 s\ntriage passed 1 min 05 s\n",
		"status 20261016-090000-1": "20261016-090000-1 failed\ninput done 0 min 05 s\n",
	} {
		if code, out, errOut := dulidu(t, strings.Fields(args)...); code != 0 || out != want {
			t.Errorf("dulidu %s: exit %d\n%s%s\nwant:\n%s", args, code, out, errOut, want)
		}
	}
}

func TestAttachToAFinishedRunPrintsEveryStepAndItsAgentOutputIsKept(t *testing.T) {
	runRepo(t, map[string]string{})
	triageOn(t, `[]`, `[]`)
	if code, out, errOut := dulidu(t, "run", "brief.md"); code != 0 {
		t.Fatalf("dulidu run: exit %d\n%s%s", code, out, errOut)
	}
	dir := runs(t)[0]
	code, out, errOut := dulidu(t, "attach", filepath.Base(dir))
	if want := `^input done \d+ min \d\d s\ntriage passed \d+ min \d\d s\nresearch passed \d+ min \d\d s\n` +
		`analysis passed \d+ min \d\d s\nepics passed \d+ min \d\d s\nexport passed \d+ min \d\d s\n$`; code != 0 ||
		!regexp.MustCompile(want).MatchString(out) {
		t.Fatalf("dulidu attach: exit %d\n%s%s", code, out, errOut)
	}
	if got := read(t, filepath.Join(dir, "agent.out")); !strings.Contains(got, `\"complexity\":\"simple\"`) {
		t.Errorf("agent.out lacks the agents' lines:\n%s", got)
	}
}
