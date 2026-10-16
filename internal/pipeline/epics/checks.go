package epics

import (
	"fmt"
	"slices"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
)

// Each check is nil when it holds, else "<check>: <what broke it>". Dependencies are pipeline.Epics.Validate's.

// overCap is nil when n of what fit in limit: more fails the step, nothing is cut.
func overCap(n, limit int, what string) error {
	if n <= limit {
		return nil
	}
	return fmt.Errorf("over cap: %d %s, cap %d", n, what, limit)
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
