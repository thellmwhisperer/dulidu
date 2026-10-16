package export

import (
	"encoding/json"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
)

// maxActive is how many tickets ATM runs at once. ponytail: one, since code cannot tell which tickets touch the same
// files and conflicts_with stays empty; raise it when something fills conflicts_with.
const maxActive = 1

type planTicket struct {
	Issue         int   `json:"issue"`
	DependsOn     []int `json:"depends_on"`
	ConflictsWith []int `json:"conflicts_with"`
}

// plan is plan.json for ATM: every ticket, in order, with the tickets it waits for by the epic graph.
func plan(es pipeline.Epics, nums map[string][]int) []byte {
	p := struct {
		Version   int          `json:"version"`
		MaxActive int          `json:"max_active"`
		Tickets   []planTicket `json:"tickets"`
	}{Version: 1, MaxActive: maxActive, Tickets: []planTicket{}}
	for _, e := range es {
		for _, n := range nums[e.ID] {
			p.Tickets = append(p.Tickets, planTicket{Issue: n, DependsOn: after(e, nums), ConflictsWith: []int{}})
		}
	}
	b, _ := json.MarshalIndent(p, "", "  ") // ints and lists only
	return append(b, '\n')
}
