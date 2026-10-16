package config

import (
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestProjectLaterLayerNullEmptiesEarlierValue(t *testing.T) {
	p := load(t, full(t), "max_epics:\nticket_size: ~\nexclusions: null\nsteps:\n  triage:\n    model:\n")
	if err := p.Complete(); err != nil {
		t.Fatalf("keys across layers: %v", err)
	}
	if got := p.Step("triage"); got.Model != "" || got.Harness != "claude" {
		t.Fatalf("triage: got %+v, want model emptied by the repository, harness claude from the machine", got)
	}
	var c struct {
		MaxEpics          int      `yaml:"max_epics"`
		MaxStoriesPerEpic int      `yaml:"max_stories_per_epic"`
		TicketSize        string   `yaml:"ticket_size"`
		Exclusions        []string `yaml:"exclusions"`
	}
	b, err := yaml.Marshal(p)
	if err == nil {
		err = yaml.Unmarshal(b, &c)
	}
	if err != nil || c.MaxEpics != 0 || c.TicketSize != "" || len(c.Exclusions) != 0 || c.MaxStoriesPerEpic != 8 {
		t.Fatalf("nulls over machine values: got %+v, %v", c, err)
	}
}

func TestProjectUnknownEmptyMappingIsNamed(t *testing.T) {
	for _, c := range []struct{ yaml, key string }{
		{"foo: {}\n", "foo"},
		{"steps:\n  deploy: {}\n", "steps.deploy"},
		{"foo:\n  bar: {}\n", "foo.bar"},
	} {
		root, home := t.TempDir(), t.TempDir()
		path := filepath.Join(root, "dulidu.yaml")
		write(t, path, full(t)+c.yaml)
		_, _, err := Load(root, home)
		if err == nil || err.Error() != path+": config incomplete: "+c.key {
			t.Fatalf("unknown %s: %v", c.key, err)
		}
	}
	p := load(t, "", full(t, "triage: {harness: claude, model: strong, effort: high}", "triage: {}"))
	if err := p.Complete(); err == nil || !strings.Contains(err.Error(), "config incomplete: steps.triage.harness") {
		t.Fatalf("known empty step: %v", err)
	}
}
