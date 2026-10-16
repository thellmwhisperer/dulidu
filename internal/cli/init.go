package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

// template is dulidu.yaml with every key config knows: the project's empty for it to fill, each step's agent
// a default.
const template = `# dulidu's configuration for this repository, over ~/.config/dulidu/config.yaml.
# Every key dulidu knows is listed here; a run needs each one, empty means none, any other key is an error.

# The agent CLI doctor --agent greets: claude, codex, opencode, pi, cursor or hermes; "" is claude
harness: ""
# Its model and effort, as its CLI takes them
model: ""
effort: ""
# How long a run may take, as 90s or 20m; "" is 30m for runs, 2m for doctor --agent
agent_timeout: ""

# The product: the platform it runs on and the kind of project it is
platform: ""
project_type: ""
# At most this many epics, and stories in each; empty is no limit
max_epics:
max_stories_per_epic:
# How big one ticket is, in words
ticket_size: ""
# What stays out of the epics and stories, one item each
exclusions: []
# Seconds between the frames read from a video in the input; empty reads none
video_frame_seconds:

# Each step's agent: harness and model as above, effort as its CLI takes it
steps:
  # input reads what came with the brief: a cheap model, sight is enough
  input:
    harness: claude
    model: haiku
    effort: low
  # triage decides what the brief is about and what to research: a strong model
  triage:
    harness: claude
    model: opus
    effort: high
  # research finds what each dimension needs: a medium model
  research:
    harness: claude
    model: sonnet
    effort: medium
  # analysis takes the brief apart: a medium model
  analysis:
    harness: claude
    model: sonnet
    effort: medium
  # epics outlines the work: a medium model
  epics:
    harness: claude
    model: sonnet
    effort: medium
  # stories writes the tickets: a strong model
  stories:
    harness: claude
    model: opus
    effort: high
`

// initCmd is dulidu init: dulidu.yaml from template unless there is one, and .dulidu/ in .gitignore unless it is.
func initCmd() *cobra.Command {
	return &cobra.Command{Use: "init", Short: "Write dulidu.yaml to fill and ignore .dulidu/", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := toplevel()
			if err != nil {
				return err
			}
			f, err := os.OpenFile(filepath.Join(root, "dulidu.yaml"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
			if err == nil {
				_, err = f.WriteString(template)
				err = errors.Join(err, f.Close())
			} else if errors.Is(err, fs.ErrExist) {
				_, err = fmt.Fprintln(cmd.OutOrStdout(), "dulidu.yaml exists")
			}
			if err != nil {
				return err
			}
			return ignore(root)
		}}
}

func ignore(root string) error {
	path := filepath.Join(root, ".gitignore")
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	text := string(b)
	if slices.Contains(strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n"), ".dulidu/") {
		check := exec.Command("git", "-C", root, "check-ignore", "-q", "--no-index", "--", ".dulidu/")
		if err := check.Run(); err == nil {
			return nil
		} else if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != 1 {
			return err
		}
	}
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return os.WriteFile(path, []byte(text+".dulidu/\n"), 0o644)
}
