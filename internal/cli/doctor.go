package cli

import (
	"cmp"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/thellmwhisperer/dulidu/internal/config"
	agent "github.com/thellmwhisperer/dulidu/internal/run" // run is the cli's own run
)

// helloLimit bounds dulidu doctor --agent's hello when dulidu.yaml sets no agent_timeout.
var helloLimit = 2 * time.Minute

// doctorCmd is dulidu doctor: the go toolchain, git and the config layers found, one line each, then, with
// --agent, the agent's answer to a hello. Every check runs; any failure fails the command.
func doctorCmd() *cobra.Command {
	var hello bool
	c := &cobra.Command{Use: "doctor", Short: "Check what dulidu needs", Args: cobra.NoArgs,
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
			var c config.Config
			var found []string
			home, cfgErr := os.UserHomeDir()
			if cfgErr == nil {
				c, found, cfgErr = config.Load(cmp.Or(root, "."), home)
			}
			check("config", cmp.Or(strings.Join(found, " "), "no layer, the defaults"), cfgErr)
			if hello {
				harness := cmp.Or(c.Harness, "claude")
				limit, err := time.ParseDuration(cmp.Or(c.AgentTimeout, helloLimit.String()))
				var took time.Duration
				if err == nil {
					err = cfgErr
				}
				if err == nil {
					took, err = agent.Hello(agent.Agent{Harness: harness, Model: c.Model, Effort: c.Effort, Timeout: limit})
				}
				line := "ok agent: " + harness + " " + c.Model + " answered in " + agent.Human(took)
				if err != nil {
					failed++
					line = "FAIL agent: " + harness + ": " + err.Error()
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), strings.Join(strings.Fields(line), " "))
			}
			if failed > 0 {
				return fmt.Errorf("doctor: %d of the checks failed", failed)
			}
			return nil
		}}
	c.Flags().BoolVar(&hello, "agent", false, "say hello to the agent the way a step does")
	return c
}
