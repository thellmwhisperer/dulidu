//go:build unix

package pipeline_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
)

// TestBinaryRunReadsPastBadAttachments runs the built dulidu with an attachment it cannot read: an unsupported
// extension, a PDF without pdftotext on PATH, and a PDF pdftotext gets no text from. The run is done, the file is in
// input/, and its section in contexto.md is one line saying it was not read and why.
func TestBinaryRunReadsPastBadAttachments(t *testing.T) {
	bin := binary(t)
	for _, tc := range []struct {
		name, file, pdftotext, reason string
	}{
		{"unsupported extension", "notes.xyz", "", "does not read"},
		{"pdf without pdftotext", "terms.pdf", "", "pdftotext is not installed"},
		{"pdf without text", "terms.pdf", "printf ' \\f\\n\\t'\n", "no text"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := realTempDir(t)
			t.Setenv("HOME", t.TempDir())
			if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
				t.Fatalf("git init: %v\n%s", err, out)
			}
			files := map[string]string{"brief.md": binaryRead(t, filepath.Join(binaryExample, "brief.md")),
				tc.file: binaryRead(t, filepath.Join(binaryExample, "one.pdf"))}
			for name, body := range files {
				if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			// Only the tools the run needs are on PATH, and a fake pdftotext when the case has one.
			tools := t.TempDir()
			for _, name := range []string{"git", "cat", "wc", "ps"} {
				path, err := exec.LookPath(name)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(path, filepath.Join(tools, name)); err != nil {
					t.Fatal(err)
				}
			}
			if tc.pdftotext != "" {
				if err := os.WriteFile(filepath.Join(tools, "pdftotext"), []byte("#!/bin/sh\n"+tc.pdftotext), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", tools)
			// The attachment is no image, so the input step asks nothing.
			answers, _ := filepath.Glob(filepath.Join(binaryExample, "answers", "*"))
			answers = answers[1:]
			replay := fakeReplay(t, answers)
			for _, args := range [][]string{{"init"}, {"run", "brief.md", "--attach", tc.file}} {
				cmd := exec.Command(bin, args...)
				cmd.Dir = repo
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("dulidu %s: %v\n%s", strings.Join(args, " "), err, out)
				}
			}

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
			if got := binaryRead(t, filepath.Join(dirs[0], "input", tc.file)); got != files[tc.file] {
				t.Errorf("input/%s is not the attachment", tc.file)
			}
			context := binaryRead(t, filepath.Join(dirs[0], "contexto.md"))
			_, section, _ := strings.Cut(context, "## "+tc.file+"\n")
			if section = strings.TrimSpace(section); strings.Contains(section, "\n") ||
				!strings.HasPrefix(section, "Not read: ") || !strings.Contains(section, tc.reason) {
				t.Errorf("section of %s, want one line saying it was not read because %s:\n%s", tc.file, tc.reason, context)
			}
		})
	}
}
