//go:build unix

// Package scripts holds the tests of the repository's shell scripts.
package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// sh runs the script name from the repository root with args and returns its combined output and error.
func sh(t *testing.T, name string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("sh", append([]string{filepath.Join("scripts", name)}, args...)...)
	cmd.Dir = ".."
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// TestReadmeFirstRunReplays runs the README's first-run block on the recorded example: it passes and shows the
// commands it ran.
func TestReadmeFirstRunReplays(t *testing.T) {
	for _, tool := range []string{"pdftotext", "ffmpeg", "ffprobe", "whisper-cli"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed", tool)
		}
	}
	out, err := sh(t, "readme-check.sh")
	if err != nil || !strings.Contains(out, "$ dulidu run brief.md") {
		t.Fatalf("readme-check.sh: %v\n%s", err, out)
	}
}

func TestBuiltDuliduReadmeWritesCoverage(t *testing.T) {
	coverage := t.TempDir()
	t.Setenv("GOCOVERDIR", coverage)
	TestReadmeFirstRunReplays(t)
	counters, err := filepath.Glob(filepath.Join(coverage, "covcounters.*"))
	if err != nil || len(counters) == 0 {
		t.Fatalf("README dulidu commands wrote no coverage: counters=%v, error=%v", counters, err)
	}
}

// TestLeakScan passes on the README and docs/, and fails on each kind of leak, naming it.
func TestLeakScan(t *testing.T) {
	if out, err := sh(t, "leak-scan.sh", "README.md", "docs/"); err != nil {
		t.Fatalf("leak-scan.sh README.md docs/: %v\n%s", err, out)
	}
	dir := t.TempDir()
	clean := filepath.Join(dir, "clean.md")
	if err := os.WriteFile(clean, []byte("dulidu 1.0.2 runs in ~/work, see https://example.com/a-b.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := sh(t, "leak-scan.sh", clean); err != nil {
		t.Errorf("clean file: %v\n%s", err, out)
	}
	for _, leak := range []string{"/Users/someone/repo", "write to someone@example.org", "host 192.168.1.20 up",
		"token ghp_abcdefghijklmnop1234", "key " + strings.Repeat("a1", 24)} {
		f := filepath.Join(dir, "leak.md")
		if err := os.WriteFile(f, []byte("text\n"+leak+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if out, err := sh(t, "leak-scan.sh", f); err == nil || !strings.Contains(out, leak) {
			t.Errorf("%q: err %v, output %q", leak, err, out)
		}
	}
}
