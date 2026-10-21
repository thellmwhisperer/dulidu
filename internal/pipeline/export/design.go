package export

import "github.com/thellmwhisperer/dulidu/internal/pipeline"

// design is brief-diseno.md for a design tool: each screen the design step drew, with its purpose, wireframe,
// components, navigation, states and accessibility, or one line saying there are none.
func design(screens []pipeline.Screen) string {
	parts := []string{"# Brief de diseño\n"}
	if len(screens) == 0 {
		parts = append(parts, "No screens.\n")
	}
	for _, s := range screens {
		parts = append(parts, "## "+s.Name+"\n", s.Purpose+"\n", "```\n"+s.Wireframe+"\n```\n",
			"**Components:**\n", list(s.Components), "**Navigation:**\n", s.Navigation+"\n", "**States:**\n",
			list(s.States), "**Accessibility:**\n", s.Accessibility+"\n")
	}
	return doc(parts...)
}
