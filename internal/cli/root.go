// Package cli is dulidu's command line: one cobra command per file.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/thellmwhisperer/dulidu/internal/buildinfo"
)

// Execute runs dulidu with os.Args and exits with its exit code.
func Execute() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// usageError is a bad flag: like an unknown subcommand, it prints the usage and exits 2.
type usageError struct{ error }

// run is dulidu with args: 0 when it went well, 2 for a usage error, 1 for any other.
func run(args []string, stdout, stderr io.Writer) int {
	c := root()
	c.SetArgs(args)
	c.SetOut(stdout)
	c.SetErr(stderr)
	cmd, err := c.ExecuteC()
	if err == nil {
		return 0
	}
	_, _ = fmt.Fprintln(stderr, "dulidu:", err)
	if errors.As(err, &usageError{}) || strings.HasPrefix(err.Error(), "unknown command") {
		_, _ = fmt.Fprint(stderr, cmd.UsageString())
		return 2
	}
	return 1
}

// root is dulidu with every command; it prints errors to no one, run does.
func root() *cobra.Command {
	c := &cobra.Command{Use: "dulidu", Short: "dulidu works in the repository it runs in",
		Version: buildinfo.String(), SilenceErrors: true, SilenceUsage: true}
	c.SetVersionTemplate("dulidu {{.Version}}\n")
	c.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return usageError{err} })
	c.AddCommand(initCmd(), doctorCmd(), updateCmd(), releaseCmd(), runsCmd(), runCmd())
	return c
}

// toplevel is the root of the git repository dulidu runs in.
func toplevel() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("not in a git repository: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
