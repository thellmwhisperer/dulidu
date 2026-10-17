package tui

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
)

// TestFrameHasRoundedBordersBoxTitlesIconsAndDurations is the screen the plain view does not draw.
func TestFrameHasRoundedBordersBoxTitlesIconsAndDurations(t *testing.T) {
	at := time.Date(2026, 10, 17, 1, 0, 0, 0, time.UTC)
	r := pipeline.Report{Status: "running", Steps: []pipeline.Step{
		{Name: "input", Status: "done", Started: at, Finished: at.Add(5 * time.Second)},
		{Name: "triage", Status: "running", Started: at.Add(5 * time.Second)},
	}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = Watch(ctx, t.TempDir(), io.Discard, strings.NewReader(""), func() (int, int) { return 120, 40 })
	frame := Frame("20261017-010000-1", r, nil, []string{"reading the brief"}, 120, 40, at.Add(time.Minute))
	if !strings.Contains(frame, "╭") || !strings.Contains(frame, "╮") || !strings.Contains(frame, "╰") || !strings.Contains(frame, "╯") ||
		!strings.Contains(frame, "Pipeline") || !strings.Contains(frame, "Agent") ||
		!strings.Contains(frame, "✓") || !strings.Contains(frame, "○") || !strings.ContainsAny(frame, "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏") ||
		!strings.Contains(frame, "0 min 05 s") || !strings.Contains(frame, "0 min 55 s") {
		t.Fatalf("output has no rounded borders, box titles, icons or durations:\n%s", frame)
	}
}
