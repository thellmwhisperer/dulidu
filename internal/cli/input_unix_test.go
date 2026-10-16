//go:build unix

package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRejectsAVideoAttachmentBeforeAnyAgentCall(t *testing.T) {
	calls := runRepo(t, map[string]string{"demo.mp4": "mp4", "shot.png": "png"})
	code, _, errOut := dulidu(t, "run", "brief.md", "--attach", "shot.png", "--attach", "demo.mp4")
	if code != 1 || !strings.Contains(errOut, "unsupported attachment: demo.mp4") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if _, err := os.Stat(calls); err == nil {
		t.Error("the agent was called")
	}
	if dirs := runs(t); len(dirs) != 0 {
		t.Errorf("a run folder was written: %v", dirs)
	}
}

func TestDoctorAgentNeedsNoMediaTool(t *testing.T) {
	agentRepo(t, "pi", "ok", "")
	bin := strings.SplitN(os.Getenv("PATH"), string(os.PathListSeparator), 2)[0] // the fake pi alone
	for _, name := range []string{"go", "git"} {
		path, err := exec.LookPath(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(path, filepath.Join(bin, name)); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	if code, out, errOut := dulidu(t, "doctor", "--agent"); code != 0 {
		t.Fatalf("dulidu doctor --agent with only go, git and the agent: exit %d\n%s%s", code, out, errOut)
	}
}
