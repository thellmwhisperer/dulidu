package tui

import (
	"context"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
)

// TestFrameHasRoundedBordersBoxTitlesIconsAndDurations is the screen the plain view does not draw.
func TestFrameHasRoundedBordersBoxTitlesIconsAndDurations(t *testing.T) {
	at := time.Now().Add(-time.Minute)
	r := pipeline.Report{Status: "running", Steps: []pipeline.Step{
		{Name: "input", Status: "done", Started: at, Finished: at.Add(5 * time.Second)},
		{Name: "triage", Status: "running", Harness: "claude", Started: at.Add(5 * time.Second)},
	}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = Watch(ctx, t.TempDir(), io.Discard, strings.NewReader(""), func() (int, int) { return 120, 40 })
	_, frame := screen(t, runAt(t, r, nil, says("reading the brief")), 120, 40)
	if !strings.Contains(frame, "╭") || !strings.Contains(frame, "╮") || !strings.Contains(frame, "╰") || !strings.Contains(frame, "╯") ||
		!strings.Contains(frame, "Pipeline") || !strings.Contains(frame, "Agent") ||
		!strings.Contains(frame, "✓") || !strings.Contains(frame, "○") || !strings.ContainsAny(frame, "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏") ||
		!strings.Contains(frame, "5.0 s") || !regexp.MustCompile(`55\.\d s`).MatchString(frame) {
		t.Fatalf("output has no rounded borders, box titles, icons or durations:\n%s", frame)
	}
}
