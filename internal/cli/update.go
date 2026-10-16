package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/spf13/cobra"

	"github.com/thellmwhisperer/dulidu/internal/buildinfo"
)

func updateCmd() *cobra.Command {
	return &cobra.Command{Use: "update", Short: "Replace this dulidu with the latest published release", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			exe, err := os.Executable()
			if err == nil {
				exe, err = filepath.EvalSymlinks(exe)
			}
			if err != nil {
				return err
			}
			return update(cmd.OutOrStdout(), buildinfo.CurrentVersion(), exe)
		}}
}

var releaseTag = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

// update replaces the dulidu at target, which is current, with the latest release; not over a build that is no
// release.
func update(w io.Writer, current, target string) error {
	latest, err := release("latest")
	if err != nil {
		return err
	}
	if latest.Tag == current {
		_, err = fmt.Fprintf(w, "dulidu %s is the latest release\n", current)
		return err
	}
	if !releaseTag.MatchString(current) {
		return fmt.Errorf("this dulidu is %s, a build and not a release, and the latest release is %s: install it "+
			"with install.sh", current, latest.Tag)
	}
	bin, err := latest.binary()
	if err == nil {
		err = swap(target, bin, latest.Tag)
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "dulidu %s -> %s\n", current, latest.Tag)
	return err
}
