package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/spf13/cobra"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
	"github.com/thellmwhisperer/dulidu/internal/tui"
)

// runsCmd is dulidu runs: one line per run folder in .dulidu/runs, newest first: label, status, last step, started.
func runsCmd() *cobra.Command {
	return &cobra.Command{Use: "runs", Short: "List the runs in .dulidu/runs, newest first", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := toplevel()
			if err != nil {
				return err
			}
			dir := filepath.Join(root, ".dulidu", "runs")
			entries, err := os.ReadDir(dir)
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			} else if err != nil {
				return err
			}
			type run struct {
				label  string
				report pipeline.Report
			}
			var runs []run
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				r, err := tui.Report(filepath.Join(dir, e.Name()))
				if err != nil {
					return fmt.Errorf("run %s: %w", e.Name(), err)
				}
				runs = append(runs, run{e.Name(), r})
			}
			slices.SortStableFunc(runs, func(a, b run) int { return b.report.Started.Compare(a.report.Started) })
			for _, r := range runs {
				last := "-"
				if n := len(r.report.Steps); n > 0 {
					last = r.report.Steps[n-1].Name
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), r.label, r.report.Status, last, r.report.Started.Format(time.RFC3339))
			}
			return nil
		}}
}
