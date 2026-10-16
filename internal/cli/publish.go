package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func releaseCmd() *cobra.Command {
	return &cobra.Command{Use: "release vX.Y.Z", Short: "Build and publish a tagged release",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 || !releaseTag.MatchString(args[0]) {
				return usageError{fmt.Errorf("release requires one tag vX.Y.Z")}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := toplevel()
			if err != nil {
				return err
			}
			out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
			if err != nil {
				return fmt.Errorf("current commit: %w", err)
			}
			repo, err := exec.Command("git", "-C", root, "remote", "get-url", "origin").Output()
			if err != nil {
				return fmt.Errorf("origin repository: %w", err)
			}
			tag := args[0]
			ref := "refs/tags/" + tag
			remote, err := exec.Command("git", "-C", root, "ls-remote", "--tags", "origin", ref, ref+"^{}").Output()
			if err != nil {
				return fmt.Errorf("remote tag lookup: %w", err)
			}
			var tagCommit string
			for _, line := range strings.Split(strings.TrimSpace(string(remote)), "\n") {
				if line == "" {
					continue
				}
				fields := strings.Fields(line)
				if len(fields) != 2 {
					return fmt.Errorf("invalid remote tag response")
				}
				switch fields[1] {
				case ref:
					if tagCommit == "" {
						tagCommit = fields[0]
					}
				case ref + "^{}":
					tagCommit = fields[0]
				default:
					return fmt.Errorf("invalid remote tag response")
				}
			}
			if tagCommit != "" && tagCommit != strings.TrimSpace(string(out)) {
				return fmt.Errorf("remote tag %s points to %s, not current commit", tag, tagCommit)
			}
			build := exec.Command("make", "-C", root, "dist", "TAG="+tag)
			if out, err := build.CombinedOutput(); err != nil {
				return fmt.Errorf("build release: %w: %s", err, strings.TrimSpace(string(out)))
			}
			files := []string{"release", "create", tag, "--target", strings.TrimSpace(string(out))}
			for _, platform := range []string{"darwin-amd64", "darwin-arm64", "linux-amd64", "linux-arm64"} {
				files = append(files, filepath.Join("dist", "dulidu-"+tag+"-"+platform+".tar.gz"))
			}
			files = append(files, filepath.Join("dist", "checksums.txt"))
			publish := exec.Command("gh", files...)
			publish.Dir = root
			// Publish where the tag was checked, not wherever an inherited GH_REPO points.
			publish.Env = append(os.Environ(), "GH_REPO="+strings.TrimSpace(string(repo)))
			if out, err := publish.CombinedOutput(); err != nil {
				return fmt.Errorf("publish release: %w: %s", err, strings.TrimSpace(string(out)))
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "published dulidu %s\n", tag)
			return err
		}}
}
