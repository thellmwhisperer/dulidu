// Package config is dulidu's configuration in layers, each overriding the one before: the built-in defaults,
// ~/.config/dulidu/config.yaml, then the repository's dulidu.yaml. Copied from ATM's internal/config, with
// only dulidu's keys.
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

// Config is dulidu's configuration: the agent CLI, its model and effort, passed through untouched, and how long
// it may take, a Go duration, "" for the default. The prompts (#4) bring theirs, each with a line in dulidu
// init's template. Any other key in a layer is an error.
type Config struct {
	Harness      string `yaml:"harness"`
	Model        string `yaml:"model"`
	Effort       string `yaml:"effort"`
	AgentTimeout string `yaml:"agent_timeout"`
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

// decode decodes the file at path, if any, over v and says whether there was one. A key v lacks is an error.
func decode(path string, v any) (bool, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(v); err != nil && !errors.Is(err, io.EOF) {
		return true, fmt.Errorf("%s: %w", path, err)
	}
	return true, nil
}
