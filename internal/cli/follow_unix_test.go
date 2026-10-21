//go:build unix

package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The fake blocks on a FIFO until the reader has received its live message and
// tool call. A buffered-at-step-end implementation cannot complete this test.
func TestRunFollowStreamsEvents(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "dulidu")
	build := exec.Command("go", "build", "-o", bin, "../../cmd/dulidu")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	t.Run("without follow", func(t *testing.T) {
		runRepo(t, map[string]string{})
		cmd := exec.Command(bin, "run", "brief.md")
		out, err := cmd.CombinedOutput()
		if err != nil || !strings.HasSuffix(string(out), "/contexto.md\n") {
			t.Fatalf("ordinary run: %v, %s", err, out)
		}
	})
	t.Run("output error", func(t *testing.T) {
		runRepo(t, map[string]string{})
		out, err := os.Open("brief.md") // stdout is deliberately read-only
		if err != nil {
			t.Fatal(err)
		}
		defer out.Close()
		cmd := exec.Command(bin, "run", "brief.md", "--follow")
		var stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = out, &stderr
		if err := cmd.Run(); err == nil || cmd.ProcessState.ExitCode() != 1 || !strings.Contains(stderr.String(), "bad file descriptor") {
			t.Fatalf("output error: %v, %s", err, stderr.String())
		}
	})
	for _, fail := range []bool{false, true} {
		name := "done"
		if fail {
			name = "failed"
		}
		t.Run(name, func(t *testing.T) {
			runRepo(t, map[string]string{"image.png": "fake image"})
			fakeBin := strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))[0]
			if err := os.WriteFile(filepath.Join(fakeBin, "ffprobe"), []byte("#!/bin/sh\nprintf '%s\\n' '{\"streams\":[{\"width\":32,\"height\":32}]}'\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			fifo := filepath.Join(t.TempDir(), "release")
			if err := syscall.Mkfifo(fifo, 0o600); err != nil {
				t.Fatal(err)
			}
			release, err := os.OpenFile(fifo, os.O_RDWR, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer release.Close()
			t.Setenv("FOLLOW_RELEASE", fifo)
			if fail {
				t.Setenv("FOLLOW_FAIL", "1")
			}
			fake := strings.Replace(fakeDescriber, `cat > "$prompt"`, `cat > "$prompt"
if grep -q '^Research the following product idea' "$prompt" && [ ! -f "$FAKE_LOG.research" ]; then
  printf '%s\n' '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Still researching"}]}}'
  printf '%s\n' '{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"tool-1","name":"WebSearch","input":{"query":"bikes"}}]}}'
  printf '%s\n' 'research diagnostic' >&2
  read -r continue < "$FOLLOW_RELEASE"
  if [ -n "$FOLLOW_FAIL" ]; then exit 7; fi
fi`, 1)
			agentPath, err := exec.LookPath("claude")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(agentPath, []byte(fake), 0o755); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, bin, "run", "brief.md", "--attach", "image.png", "--follow")
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = cmd.Process.Kill() }()
			lines := make(chan string)
			scanErr := make(chan error, 1)
			go func() {
				defer close(lines)
				scanner := bufio.NewScanner(stdout)
				scanner.Buffer(make([]byte, 4096), 4<<20)
				for scanner.Scan() {
					select {
					case lines <- scanner.Text():
					case <-ctx.Done():
						scanErr <- ctx.Err()
						return
					}
				}
				scanErr <- scanner.Err()
			}()
			var starts, results []string
			var active string
			var message, tool, diagnostic, released, question, ended bool
			agentEvents := 0
			for line := range lines {
				t.Logf("dulidu stdout: %s", line)
				var e struct {
					Type, Step string
					TS         time.Time
					Data       json.RawMessage
				}
				if err := json.Unmarshal([]byte(line), &e); err != nil || e.Step == "" || e.TS.IsZero() {
					t.Fatalf("invalid event: %q (%v)", line, err)
				}
				if ended {
					t.Fatalf("event after run_end: %s", line)
				}
				switch e.Type {
				case "step_start":
					if active != "" {
						t.Fatalf("start before %s ended: %s", active, line)
					}
					active = e.Step
					starts = append(starts, e.Step)
				case "agent":
					if active != e.Step {
						t.Fatalf("agent outside active step: %s", line)
					}
					agentEvents++
					message = message || strings.Contains(string(e.Data), "Still researching")
					tool = tool || strings.Contains(string(e.Data), `"name":"WebSearch"`)
					diagnostic = diagnostic || string(e.Data) == `"research diagnostic"`
					if message && tool && diagnostic && !released {
						var report reportFile
						decodeFile(t, filepath.Join(runs(t)[0], "report.json"), &report)
						last := report.Steps[len(report.Steps)-1]
						if active != "research" || last.Name != "research" || last.Status != "running" {
							t.Fatalf("live events arrived after research ended: %+v", report)
						}
						t.Logf("research report while agent events arrived: %s", last.Status)
						if _, err := release.WriteString("continue\n"); err != nil {
							t.Fatal(err)
						}
						released = true
					}
				case "step_result":
					if active != e.Step {
						t.Fatalf("result outside active step: %s", line)
					}
					var result struct{ Status string }
					if err := json.Unmarshal(e.Data, &result); err != nil || result.Status == "" || result.Status == "running" {
						t.Fatalf("missing step result: %s", line)
					}
					active = ""
					results = append(results, e.Step)
				case "question":
					if e.Step != "analysis" || string(e.Data) != `"Which payments?"` || question {
						t.Fatalf("question: %s", line)
					}
					question = true
				case "run_end":
					wantCode := 0
					if fail {
						wantCode = 1
					}
					var end struct {
						Status   string
						ExitCode int `json:"exit_code"`
					}
					if err := json.Unmarshal(e.Data, &end); err != nil {
						t.Fatal(err)
					}
					if active != "" || end.Status != name || end.ExitCode != wantCode {
						t.Fatalf("run end: %s", line)
					}
					ended = true
				default:
					t.Fatalf("unknown event: %s", line)
				}
			}
			if err := <-scanErr; err != nil {
				t.Fatal(err)
			}
			err = cmd.Wait()
			wantCode := 0
			wantSteps := []string{"input", "triage", "research", "analysis", "epics", "design", "export"}
			if fail {
				wantCode, wantSteps = 1, wantSteps[:3]
			}
			if ctx.Err() != nil || cmd.ProcessState.ExitCode() != wantCode {
				t.Fatalf("run: %v, context: %v, stderr: %s", err, ctx.Err(), stderr.String())
			}
			if !ended || !released || question == fail || !reflect.DeepEqual(starts, wantSteps) || !reflect.DeepEqual(results, wantSteps) {
				t.Fatalf("events: starts=%v results=%v ended=%v released=%v question=%v; stderr: %s", starts, results, ended, released, question, stderr.String())
			}
			// Every original agent line survives both in agent.out and as an event.
			log := strings.TrimSpace(read(t, filepath.Join(runs(t)[0], "agent.out")))
			if agentEvents != len(strings.Split(log, "\n")) {
				t.Fatalf("lost agent events: %d, log: %s", agentEvents, log)
			}
		})
	}
}
