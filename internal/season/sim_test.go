package season

import (
	"path/filepath"
	"testing"
)

func TestSimulateDevSeason(t *testing.T) {
	loaded, err := LoadFile(filepath.Join("..", "..", "seasons", "dev", "season.json"))
	if err != nil {
		t.Fatalf("load dev season: %v", err)
	}

	report, err := SimulateWithOptions(loaded, SimulationOptions{
		RandomRuns: 2000,
		RandomSeed: 7,
	})
	if err != nil {
		t.Fatalf("simulate season: %v", err)
	}

	if report.TickCount != len(loaded.Ticks) {
		t.Fatalf("unexpected tick count: got %d want %d", report.TickCount, len(loaded.Ticks))
	}
	if len(report.Reveals) != len(loaded.Ticks)-loaded.RevealLagTicks+1 {
		t.Fatalf("unexpected reveal count: got %d", len(report.Reveals))
	}
	if report.TotalDuration <= 0 {
		t.Fatalf("expected positive total duration")
	}
	if len(report.Baselines["perfect_best"].ScoreTrace) != len(loaded.Ticks) {
		t.Fatalf("unexpected best baseline trace length")
	}
	if len(report.Baselines["always_hold"].ScoreTrace) != len(loaded.Ticks) {
		t.Fatalf("unexpected hold baseline trace length")
	}
	if report.Baselines["perfect_best"].Ledger.Score <= report.Baselines["always_hold"].Ledger.Score {
		t.Fatalf("expected perfect_best baseline to outperform always_hold")
	}
	if report.RandomAudit == nil {
		t.Fatalf("expected random audit")
	}
	if report.RandomAudit.Runs != 2000 {
		t.Fatalf("unexpected random runs: got %d want %d", report.RandomAudit.Runs, 2000)
	}
	if report.RandomAudit.MeanScore >= 0 {
		t.Fatalf("expected random audit mean score to be negative, got %.2f", report.RandomAudit.MeanScore)
	}
	if report.RandomAudit.P99Score > report.Baselines["perfect_best"].Ledger.Score {
		t.Fatalf("expected random audit p99 not to exceed perfect baseline")
	}

	first := report.Ticks[0]
	if first.StartsAtMS != 0 {
		t.Fatalf("unexpected first tick start: got %d", first.StartsAtMS)
	}
	if first.RevealPublishesAtTick == "" {
		t.Fatalf("expected reveal publish tick on first tick preview")
	}

	second := report.Ticks[1]
	if second.Annotations.MemoryDistanceMin != 1 {
		t.Fatalf("unexpected memory distance on second tick: got %d", second.Annotations.MemoryDistanceMin)
	}

	reveal := report.Reveals[0]
	if reveal.TickID != loaded.Ticks[0].TickID {
		t.Fatalf("unexpected first reveal tick: got %s want %s", reveal.TickID, loaded.Ticks[0].TickID)
	}
	if reveal.PublishedAfterTickID != loaded.Ticks[loaded.RevealLagTicks-1].TickID {
		t.Fatalf("unexpected reveal publish tick: got %s want %s", reveal.PublishedAfterTickID, loaded.Ticks[loaded.RevealLagTicks-1].TickID)
	}
	if reveal.ResolutionPreview == nil || reveal.ResolutionPreview.BestKnownAction.Command == "" {
		t.Fatalf("expected reveal resolution preview")
	}
}
