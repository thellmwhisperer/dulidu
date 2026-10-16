// Package prompts is the steps' prompts, one text/template per file, embedded in the binary.
package prompts

import (
	"embed"
	"fmt"
	"math"
	"strings"
	"text/template"
)

//go:embed *.md.tmpl
var files embed.FS

// templates fail on a key their data lacks, naming it. seventy is the lower story target, 70% of the maximum, rounded.
var templates = template.Must(template.New("").
	Funcs(template.FuncMap{"seventy": func(n int) int { return int(math.Round(float64(n) * 0.7)) }}).
	Option("missingkey=error").ParseFS(files, "*.md.tmpl"))

// Render is the prompt of step, a file name without .md.tmpl, filled with data: context text and config values.
func Render(step string, data map[string]any) (string, error) {
	t := templates.Lookup(step + ".md.tmpl")
	if t == nil {
		return "", fmt.Errorf("prompt %s: none", step)
	}
	var b strings.Builder
	if err := t.Execute(&b, data); err != nil {
		return "", fmt.Errorf("prompt %s: %w", step, err)
	}
	return b.String(), nil
}
