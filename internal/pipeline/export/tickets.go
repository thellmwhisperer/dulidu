package export

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
)

// numbers is each epic's ticket numbers by its id: stories count from 1 in epic order, then story order.
func numbers(es pipeline.Epics) map[string][]int {
	nums, n := map[string][]int{}, 0
	for _, e := range es {
		for range e.Stories {
			n++
			nums[e.ID] = append(nums[e.ID], n)
		}
	}
	return nums
}

// after is the tickets a story of e waits for, ascending: every story of the epics e depends on.
func after(e pipeline.EpicOutline, nums map[string][]int) []int {
	deps := []int{}
	for _, d := range e.DependsOn {
		deps = append(deps, nums[d]...)
	}
	slices.Sort(deps)
	return slices.Compact(deps)
}

// tickets is one tickets/<n>.md per story of r, in ticket order: Type: and the ten sections of the ten-point contract,
// each filled from the story alone, Acceptance from its first test, Evidence from its use cases, and ## Depends on from the epic graph when it waits for any ticket.
func tickets(r pipeline.Run, nums map[string][]int) []string {
	ids := map[string]string{}
	for _, f := range functionalities(*r.Analysis) {
		id, _, _ := strings.Cut(f, " ")
		ids[id] = f
	}
	var ts []string
	for _, e := range r.Epics {
		deps := after(e, nums)
		refs := make([]string, len(deps))
		for i, n := range deps {
			refs[i] = fmt.Sprintf("#%d", n)
		}
		waits := "none"
		if len(refs) > 0 {
			waits = strings.Join(refs, ", ")
		}
		for _, s := range e.Stories {
			scope := make([]string, len(s.Functionalities))
			for i, id := range s.Functionalities {
				scope[i] = ids[id]
			}
			kind := cmp.Or(s.Type, "feature")
			parts := []string{"# " + s.Title + "\n", "Type: " + kind + "\n", "## Kind\n", kind + "\n",
				"## Goal\n", strings.TrimSpace(s.Text) + "\n",
				"## Non-goals\n", list(s.NonGoals),
				"## Invariants\n", list(s.Invariants),
				"## Behavioral contract / States\n", list(s.Acceptance),
				"## Scope surface\n", list(scope),
				"## Acceptance\n", "- [ ] " + s.FirstTest + "\n",
				"## Split triggers\n", list(s.SplitTriggers),
				"## Dependencies / Parent\n", list([]string{"Parent: " + e.ID + " " + e.Title + ".", "Story: " + s.ID + ".",
					"Depends on: " + waits + "."})}
			if len(refs) > 0 {
				parts = append(parts, "## Depends on\n", list(refs))
			}
			ts = append(ts, doc(append(parts, "## Evidence (fix / ops)\n", list(s.UseCases),
				"## Validation and rollback\n", "Revert the PR.\n")...))
		}
	}
	return ts
}
