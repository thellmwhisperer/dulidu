package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/thellmwhisperer/dulidu/internal/tui"
)

// attachCmd is dulidu attach <label>: the run's live view on a terminal, q detaches and the run goes on; anything
// else gets one line per step, as each finishes, until the run ends.
func attachCmd() *cobra.Command {
	return &cobra.Command{Use: "attach <label>", Short: "Watch a run: pipeline box and agent box",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := toplevel()
			if err != nil {
				return err
			}
			dir := filepath.Join(root, ".dulidu", "runs", args[0])
			if _, err := os.Stat(dir); err != nil {
				return fmt.Errorf("no run %s", args[0])
			}
			return attach(cmd, dir)
		}}
}

// attach views the run in dir on cmd's output.
func attach(cmd *cobra.Command, dir string) error {
	out := cmd.OutOrStdout()
	if !terminal(out) {
		return tui.Follow(dir, out)
	}
	saved, err := stty("-g")
	if err != nil {
		return err
	}
	defer func() { _, _ = stty(strings.TrimSpace(saved)) }()
	// A zero-byte terminal read looks like EOF to Go and ends the key reader.
	// Set the read minimum too: canonical mode can leave min/time at any value.
	if _, err := stty("-icanon", "-echo", "min", "1", "time", "0"); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()
	return tui.Watch(ctx, dir, out, os.Stdin, func() (int, int) {
		cols, rows := 80, 24
		if size, err := stty("size"); err == nil {
			_, _ = fmt.Sscan(size, &rows, &cols)
		}
		return cols, rows
	})
}

// terminal says whether w is a terminal.
// ponytail: any character device counts, /dev/null too; a real isatty check when that matters.
func terminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// stty runs stty with args on dulidu's terminal and returns what it prints.
func stty(args ...string) (string, error) {
	c := exec.Command("stty", args...)
	c.Stdin = os.Stdin
	out, err := c.Output()
	return string(out), err
}
