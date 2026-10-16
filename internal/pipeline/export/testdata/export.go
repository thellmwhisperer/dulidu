// Command export exports the run folder given, as dulidu would once its epics pass.
package main

import (
	"log"
	"os"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/export"
)

func main() {
	r, err := pipeline.Read(os.Args[1])
	if err == nil {
		err = export.Write(os.Args[1], &r)
	}
	if err != nil {
		log.Fatal(err)
	}
}
