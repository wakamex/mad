package main

import (
	"strings"
	"testing"
	"time"

	"github.com/mihai/mad/internal/harness"
	"github.com/mihai/mad/internal/season"
)

func TestStepDurationStats(t *testing.T) {
	t.Parallel()

	steps := []harness.StepTrace{
		{DurationMS: 10},
		{DurationMS: 20},
		{DurationMS: 30},
		{DurationMS: 40},
	}
	avg, p50, p95 := stepDurationStats(steps)
	if avg != 25*time.Millisecond {
		t.Fatalf("avg = %s, want 25ms", avg)
	}
	if p50 != 20*time.Millisecond {
		t.Fatalf("p50 = %s, want 20ms", p50)
	}
	if p95 != 40*time.Millisecond {
		t.Fatalf("p95 = %s, want 40ms", p95)
	}
}

func TestFormatProgressLine(t *testing.T) {
	t.Parallel()

	result := harness.RunResult{
		Runner:      harness.RunnerSpec{Provider: "codex", Model: "gpt-5.2-codex", Effort: "high"},
		StartTick:   0,
		EndTick:     10,
		StepCount:   4,
		StartedAt:   time.Unix(0, 0),
		CompletedAt: time.Unix(0, 0).Add(8 * time.Second),
		FinalScore:  harness.RunResult{}.FinalScore,
		ScoreTrace: []harness.ScorePoint{
			{TickID: "S1-T0004"},
		},
	}
	result.FinalScore.Score = 123

	line := formatProgressLine(result)
	for _, fragment := range []string{
		"4/10",
		"40.0%",
		"score=123",
		"d=+0",
		"avg=2s",
		"eta=12s",
		"last=S1-T0004",
	} {
		if !strings.Contains(line, fragment) {
			t.Fatalf("progress line %q missing %q", line, fragment)
		}
	}
}

func TestSummarizeRunGroups(t *testing.T) {
	t.Parallel()

	base := time.Unix(0, 0)
	runs := []harness.RunResult{
		{
			Runner:      harness.RunnerSpec{Provider: "codex", Model: "gpt-5.1-codex-mini", Effort: "medium"},
			FinalScore:  season.SimulatedLedger{Score: 10},
			StepCount:   2,
			StartedAt:   base,
			CompletedAt: base.Add(10 * time.Second),
		},
		{
			Runner:      harness.RunnerSpec{Provider: "codex", Model: "gpt-5.1-codex-mini", Effort: "medium"},
			FinalScore:  season.SimulatedLedger{Score: 30},
			StepCount:   2,
			StartedAt:   base,
			CompletedAt: base.Add(20 * time.Second),
		},
		{
			Runner:      harness.RunnerSpec{Provider: "claude", Model: "haiku", Effort: "low"},
			FinalScore:  season.SimulatedLedger{Score: -5},
			StepCount:   1,
			StartedAt:   base,
			CompletedAt: base.Add(5 * time.Second),
		},
	}

	groups := summarizeRunGroups(runs)
	if len(groups) != 2 {
		t.Fatalf("len(groups) = %d, want 2", len(groups))
	}
	if groups[0].RunCount != 2 {
		t.Fatalf("groups[0].RunCount = %d, want 2", groups[0].RunCount)
	}
	if groups[0].MeanScore != 20 {
		t.Fatalf("groups[0].MeanScore = %v, want 20", groups[0].MeanScore)
	}
	if groups[0].MedianScore != 10 {
		t.Fatalf("groups[0].MedianScore = %v, want 10", groups[0].MedianScore)
	}
	if groups[0].P90Score != 30 {
		t.Fatalf("groups[0].P90Score = %v, want 30", groups[0].P90Score)
	}
}
