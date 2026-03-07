package main

import (
	"testing"
	"time"

	"github.com/mihai/mad/internal/harness"
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
