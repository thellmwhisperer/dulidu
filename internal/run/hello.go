package run

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"
)

// helloPrompt is all dulidu doctor --agent asks the agent.
const helloPrompt = "Reply with the single word OK and nothing else."

// Hello asks the agent for OK through Agent.Run and returns the elapsed time. It uses a temporary Git repository
// for agents such as Codex that require one, then removes it. Its error is the cause, then the agent's last 5 lines.
func Hello(a Agent) (time.Duration, error) {
	dir, err := os.MkdirTemp("", "dulidu-hello-")
	if err != nil {
		return 0, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	if err := exec.Command("git", "-C", dir, "init", "-q").Run(); err != nil {
		return 0, err
	}
	start := time.Now()
	_, err = a.Run(dir, helloPrompt, func(final string) error {
		if !strings.Contains(final, "OK") {
			return errors.New("no OK in its final message")
		}
		return nil
	})
	return time.Since(start), err
}
