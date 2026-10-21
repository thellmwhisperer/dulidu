//go:build unix

package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoverDiffNamesTheUncoveredChangedLines(t *testing.T) {
	script, err := filepath.Abs("cover-diff.sh")
	if err != nil {
		t.Fatal(err)
	}
	repo, coverage := t.TempDir(), t.TempDir()
	t.Setenv("GOCOVERDIR", coverage)
	t.Setenv("GOWORK", "off")
	run := func(name string, args ...string) string {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = repo
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, path), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.org/coverage\n\ngo 1.25.0\n")
	base := `package main
import "os"
// Outside main, before.
func main() {
	if os.Args[1] == "a" {
		println("a before")
	}
	if os.Args[1] == "b" {
		println("b before")
		// Inside b, before.
	}
}
`
	write("main.go", base)
	write("main_test.go", "package main\n// Before.\n")
	if err := os.Mkdir(filepath.Join(repo, "testdata"), 0o755); err != nil {
		t.Fatal(err)
	}
	write("testdata/ignored.go", base)
	run("git", "init", "-q")
	run("git", "add", ".")
	// Create the fixture's base without invoking commit hooks or user configuration.
	tree := run("git", "write-tree")
	commit := run("git", "-c", "user.name=Test", "-c", "user.email=test@example.org", "commit-tree", tree, "-m", "base")
	run("git", "update-ref", "HEAD", commit)
	run("git", "update-ref", "refs/remotes/origin/main", commit)
	t.Setenv("COVER_BASE", "")
	write("main.go", strings.ReplaceAll(base, "before", "after"))
	write("main_test.go", "package main\n// After.\n")
	write("testdata/ignored.go", strings.ReplaceAll(base, "before", "after"))
	bin := filepath.Join(t.TempDir(), "example")
	run("go", "build", "-cover", "-o", bin, ".")
	run(bin, "a")
	check := func(t *testing.T, want string, code int) {
		t.Helper()
		before := run("git", "status", "--porcelain")
		cmd := exec.Command("sh", script)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		gotCode := 0
		if err != nil {
			if exit, ok := err.(*exec.ExitError); ok {
				gotCode = exit.ExitCode()
			} else {
				t.Fatal(err)
			}
		}
		if gotCode != code || string(out) != want {
			t.Fatalf("cover-diff: exit %d, want %d\n%s\nwant:\n%s", gotCode, code, out, want)
		}
		if after := run("git", "status", "--porcelain"); before != after {
			t.Fatalf("checker changed the tree: before %q, after %q", before, after)
		}
	}
	want := "cover-diff: these changed lines are not executed by any end-to-end run of the real dulidu binary. Delete them, or write the end-to-end test that reaches them from a real dulidu command.\nmain.go:9\nmain.go:10\n"
	check(t, want, 1)
	check(t, want, 1)
	// Even a blank line and a brace count when an unexecuted block spans them.
	for _, source := range []string{
		strings.Replace(strings.ReplaceAll(base, "before", "after"), "\t\t// Inside b, after.", "", 1),
		strings.Replace(strings.ReplaceAll(base, "before", "after"), "\t\t// Inside b, after.\n\t}", "\t }", 1),
	} {
		write("main.go", source)
		t.Setenv("GOCOVERDIR", t.TempDir())
		run("go", "build", "-cover", "-o", bin, ".")
		run(bin, "a")
		check(t, want, 1)
	}
	write("main.go", strings.ReplaceAll(base, "before", "after"))
	t.Setenv("GOCOVERDIR", coverage)
	run("go", "build", "-cover", "-o", bin, ".")
	// An explicit base also works without origin/main. Counts from both runs combine.
	t.Setenv("COVER_BASE", commit)
	run("git", "update-ref", "-d", "refs/remotes/origin/main")
	run(bin, "b")
	check(t, "cover-diff: 3 changed lines inside covered blocks, all executed by an end-to-end run\n", 0)
	// Scratch configured inside the checked tree must not cause a write there.
	t.Run("scratch inside tree", func(t *testing.T) {
		t.Setenv("TMPDIR", repo)
		check(t, "cover-diff: TMPDIR must be outside the checked tree\n", 1)
	})
	// Only deleted lines, declarations, test files and testdata remain changed.
	write("main.go", strings.Replace(base, "\t\tprintln(\"b before\")\n", "", 1)+"\n// Outside all blocks.\n")
	t.Setenv("GOCOVERDIR", t.TempDir())
	run("go", "build", "-cover", "-o", bin, ".")
	run(bin, "a")
	check(t, "cover-diff: 0 changed lines inside covered blocks, all executed by an end-to-end run\n", 0)
	write("main.go", base)
	check(t, "cover-diff: no Go changes\n", 0)
	t.Setenv("COVER_BASE", "")
	branches := run("git", "branch", "-a")
	check(t, "cover-diff: cannot find the base commit\n"+branches+"\n", 1)
}
