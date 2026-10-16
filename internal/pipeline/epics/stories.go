package epics

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/prompts"
)

type story struct {
	ID, Title, Description string
	Acceptance             []string
	FirstTest              string `json:"first_test"`
	Functionalities        []string
}

// askStories is e's stories, phase two's call for it on data, or why there are none: each story needs an id, a
// title, acceptance criteria and its first test.
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
	var got []story
	if json.Unmarshal([]byte(answer), &got) != nil || len(got) == 0 {
		return nil, errors.New("invalid output: stories of " + e.ID)
	}
	ss := make([]pipeline.Story, len(got))
	for i, s := range got {
		if blank(s.ID) || blank(s.Title) || blank(s.FirstTest) || len(s.Acceptance) == 0 || slices.ContainsFunc(s.Acceptance, blank) {
			return nil, errors.New("invalid output: a story of " + e.ID + " lacks id, title, acceptance or first test")
		}
		if blank(s.Description) {
			return nil, errors.New("invalid output: a story of " + e.ID + " lacks description")
		}
		if s.Functionalities == nil || slices.ContainsFunc(s.Functionalities, func(id string) bool { return !slices.Contains(ids, id) }) {
			return nil, errors.New("invalid output: a story of " + e.ID + " has invalid functionalities")
		}
		ss[i] = pipeline.Story{ID: s.ID, Epic: e.ID, Title: s.Title, Text: s.Description, Acceptance: s.Acceptance,
			FirstTest: s.FirstTest, Functionalities: s.Functionalities}
	}
	return ss, nil
}
