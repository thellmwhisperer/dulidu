package cli

import (
	"cmp"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/thellmwhisperer/dulidu/internal/config"
)

// doctorCmd is dulidu doctor: the go toolchain, git and the config layers found, one line each. Every check runs;
// any failure fails the command.
func doctorCmd() *cobra.Command {
	return &cobra.Command{Use: "doctor", Short: "Check what dulidu needs", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			failed := 0
			check := func(name, detail string, err error) {
				line := "ok " + name + ": " + detail
				if err != nil {
					failed++
					line = "fail " + name + ": " + err.Error()
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), strings.Join(strings.Fields(line), " "))
			}
			path, err := exec.LookPath("go")
			check("go", path, err)
			root, err := toplevel()
			check("git", root, err)
			var found []string
			home, err := os.UserHomeDir()
			if err == nil {
				_, found, err = config.Load(cmp.Or(root, "."), home)
			}
			check("config", cmp.Or(strings.Join(found, " "), "no layer, the defaults"), err)
			if failed > 0 {
				return fmt.Errorf("doctor: %d of the checks failed", failed)
			}
			return nil
		}}
}
