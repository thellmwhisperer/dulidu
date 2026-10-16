package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

// template is dulidu.yaml with every key config knows, empty for the project to fill.
const template = `# dulidu's configuration for this repository, over ~/.config/dulidu/config.yaml.
# Every key dulidu knows is listed here with an empty value; any other key is an error.
# None yet: the agent layer and the prompts bring theirs.
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
			return ignore(filepath.Join(root, ".gitignore"), ".dulidu/")
		}}
}

// ignore appends line to the .gitignore at path unless it has it.
func ignore(path, line string) error {
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	text := string(b)
	if slices.Contains(strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n"), line) {
		return nil
	}
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return os.WriteFile(path, []byte(text+line+"\n"), 0o644)
}
