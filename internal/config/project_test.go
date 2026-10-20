package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
)

// project is a config whose run needs every key: it says which one is missing and puts a step in the report.
type project interface {
	Complete() error
	Step(name string) pipeline.Step
}

// full is testdata/full.yaml, every key set, with each old replaced by its new.
func full(t *testing.T, oldnew ...string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "full.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.NewReplacer(oldnew...).Replace(string(b))
}

// load loads machine and repository layers, "" for none, and fails the test on a load error.
func load(t *testing.T, machine, repository string) project {
	t.Helper()
	root, home := t.TempDir(), t.TempDir()
	if machine != "" {
		write(t, filepath.Join(home, ".config", "dulidu", "config.yaml"), machine)
	}
	if repository != "" {
		write(t, filepath.Join(root, "dulidu.yaml"), repository)
	}
	c, _, err := Load(root, home)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	p, ok := any(c).(project)
	if !ok {
		t.Fatal("Config does not say whether a run has every key")
	}
	return p
}

func TestProjectLoadsEveryKeyWithItsStepAgents(t *testing.T) {
	p := load(t, "", full(t))
	if err := p.Complete(); err != nil {
		t.Fatalf("every key set: %v", err)
	}
	report := pipeline.Report{Status: "running", Steps: []pipeline.Step{p.Step("research"), p.Step("epics")}}
	b, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var back pipeline.Report
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	want := []pipeline.Step{{Name: "research", Harness: "pi", Model: "medium"}, {Name: "epics", Harness: "claude", Model: "medium"}}
	if len(back.Steps) != 2 || back.Steps[0] != want[0] || back.Steps[1] != want[1] {
		t.Fatalf("report.json steps: got %+v, want %+v", back.Steps, want)
	}
}

func TestProjectMissingKeyIsIncompleteButEmptyIsNone(t *testing.T) {
	triage := "triage: {harness: claude, model: strong, effort: high}"
	p := load(t, "", full(t, triage, "triage: {harness: claude, effort: high}"))
	if err := p.Complete(); err == nil || err.Error() != "config incomplete: steps.triage.model" {
		t.Fatalf("steps.triage.model missing: %v", err)
	}
	p = load(t, "", full(t, triage, `triage: {harness: claude, model: "", effort: high}`, "max_epics: 5", "max_epics:"))
	if err := p.Complete(); err != nil {
		t.Fatalf("steps.triage.model and max_epics empty: %v", err)
	}
	if got := p.Step("triage"); got.Model != "" || got.Harness != "claude" {
		t.Fatalf("empty model: got %+v", got)
	}
}

func TestProjectUnknownKeyIsNamed(t *testing.T) {
	for _, c := range []struct{ yaml, key string }{
		{"foo: 1\n", "foo"},
		{"steps:\n  triage:\n    foo: x\n", "steps.triage.foo"},
		{"steps:\n  deploy: {model: x}\n", "steps.deploy.model"},
	} {
		root, home := t.TempDir(), t.TempDir()
		path := filepath.Join(root, "dulidu.yaml")
		write(t, path, full(t)+c.yaml)
		_, _, err := Load(root, home)
		if err == nil || !strings.Contains(err.Error(), path+": config incomplete: "+c.key) {
			t.Fatalf("unknown %s: %v", c.key, err)
		}
	}
}

func TestProjectRejectsSecondYAMLDocument(t *testing.T) {
	for _, c := range []struct{ extra, want string }{
		{"foo: 1", "config incomplete: foo"},
		{"max_epics: 3", "config incomplete: multiple documents"},
	} {
		root, home := t.TempDir(), t.TempDir()
		path := filepath.Join(root, "dulidu.yaml")
		write(t, path, full(t)+"---\n"+c.extra+"\n")
		_, _, err := Load(root, home)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("second YAML document %q: %v", c.extra, err)
		}
	}
}

func TestProjectLaterLayerOverridesOnlyWhatItSets(t *testing.T) {
	p := load(t, full(t), "steps:\n  research:\n    model: deep\nagent_timeout: \"\"\n")
	if err := p.Complete(); err != nil {
		t.Fatalf("keys across layers: %v", err)
	}
	if got := p.Step("research"); got.Harness != "pi" || got.Model != "deep" {
		t.Fatalf("research: got %+v, want harness pi from the machine, model deep from the repository", got)
	}
	if got := p.Step("epics"); got.Harness != "claude" || got.Model != "medium" {
		t.Fatalf("epics, set only by the machine: got %+v", got)
	}
	var c struct {
		AgentTimeout string `yaml:"agent_timeout"`
		MaxEpics     int    `yaml:"max_epics"`
	}
	b, err := yaml.Marshal(p)
	if err == nil {
		err = yaml.Unmarshal(b, &c)
	}
	if err != nil || c.AgentTimeout != "" || c.MaxEpics != 5 {
		t.Fatalf("agent_timeout empty over 20m, max_epics kept: got %q, %d, %v", c.AgentTimeout, c.MaxEpics, err)
	}
}

// Every layer rejects removed keys, even empty ones, before checking their former value types.
func TestProjectRejectsRemovedKeysInEachLayer(t *testing.T) {
	for _, machine := range []bool{false, true} {
		for _, key := range []string{"platform", "project_type", "ticket_size", "exclusions", "steps.stories"} {
			for _, value := range []string{`""`, "null", "{}", "{model: old}", "[]"} {
				root, home := t.TempDir(), t.TempDir()
				path := filepath.Join(root, "dulidu.yaml")
				if machine {
					path = filepath.Join(home, ".config", "dulidu", "config.yaml")
				}
				body := key + ": " + value + "\n"
				if key == "steps.stories" {
					body = "steps:\n  stories: " + value + "\n"
				}
				write(t, path, body)
				_, _, err := Load(root, home)
				if err == nil || err.Error() != path+": "+key+" is no longer read; delete it" {
					t.Errorf("%s in %s: %v", body, path, err)
				}
			}
		}
	}
}
