// Package config is dulidu's configuration in layers, each overriding the one before: the built-in defaults,
// ~/.config/dulidu/config.yaml, then the repository's dulidu.yaml.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is dulidu's configuration: the agent CLI doctor greets, its model and effort, passed through untouched,
// and how long it may take, a Go duration, "" for the default; then the project the steps work on and each
// step's agent, every one a line in dulidu init's template. Any other key in a layer is an error.
type Config struct {
	Harness           string   `yaml:"harness"`
	Model             string   `yaml:"model"`
	Effort            string   `yaml:"effort"`
	AgentTimeout      string   `yaml:"agent_timeout"`
	Platform          string   `yaml:"platform"`
	ProjectType       string   `yaml:"project_type"`
	MaxEpics          int      `yaml:"max_epics"`
	MaxStoriesPerEpic int      `yaml:"max_stories_per_epic"`
	TicketSize        string   `yaml:"ticket_size"`
	Exclusions        []string `yaml:"exclusions"`
	VideoFrameSeconds int      `yaml:"video_frame_seconds"`
	Steps             Steps    `yaml:"steps"`

	set map[string]bool // every key a layer declared, dotted, empty or not
}

// Layers is the files Load reads, in order: the machine's under home, then the repository's under root.
func Layers(root, home string) []string {
	return []string{filepath.Join(home, ".config", "dulidu", "config.yaml"), filepath.Join(root, "dulidu.yaml")}
}

// Load is the defaults overridden by every layer that exists; found is those layers. A missing layer is no error.
func Load(root, home string) (c Config, found []string, err error) {
	for _, path := range Layers(root, home) {
		ok, err := decode(path, &c)
		if err != nil {
			return c, found, err
		}
		if ok {
			found = append(found, path)
		}
	}
	return c, found, nil
}

// decode decodes the file at path, if any, over c and says whether there was one. A key c lacks is an error.
func decode(path string, c *Config) (bool, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var n yaml.Node
	if err := yaml.Unmarshal(b, &n); err != nil {
		return true, fmt.Errorf("%s: %w", path, err)
	}
	if err := c.declare(&n, ""); err != nil {
		return true, fmt.Errorf("%s: %w", path, err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(c); err != nil && !errors.Is(err, io.EOF) {
		return true, fmt.Errorf("%s: %w", path, err)
	}
	return true, nil
}
