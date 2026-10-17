package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/thellmwhisperer/dulidu/internal/tui"
)

// statusCmd is dulidu status [label]: the run's label and status, then one line per step, now; with no label, the
// newest run's.
func statusCmd() *cobra.Command {
	return &cobra.Command{Use: "status [label]", Short: "Print a run's status and its steps, the newest run's by default",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := toplevel()
			if err != nil {
				return err
			}
			runs := filepath.Join(root, ".dulidu", "runs")
			var label string
			if len(args) == 1 {
				label = args[0]
			} else {
				entries, _ := os.ReadDir(runs)
				for _, e := range entries {
					if e.IsDir() {
						label = e.Name() // ponytail: labels start with their start time, so the last by name is the newest
					}
				}
				if label == "" {
					return errors.New("no runs")
				}
			}
			r, err := tui.Report(filepath.Join(runs, label))
			if err != nil {
				return fmt.Errorf("run %s: %w", label, err)
			}
			out := cmd.OutOrStdout()
			_, _ = fmt.Fprintln(out, label, r.Status)
			for _, s := range r.Steps {
				_, _ = fmt.Fprintln(out, tui.Line(s, time.Now()))
			}
			return nil
		}}
}
