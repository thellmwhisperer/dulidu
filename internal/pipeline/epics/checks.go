package epics

import (
	"fmt"
	"slices"
	"strings"

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

func uncovered(es pipeline.Epics, musts []string, notes *[]string) {
	missing := slices.DeleteFunc(slices.Clone(musts), func(id string) bool {
		return slices.ContainsFunc(es, func(e pipeline.EpicOutline) bool {
			return slices.ContainsFunc(e.Stories, func(s pipeline.Story) bool { return slices.Contains(s.Functionalities, id) })
		})
	})
	if len(missing) > 0 {
		*notes = append(*notes, "uncovered must: "+strings.Join(missing, ", "))
	}
}
