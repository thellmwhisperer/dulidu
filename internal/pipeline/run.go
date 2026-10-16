package pipeline

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Ask stops r, in dir, on its triage's missing questions: they are questions.json and r is waiting_answers.
func Ask(dir string, r *Run) error {
	r.Questions, r.Report.Status = r.Triage.MissingQuestions, "waiting_answers"
	return Write(dir, *r)
}

// Resume is the run in dir, waiting_answers, running again with the file answers under "## Answers" at the end of its
// contexto.md, in memory for the next step to write. A run in any other status is "run <label> is <status>, nothing to
// resume".
func Resume(dir, answers string) (Run, error) {
	r, err := Read(dir)
	if err != nil {
		return r, err
	}
	if r.Report.Status != "waiting_answers" {
		return r, fmt.Errorf("run %s is %s, nothing to resume", filepath.Base(dir), r.Report.Status)
	}
	if answers == "" {
		return r, errors.New("no answers file to resume with")
	}
	b, err := os.ReadFile(answers)
	r.Context, r.Report.Status = strings.TrimRight(r.Context, "\n")+"\n\n## Answers\n\n"+string(b), "running"
	return r, err
}
