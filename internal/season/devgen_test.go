package season

import "testing"

func TestBuildGeneratedDevSeasonIR(t *testing.T) {
	t.Parallel()

	ir, err := BuildGeneratedDevSeasonIR(1000)
	if err != nil {
		t.Fatalf("BuildGeneratedDevSeasonIR: %v", err)
	}
	if len(ir.Elements) != 250 {
		t.Fatalf("expected 250 elements, got %d", len(ir.Elements))
	}

	totalBeats := 0
	lengthCounts := make(map[int]int)
	for _, element := range ir.Elements {
		totalBeats += len(element.Beats)
		lengthCounts[len(element.Beats)]++
	}
	if totalBeats != 1000 {
		t.Fatalf("expected 1000 beats, got %d", totalBeats)
	}
	for beats := 2; beats <= 5; beats++ {
		if lengthCounts[beats] == 0 {
			t.Fatalf("expected at least one %d-beat element, got distribution %#v", beats, lengthCounts)
		}
	}

	compiled, err := CompileIR(ir)
	if err != nil {
		t.Fatalf("CompileIR: %v", err)
	}
	if len(compiled.Ticks) != 1000 {
		t.Fatalf("expected 1000 compiled ticks, got %d", len(compiled.Ticks))
	}
}

func TestBuildGeneratedDevSeasonIRRejectsUnsupportedTickCounts(t *testing.T) {
	t.Parallel()

	if _, err := BuildGeneratedDevSeasonIR(999); err == nil {
		t.Fatal("expected error for unsupported tick count")
	}
}

func TestHazardAuraThresholdScalesWithClusterProgress(t *testing.T) {
	t.Parallel()

	theme := devTheme{AuraTier: 8}

	early := hazardAuraThreshold(theme, 0, 2)
	mid := hazardAuraThreshold(theme, 10, 2)
	late := hazardAuraThreshold(theme, 49, 2)

	if !(early < mid && mid < late) {
		t.Fatalf("expected aura threshold to grow with cluster progress: early=%d mid=%d late=%d", early, mid, late)
	}
	if early != 8 {
		t.Fatalf("unexpected early threshold: got %d want 8", early)
	}
}
