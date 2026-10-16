package input

import "fmt"

// prompt asks the input step's cheap model to describe the image at path, relative to the run folder; a contact
// sheet's says it holds a video's frames.
func prompt(path string, sheet bool, seconds int) string {
	what := "an attachment of the brief"
	if sheet {
		what = fmt.Sprintf("a contact sheet of a video's frames, one every %d seconds, in reading order", seconds)
	}
	return fmt.Sprintf("Read the image file %s, %s. Describe it in plain text: what it shows, every readable "+
		"text in it, and its layout. Answer with the description alone, without preamble or headings.", path, what)
}
