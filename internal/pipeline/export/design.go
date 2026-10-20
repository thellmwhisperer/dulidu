package export

import "github.com/thellmwhisperer/dulidu/internal/pipeline"

// design is brief-diseno.md for Claude Design: who uses the product, what they go through, what it does and within
// which limits.
func design(a pipeline.Analysis) string {
	return doc("# Brief de diseño\n", "## Personas\n", list(pipeline.Lines(a.Personas)), "## Flows\n",
		list(pipeline.Lines(a.Flows)), "## Functionalities\n", list(pipeline.Lines(a.Functionalities)),
		"## Constraints\n", list(a.Constraints))
}
