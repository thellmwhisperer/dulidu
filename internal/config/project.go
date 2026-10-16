package config

import (
	"fmt"
	"slices"

	"gopkg.in/yaml.v3"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
)

// Step is one step's agent: its CLI, model and effort, passed through untouched.
type Step struct {
	Harness string `yaml:"harness"`
	Model   string `yaml:"model"`
	Effort  string `yaml:"effort"`
}

// Steps is every step's agent. A layer sets a step key by key, keeping what an earlier one set for the rest.
type Steps struct {
	Input    Step `yaml:"input"`
	Triage   Step `yaml:"triage"`
	Research Step `yaml:"research"`
	Analysis Step `yaml:"analysis"`
	Epics    Step `yaml:"epics"`
	Stories  Step `yaml:"stories"`
}

// required is every key a run needs some layer to declare, in the order Complete names the first missing.
var required = func() []string {
	keys := []string{"platform", "project_type", "max_epics", "max_stories_per_epic", "ticket_size", "exclusions",
		"agent_timeout", "video_frame_seconds"}
	for _, step := range []string{"input", "triage", "research", "analysis", "epics", "stories"} {
		for _, key := range []string{"harness", "model", "effort"} {
			keys = append(keys, "steps."+step+"."+key)
		}
	}
	return keys
}()

// known is every key a layer may declare: the required ones and doctor's agent.
var known = append([]string{"harness", "model", "effort"}, required...)

// declare records every key n declares, dotted under prefix, in c.set. A key Config lacks is an error.
func (c *Config) declare(n *yaml.Node, prefix string) error {
	if n.Kind == yaml.DocumentNode && len(n.Content) == 1 {
		n = n.Content[0]
	}
	if n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, value := prefix+n.Content[i].Value, n.Content[i+1]
		if value.Kind == yaml.MappingNode {
			if err := c.declare(value, key+"."); err != nil {
				return err
			}
			continue
		}
		if !slices.Contains(known, key) {
			return fmt.Errorf("config incomplete: %s", key)
		}
		if c.set == nil {
			c.set = map[string]bool{}
		}
		c.set[key] = true
	}
	return nil
}

// Complete is nil when the layers Load read declared every key a run needs, even empty, and else names the
// first one none did.
func (c Config) Complete() error {
	for _, key := range required {
		if !c.set[key] {
			return fmt.Errorf("config incomplete: %s", key)
		}
	}
	return nil
}

// Step is the named step as report.json records it: its name, harness and model. An unknown name has neither.
func (c Config) Step(name string) pipeline.Step {
	s := map[string]Step{"input": c.Steps.Input, "triage": c.Steps.Triage, "research": c.Steps.Research,
		"analysis": c.Steps.Analysis, "epics": c.Steps.Epics, "stories": c.Steps.Stories}[name]
	return pipeline.Step{Name: name, Harness: s.Harness, Model: s.Model}
}
