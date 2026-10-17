package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
				var mod time.Time
				for _, e := range entries {
					if !e.IsDir() {
						continue
					}
					m := time.Time{}
					if info, err := e.Info(); err == nil {
						m = info.ModTime()
					}
					if label == "" || newer(e.Name(), m, label, mod) {
						label, mod = e.Name(), m
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

// began is the start time a run's label begins with, before its serial: 20261017-010000-3 began 20261017-010000.
func began(label string) string {
	if i := strings.LastIndex(label, "-"); i > 0 {
		return label[:i]
	}
	return label
}

// newer says whether the run labelled a, its folder last changed at ma, started after the one labelled b, changed
// at mb: by label across seconds, by its folder's changes inside one.
func newer(a string, ma time.Time, b string, mb time.Time) bool {
	if x, y := began(a), began(b); x != y {
		return x > y
	}
	return ma.After(mb)
}
