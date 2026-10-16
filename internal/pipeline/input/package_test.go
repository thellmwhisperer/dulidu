//go:build unix

package input

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
)

// need skips the test when a tool it needs is not installed.
func need(t *testing.T, tools ...string) {
	t.Helper()
	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed", tool)
		}
	}
}

// fakeBin puts sh scripts by name first in PATH, or alone when only is set.
func fakeBin(t *testing.T, only bool, scripts map[string]string) {
	t.Helper()
	bin := t.TempDir()
	for name, script := range scripts {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if !only {
		bin += string(os.PathListSeparator) + os.Getenv("PATH")
	}
	t.Setenv("PATH", bin)
}

// load is the input of a brief and the files at paths.
func load(t *testing.T, paths ...string) pipeline.InputContext {
	t.Helper()
	brief := filepath.Join(t.TempDir(), "brief.md")
	if err := os.WriteFile(brief, []byte("A shop for second-hand bikes.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	in, err := Load(brief, paths)
	if err != nil {
		t.Fatal(err)
	}
	return in
}

// pack writes in to a new run folder the way dulidu run does, then packages it with a fake agent; it returns the
// folder, contexto.md, the prompts the agent got, and Package's error.
func pack(t *testing.T, in pipeline.InputContext) (dir, context string, asked []string, err error) {
	t.Helper()
	dir = t.TempDir()
	if err := pipeline.Write(dir, pipeline.Run{Report: pipeline.Report{Status: "running", Started: time.Now()}, Input: &in}); err != nil {
		t.Fatal(err)
	}
	context, err = Package(dir, in, func(prompt string) (string, error) {
		asked = append(asked, prompt)
		return "A fake description.", nil
	})
	return dir, context, asked, err
}

func TestPackageTestdata(t *testing.T) {
	need(t, "ffprobe", "pdftotext")
	names := []string{"notes.md", "terms.pdf", "pattern.png", "blue.jpg"}
	var paths []string
	for _, name := range names {
		paths = append(paths, filepath.Join("testdata", name))
	}
	in := load(t, paths...)
	dir, context, asked, err := pack(t, in)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("contexto.md:\n%s", context)
	if n := strings.Count(context, "\n## "); n != 5 {
		t.Errorf("want one heading per file, 5, got %d", n)
	}
	for _, row := range []string{"| pattern.png | image | 469 bytes | 64x48 |", "| blue.jpg | image | 232 bytes | 40x30 |"} {
		if !strings.Contains(context, row) {
			t.Errorf("index lacks %q", row)
		}
	}
	for _, want := range []string{"Sellers list a bike", "Refunds within fourteen days."} {
		if !strings.Contains(context, want) {
			t.Errorf("contexto.md lacks %q", want)
		}
	}
	if len(asked) != 2 || !strings.Contains(asked[0], "input/blue.jpg") || !strings.Contains(asked[1], "input/pattern.png") {
		t.Errorf("want one ask per image, got %q", asked)
	}
	for _, name := range []string{"pattern.png", "blue.jpg"} {
		section := strings.SplitN(strings.SplitN(context, "## "+name+"\n", 2)[1], "\n## ", 2)[0]
		if !strings.Contains(section, "### Description\n\nA fake description.") {
			t.Errorf("%s has no description under its heading", name)
		}
	}
	for _, name := range names {
		if read(t, filepath.Join(dir, "input", name)) != read(t, filepath.Join("testdata", name)) {
			t.Errorf("input/%s is not the attachment byte for byte", name)
		}
	}
	if _, context2, _, err := pack(t, in); err != nil || context2 != context {
		t.Errorf("contexto.md differs between two runs (%v):\n%s", err, context2)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
