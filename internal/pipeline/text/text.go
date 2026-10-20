// Package text reads a model's plain-text answer: blocks at lines that start with "## ", each with its "Key: value"
// fields, "### Name" sub-sections, "- item" and "1. item" lists and prose. What the answer lacks reads as empty.
package text

import (
	"regexp"
	"strings"
)

// Block is one part of an answer: its fields, items and prose, and its sub-sections.
type Block struct {
	Name     string
	RawLines []string          // original lines after the heading, including lines parsed as fields
	Fields   map[string]string // keys lower-cased
	Items    []string
	Prose    string // its other lines, joined by newlines
	Sections []Block
}

var (
	field = regexp.MustCompile(`^([A-Za-z][\w -]*):\s*(.*)$`)
	item  = regexp.MustCompile(`^(?:-|\d+\.)\s+(.*)$`)
)

// Parse reads answer into its blocks, the first the lead: the text before the first "## ", named "".
func Parse(answer string) []Block {
	blocks := []Block{{}}
	cur := &blocks[0]
	for _, rawLine := range strings.Split(answer, "\n") {
		line := strings.TrimSpace(rawLine)
		if name, ok := strings.CutPrefix(line, "## "); ok {
			blocks = append(blocks, Block{Name: strings.TrimSpace(name)})
			cur = &blocks[len(blocks)-1]
			continue
		}
		b := &blocks[len(blocks)-1]
		b.RawLines = append(b.RawLines, rawLine)
		if name, ok := strings.CutPrefix(line, "### "); ok {
			b.Sections = append(b.Sections, Block{Name: strings.TrimSpace(name)})
			cur = &b.Sections[len(b.Sections)-1]
			continue
		}
		if cur != b {
			cur.RawLines = append(cur.RawLines, rawLine)
		}
		if m := item.FindStringSubmatch(line); m != nil {
			cur.Items = append(cur.Items, m[1])
		} else if m := field.FindStringSubmatch(line); m != nil {
			if cur.Fields == nil {
				cur.Fields = map[string]string{}
			}
			cur.Fields[strings.ToLower(m[1])] = m[2]
		} else if line != "" {
			cur.Prose = strings.TrimPrefix(cur.Prose+"\n"+line, "\n")
		}
	}
	return blocks
}

// Field is b's value for key, matched without case.
func (b Block) Field(key string) string { return b.Fields[strings.ToLower(key)] }
