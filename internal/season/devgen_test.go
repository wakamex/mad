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

func TestHazardFactionProfilesStayStableAcrossClusters(t *testing.T) {
	t.Parallel()

	first := buildDevTheme(0)
	repeat := buildDevTheme(len(devFactions))

	if first.Faction.ID != repeat.Faction.ID {
		t.Fatalf("expected same faction after one full rotation: first=%s repeat=%s", first.Faction.ID, repeat.Faction.ID)
	}
	if first.Faction.StabilizeRepThreshold != repeat.Faction.StabilizeRepThreshold {
		t.Fatalf("expected stable stabilize threshold across cluster reuse: first=%d repeat=%d", first.Faction.StabilizeRepThreshold, repeat.Faction.StabilizeRepThreshold)
	}
	if first.Faction.StabilizeRepSpend != repeat.Faction.StabilizeRepSpend {
		t.Fatalf("expected stable stabilize spend across cluster reuse: first=%d repeat=%d", first.Faction.StabilizeRepSpend, repeat.Faction.StabilizeRepSpend)
	}
	if first.Faction.ExploitAuraThreshold != repeat.Faction.ExploitAuraThreshold {
		t.Fatalf("expected stable exploit threshold across cluster reuse: first=%d repeat=%d", first.Faction.ExploitAuraThreshold, repeat.Faction.ExploitAuraThreshold)
	}
	if first.Faction.ExploitAuraSpend != repeat.Faction.ExploitAuraSpend {
		t.Fatalf("expected stable exploit spend across cluster reuse: first=%d repeat=%d", first.Faction.ExploitAuraSpend, repeat.Faction.ExploitAuraSpend)
	}
}
