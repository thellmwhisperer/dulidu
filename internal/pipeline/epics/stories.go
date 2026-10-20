package epics

import (
	"fmt"
	"slices"
	"strings"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/prompts"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/text"
)

// askStories is e's stories, phase two's call for it on data, or why there are none: each story needs an id,
// title; other fields are optional. Unknown functionality ids are dropped with a note.
func askStories(e pipeline.EpicOutline, data map[string]any, ids []string, notes *[]string, ask func(prompt string) (string, error)) ([]pipeline.Story, error) {
	data["Epic"] = e
	prompt, err := prompts.Render("p16-epics-stories", data)
	var answer string
	if err == nil {
		answer, err = ask(prompt)
	}
	if err != nil {
		return nil, fmt.Errorf("stories: %s: %w", e.ID, err)
	}
	blocks := text.Parse(answer)[1:]
	if len(blocks) == 0 {
		return nil, fmt.Errorf("stories: %s has no story blocks", e.ID)
	}
	ss := make([]pipeline.Story, len(blocks))
	for i, b := range blocks {
		id, title := heading(b)
		s := pipeline.Story{ID: id, Epic: e.ID, Title: title, Text: description(b, "Functionalities", "Type"),
			Type: strings.ToLower(b.Field("Type"))}
		for _, section := range b.Sections {
			switch strings.ToLower(section.Name) {
			case "acceptance":
				s.Acceptance = section.Items
			case "first test":
				s.FirstTest = strings.TrimSpace(strings.Join(section.RawLines, "\n"))
			case "use cases":
				s.UseCases = section.Items
			case "invariants":
				s.Invariants = section.Items
			case "non-goals":
				s.NonGoals = section.Items
			case "split triggers":
				s.SplitTriggers = section.Items
			}
		}
		if blank(s.ID) || blank(s.Title) {
			return nil, fmt.Errorf("stories: %s story %s lacks id or title", e.ID, id)
		}
		if value := b.Field("Functionalities"); value != "" {
			s.Functionalities = references(value)
		}
		s.Functionalities = slices.DeleteFunc(s.Functionalities, func(id string) bool {
			if slices.Contains(ids, id) {
				return false
			}
			*notes = append(*notes, fmt.Sprintf("dropped functionality %s from %s in %s", id, s.ID, e.ID))
			return true
		})
		ss[i] = s
	}
	return ss, nil
}
