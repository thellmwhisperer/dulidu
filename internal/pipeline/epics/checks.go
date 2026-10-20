package epics

import (
	"fmt"
	"slices"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
)

// Each check is nil when it holds, else "<check>: <what broke it>". Dependencies are pipeline.Epics.Validate's.

// trim keeps the first limit of s, a positive limit, as the reference prototype's hard cap, noting what it cut.
func trim[S ~[]E, E any](s S, limit int, what string, notes *[]string) S {
	if limit <= 0 || len(s) <= limit {
		return s
	}
	*notes = append(*notes, fmt.Sprintf("kept %d of %d %s", limit, len(s), what))
	return s[:limit]
}

// uncovered is nil when a story of es delivers each of musts, functionality ids.
func uncovered(es pipeline.Epics, musts []string) error {
	for _, id := range musts {
		if !slices.ContainsFunc(es, func(e pipeline.EpicOutline) bool {
			return slices.ContainsFunc(e.Stories, func(s pipeline.Story) bool { return slices.Contains(s.Functionalities, id) })
		}) {
			return fmt.Errorf("uncovered must: %s", id)
		}
	}
	return nil
}
