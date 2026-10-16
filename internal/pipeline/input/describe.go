package input

import "fmt"

// prompt asks the input step's cheap model to describe the image at path, relative to the run folder.
func prompt(path string) string {
	return fmt.Sprintf("Read the image file %s, an attachment of the brief. Describe it in plain text: what it "+
		"shows, every readable text in it, and its layout. Answer with the description alone, without preamble or "+
		"headings.", path)
}
