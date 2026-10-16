package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thellmwhisperer/dulidu/internal/config"
	"gopkg.in/yaml.v3"
)

// dulidu runs dulidu with args and returns its exit code, stdout and stderr.
func dulidu(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

// repo is a new git repository, the working directory for the rest of the test, under a new home.
func repo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	t.Chdir(dir)
	return dir
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestInitWritesConfigAndIgnoresDuliduOnce(t *testing.T) {
	for _, c := range []struct{ name, before, after string }{
		{"no .gitignore", "", ".dulidu/\n"},
		{"no trailing newline", "node_modules", "node_modules\n.dulidu/\n"},
		{"already ignored", ".dulidu/\ndist/\n", ".dulidu/\ndist/\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := repo(t)
			ignore, yml := filepath.Join(dir, ".gitignore"), filepath.Join(dir, "dulidu.yaml")
			if c.before != "" {
				if err := os.WriteFile(ignore, []byte(c.before), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if code, out, errOut := dulidu(t, "init"); code != 0 {
				t.Fatalf("dulidu init: exit %d\n%s%s", code, out, errOut)
			}
			if got := read(t, ignore); got != c.after {
				t.Fatalf(".gitignore: got %q, want %q", got, c.after)
			}
			first := read(t, yml)
			if !strings.HasPrefix(first, "# ") {
				t.Fatalf("dulidu.yaml is not commented:\n%s", first)
			}
			if _, _, err := config.Load(dir, os.Getenv("HOME")); err != nil {
				t.Fatalf("dulidu.yaml from init does not load: %v", err)
			}
			code, out, _ := dulidu(t, "init")
			if code != 0 || strings.TrimSpace(out) != "dulidu.yaml exists" {
				t.Fatalf("second dulidu init: exit %d, %q", code, out)
			}
			if read(t, ignore) != c.after || read(t, yml) != first {
				t.Fatal("a second dulidu init changed .gitignore or dulidu.yaml")
			}
		})
	}
}

func TestInitKeepsAnExistingConfig(t *testing.T) {
	yml := filepath.Join(repo(t), "dulidu.yaml")
	if err := os.WriteFile(yml, []byte("# mine\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, out, _ := dulidu(t, "init"); code != 0 || strings.TrimSpace(out) != "dulidu.yaml exists" {
		t.Fatalf("dulidu init: exit %d, %q", code, out)
	}
	if got := read(t, yml); got != "# mine\n" {
		t.Fatalf("dulidu init overwrote dulidu.yaml: %q", got)
	}
}

func TestTemplateDeclaresEveryKeyEmpty(t *testing.T) {
	dir := repo(t)
	if code, out, errOut := dulidu(t, "init"); code != 0 {
		t.Fatalf("dulidu init: exit %d\n%s%s", code, out, errOut)
	}
	var fields map[string]any
	if err := yaml.Unmarshal([]byte(read(t, filepath.Join(dir, "dulidu.yaml"))), &fields); err != nil {
		t.Fatal(err)
	}
	want := []string{"harness", "model", "effort", "agent_timeout"}
	if len(fields) != len(want) {
		t.Fatalf("dulidu.yaml has %d keys, want %d: %v", len(fields), len(want), fields)
	}
	for _, key := range want {
		if value, ok := fields[key]; !ok || value != "" {
			t.Errorf("dulidu.yaml %s = %v, want empty string", key, value)
		}
	}
}

func TestDoctorPrintsOneLinePerCheck(t *testing.T) {
	repo(t)
	if code, _, _ := dulidu(t, "init"); code != 0 {
		t.Fatal("dulidu init failed")
	}
	code, out, errOut := dulidu(t, "doctor")
	if code != 0 {
		t.Fatalf("dulidu doctor: exit %d\n%s%s", code, out, errOut)
	}
	var names []string
	for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
		names = append(names, strings.SplitN(l, ":", 2)[0])
	}
	if got := strings.Join(names, "|"); got != "ok go|ok git|ok config" {
		t.Fatalf("got checks %s\n%s", got, out)
	}
}

func TestDoctorFailsWhenACheckFails(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	t.Chdir(t.TempDir())
	code, out, _ := dulidu(t, "doctor")
	if code != 1 || !strings.Contains(out, "fail go:") || !strings.Contains(out, "fail git:") ||
		!strings.Contains(out, "ok config:") {
		t.Fatalf("dulidu doctor with nothing on PATH: exit %d\n%s", code, out)
	}
}

func TestUnknownSubcommandOrFlagIsUsageExit2(t *testing.T) {
	for _, args := range [][]string{{"nope"}, {"--nope"}, {"init", "--nope"}, {"doctor", "extra"}} {
		code, out, errOut := dulidu(t, args...)
		if code != 2 || out != "" || !strings.Contains(errOut, "Usage:") {
			t.Fatalf("dulidu %v: exit %d, stdout %q, stderr %q", args, code, out, errOut)
		}
	}
}
