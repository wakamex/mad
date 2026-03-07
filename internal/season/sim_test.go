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
	if report.ActionSurface.PhraseVariantCount != 512 {
		t.Fatalf("unexpected phrase variant count: got %d want 512", report.ActionSurface.PhraseVariantCount)
	}
	if report.ActionSurface.PerTickCounts["S1-T0004"] != 513 {
		t.Fatalf("unexpected random action count on phrase tick: got %d want 513", report.ActionSurface.PerTickCounts["S1-T0004"])
	}
	if report.ActionSurface.Distribution["5"] != 2 || report.ActionSurface.Distribution["7"] != 1 || report.ActionSurface.Distribution["513"] != 1 {
		t.Fatalf("unexpected action-surface distribution: %#v", report.ActionSurface.Distribution)
	}
	if len(report.Baselines["greedy_best"].ScoreTrace) != len(loaded.Ticks) {
		t.Fatalf("unexpected best baseline trace length")
	}
	if len(report.Baselines["always_hold"].ScoreTrace) != len(loaded.Ticks) {
		t.Fatalf("unexpected hold baseline trace length")
	}
	if report.Baselines["greedy_best"].Ledger.Score <= report.Baselines["always_hold"].Ledger.Score {
		t.Fatalf("expected greedy_best baseline to outperform always_hold")
	}
	if len(report.Notes) == 0 {
		t.Fatalf("expected simulation notes")
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
	if report.RandomAudit.P99Score > report.Baselines["greedy_best"].Ledger.Score {
		t.Fatalf("expected random audit p99 not to exceed greedy baseline")
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

func TestEvaluateSimulatedActionSkipsIneligibleBestRule(t *testing.T) {
	plan := ScoringPlan{
		Rules: []Rule{
			{
				Match:          ActionMatch{Command: "commit", Target: "vault", Option: "open"},
				Requirements:   RuleRequirements{RequiresAllTags: []string{"vault.key"}},
				Delta:          ScoreDelta{Yield: 100},
				Label:          "gated best",
				Classification: "best",
			},
			{
				Match:          ActionMatch{Command: "commit", Target: "vault", Option: "open"},
				Delta:          ScoreDelta{Debt: 10},
				Label:          "fallback bad",
				Classification: "bad",
			},
			{
				Match:          ActionMatch{Command: "hold"},
				Delta:          ScoreDelta{MissPenalties: 1},
				Label:          "hold",
				Classification: "miss",
			},
		},
	}

	rule, isBest := evaluateSimulatedAction(plan, SimulatedAction{Command: "commit", Target: "vault", Option: "open"}, newSimulatedPlayerState())
	if isBest {
		t.Fatalf("expected ineligible best rule to be skipped")
	}
	if rule.Label != "fallback bad" {
		t.Fatalf("unexpected fallback rule: got %q", rule.Label)
	}
}

func TestGreedyBaselineRespectsRequirementsAndEffects(t *testing.T) {
	file := File{
		SchemaVersion:   "v1alpha1",
		SeasonID:        "sim-reqs",
		Title:           "sim reqs",
		ScoreEpochTicks: 2,
		RevealLagTicks:  1,
		ShardCount:      1,
		Ticks: []TickDefinition{
			{
				TickID:     "S1-T0001",
				ClockClass: "standard",
				DurationMS: 1000,
				Scoring: ScoringPlan{
					Rules: []Rule{
						{
							Match:          ActionMatch{Command: "commit", Target: "seed", Option: "prepare"},
							Effects:        StateEffects{AddTags: []string{"vault.key"}},
							Delta:          ScoreDelta{Insight: 5},
							Label:          "prepare",
							Classification: "best",
						},
						{
							Match:          ActionMatch{Command: "hold"},
							Delta:          ScoreDelta{MissPenalties: 1},
							Label:          "hold",
							Classification: "miss",
						},
					},
				},
				Opportunities: []Opportunity{{OpportunityID: "seed", AllowedCommands: []string{"commit", "hold"}, AllowedOptions: []string{"prepare"}}},
			},
			{
				TickID:     "S1-T0002",
				ClockClass: "standard",
				DurationMS: 1000,
				Scoring: ScoringPlan{
					Rules: []Rule{
						{
							Match:          ActionMatch{Command: "commit", Target: "vault", Option: "open"},
							Requirements:   RuleRequirements{RequiresAllTags: []string{"vault.key"}},
							Delta:          ScoreDelta{Yield: 100},
							Label:          "open vault",
							Classification: "best",
						},
						{
							Match:          ActionMatch{Command: "hold"},
							Delta:          ScoreDelta{MissPenalties: 2},
							Label:          "hold",
							Classification: "miss",
						},
					},
				},
				Opportunities: []Opportunity{{OpportunityID: "vault", AllowedCommands: []string{"commit", "hold"}, AllowedOptions: []string{"open"}}},
			},
		},
	}

	report, err := Simulate(file)
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}

	if report.Baselines["greedy_best"].Ledger.Score != 105 {
		t.Fatalf("unexpected greedy score: got %d want 105", report.Baselines["greedy_best"].Ledger.Score)
	}
	if report.Baselines["always_hold"].Ledger.Score != -3 {
		t.Fatalf("unexpected hold score: got %d want -3", report.Baselines["always_hold"].Ledger.Score)
	}
	if report.Ticks[1].ResolutionPreview == nil || report.Ticks[1].ResolutionPreview.BestKnownAction.Option != "open" {
		t.Fatalf("expected requirement-aware resolution preview on second tick")
	}
}
