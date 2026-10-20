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
// title, description, acceptance criteria, first test and a list of known functionality ids (which may be empty).
func askStories(e pipeline.EpicOutline, data map[string]any, ids []string, ask func(prompt string) (string, error)) ([]pipeline.Story, error) {
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
		s := pipeline.Story{ID: id, Epic: e.ID, Title: title, Text: description(b, "Functionalities")}
		for _, section := range b.Sections {
			switch strings.ToLower(section.Name) {
			case "acceptance":
				s.Acceptance = section.Items
			case "first test":
				s.FirstTest = strings.TrimSpace(strings.Join(section.RawLines, "\n"))
			}
		}
		if blank(s.ID) || blank(s.Title) || blank(s.FirstTest) || len(s.Acceptance) == 0 || slices.ContainsFunc(s.Acceptance, blank) {
			return nil, fmt.Errorf("stories: %s story %s lacks id, title, acceptance or first test", e.ID, id)
		}
		if blank(s.Text) {
			return nil, fmt.Errorf("stories: %s story %s lacks description", e.ID, id)
		}
		if value := b.Field("Functionalities"); value != "" {
			s.Functionalities = references(value)
		}
		if s.Functionalities == nil || slices.ContainsFunc(s.Functionalities, func(id string) bool { return !slices.Contains(ids, id) }) {
			return nil, fmt.Errorf("stories: %s story %s has invalid functionalities", e.ID, id)
		}
		ss[i] = s
	}
	return ss, nil
}
