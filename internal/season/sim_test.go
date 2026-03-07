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
	if len(report.ActionSurface.Distribution) == 0 {
		t.Fatalf("expected non-empty action surface distribution")
	}
	for _, tick := range loaded.Ticks {
		if report.ActionSurface.PerTickCounts[tick.TickID] <= 0 {
			t.Fatalf("expected positive random action count for tick %s", tick.TickID)
		}
	}
	if len(report.Baselines["greedy_best"].ScoreTrace) != len(loaded.Ticks) {
		t.Fatalf("unexpected best baseline trace length")
	}
	if len(report.Baselines["always_hold"].ScoreTrace) != len(loaded.Ticks) {
		t.Fatalf("unexpected hold baseline trace length")
	}
	if len(report.Baselines["visible_greedy"].ScoreTrace) != len(loaded.Ticks) {
		t.Fatalf("unexpected visible greedy baseline trace length")
	}
	if len(report.Baselines["greedy_best"].Breakdown.ByFamily) == 0 {
		t.Fatalf("expected greedy baseline family breakdown")
	}
	if len(report.Baselines["visible_greedy"].Breakdown.ByFamily) == 0 {
		t.Fatalf("expected visible greedy baseline family breakdown")
	}
	if len(report.Baselines["greedy_best"].Breakdown.BySourceType) == 0 {
		t.Fatalf("expected greedy baseline source breakdown")
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
	if report.RandomAudit.P99Run == nil {
		t.Fatalf("expected p99 random run attribution")
	}
	if report.RandomAudit.P99Run.Score != report.RandomAudit.P99Score {
		t.Fatalf("expected p99 run score to match p99 score: got %d want %d", report.RandomAudit.P99Run.Score, report.RandomAudit.P99Score)
	}
	if len(report.RandomAudit.P99Run.Breakdown.ByFamily) == 0 {
		t.Fatalf("expected p99 random run family breakdown")
	}
	if report.RandomAudit.MinScore > report.RandomAudit.MaxScore {
		t.Fatalf("expected random audit min <= max")
	}
	if report.RandomAudit.PositiveRate < 0 || report.RandomAudit.PositiveRate > 1 {
		t.Fatalf("expected positive rate in [0,1], got %f", report.RandomAudit.PositiveRate)
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

	if maxMemoryDistance(report.Ticks) <= 0 {
		t.Fatalf("expected at least one derived memory distance")
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

func TestGreedyBaselineRespectsAvailabilityLocks(t *testing.T) {
	file := File{
		SchemaVersion:   "v1alpha1",
		SeasonID:        "sim-locks",
		Title:           "sim locks",
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
							Match:          ActionMatch{Command: "commit", Target: "work", Option: "shift"},
							Effects:        StateEffects{AvailabilityDelta: "committed", LockTicks: 1},
							Delta:          ScoreDelta{Yield: 5},
							Label:          "take the shift",
							Classification: "best",
						},
						{
							Match:          ActionMatch{Command: "hold"},
							Delta:          ScoreDelta{},
							Label:          "hold",
							Classification: "miss",
						},
					},
				},
				Opportunities: []Opportunity{{OpportunityID: "work", AllowedCommands: []string{"commit", "hold"}, AllowedOptions: []string{"shift"}}},
			},
			{
				TickID:     "S1-T0002",
				ClockClass: "standard",
				DurationMS: 1000,
				Scoring: ScoringPlan{
					Rules: []Rule{
						{
							Match:          ActionMatch{Command: "commit", Target: "broker.2", Option: "take"},
							Requirements:   RuleRequirements{RequiresAvailability: []string{defaultAvailability}},
							Delta:          ScoreDelta{Yield: 100},
							Label:          "take broker job",
							Classification: "best",
						},
						{
							Match:          ActionMatch{Command: "hold"},
							Delta:          ScoreDelta{MissPenalties: 2},
							Label:          "too busy to act",
							Classification: "miss",
						},
					},
				},
				Opportunities: []Opportunity{{OpportunityID: "broker.2", AllowedCommands: []string{"commit", "hold"}, AllowedOptions: []string{"take"}}},
			},
			{
				TickID:     "S1-T0003",
				ClockClass: "standard",
				DurationMS: 1000,
				Scoring: ScoringPlan{
					Rules: []Rule{
						{
							Match:          ActionMatch{Command: "commit", Target: "broker.3", Option: "take"},
							Requirements:   RuleRequirements{RequiresAvailability: []string{defaultAvailability}},
							Delta:          ScoreDelta{Yield: 100},
							Label:          "take broker job",
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
				Opportunities: []Opportunity{{OpportunityID: "broker.3", AllowedCommands: []string{"commit", "hold"}, AllowedOptions: []string{"take"}}},
			},
		},
	}

	report, err := Simulate(file)
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}

	if report.Baselines["greedy_best"].Ledger.Score != 103 {
		t.Fatalf("unexpected greedy score: got %d want 103", report.Baselines["greedy_best"].Ledger.Score)
	}
	if report.Ticks[1].ResolutionPreview != nil {
		t.Fatalf("expected no eligible best action while locked on second tick")
	}
	if report.Ticks[2].ResolutionPreview == nil || report.Ticks[2].ResolutionPreview.BestKnownAction.Option != "take" {
		t.Fatalf("expected lock to expire before third tick")
	}
}

func TestGreedyBaselineRespectsCooldownReadiness(t *testing.T) {
	file := File{
		SchemaVersion:   "v1alpha1",
		SeasonID:        "sim-cooldowns",
		Title:           "sim cooldowns",
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
							Match:          ActionMatch{Command: "commit", Target: "work", Option: "patrol"},
							Effects:        StateEffects{SetCooldowns: map[string]int{"favors.choir": 1}},
							Delta:          ScoreDelta{Insight: 3},
							Label:          "use the favor",
							Classification: "best",
						},
						{
							Match:          ActionMatch{Command: "hold"},
							Delta:          ScoreDelta{},
							Label:          "hold",
							Classification: "miss",
						},
					},
				},
				Opportunities: []Opportunity{{OpportunityID: "work", AllowedCommands: []string{"commit", "hold"}, AllowedOptions: []string{"patrol"}}},
			},
			{
				TickID:     "S1-T0002",
				ClockClass: "standard",
				DurationMS: 1000,
				Scoring: ScoringPlan{
					Rules: []Rule{
						{
							Match:          ActionMatch{Command: "commit", Target: "favor.2", Option: "cash_in"},
							Requirements:   RuleRequirements{RequiresCooldownReady: []string{"favors.choir"}},
							Delta:          ScoreDelta{Yield: 50},
							Label:          "cash in favor",
							Classification: "best",
						},
						{
							Match:          ActionMatch{Command: "hold"},
							Delta:          ScoreDelta{MissPenalties: 2},
							Label:          "favor still cooling down",
							Classification: "miss",
						},
					},
				},
				Opportunities: []Opportunity{{OpportunityID: "favor.2", AllowedCommands: []string{"commit", "hold"}, AllowedOptions: []string{"cash_in"}}},
			},
			{
				TickID:     "S1-T0003",
				ClockClass: "standard",
				DurationMS: 1000,
				Scoring: ScoringPlan{
					Rules: []Rule{
						{
							Match:          ActionMatch{Command: "commit", Target: "favor.3", Option: "cash_in"},
							Requirements:   RuleRequirements{RequiresCooldownReady: []string{"favors.choir"}},
							Delta:          ScoreDelta{Yield: 50},
							Label:          "cash in favor",
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
				Opportunities: []Opportunity{{OpportunityID: "favor.3", AllowedCommands: []string{"commit", "hold"}, AllowedOptions: []string{"cash_in"}}},
			},
		},
	}

	report, err := Simulate(file)
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}

	if report.Baselines["greedy_best"].Ledger.Score != 51 {
		t.Fatalf("unexpected greedy score: got %d want 51", report.Baselines["greedy_best"].Ledger.Score)
	}
	if report.Ticks[1].ResolutionPreview != nil {
		t.Fatalf("expected no eligible best action while cooldown is active on second tick")
	}
	if report.Ticks[2].ResolutionPreview == nil || report.Ticks[2].ResolutionPreview.BestKnownAction.Option != "cash_in" {
		t.Fatalf("expected cooldown to expire before third tick")
	}
}

func TestVisibleGreedyPrefersInspectOverBlindMultiOptionCommit(t *testing.T) {
	file := File{
		SchemaVersion:   "v1alpha1",
		SeasonID:        "sim-visible-inspect",
		Title:           "sim visible inspect",
		ScoreEpochTicks: 1,
		RevealLagTicks:  1,
		ShardCount:      1,
		Ticks: []TickDefinition{
			{
				TickID:     "S1-T0001",
				ClockClass: "standard",
				DurationMS: 1000,
				Annotations: Annotations{
					Family: "payoff_gate",
				},
				Sources: []Source{
					{SourceID: "src.1", SourceType: "market_gossip", Text: "Signal quality is unclear."},
				},
				Opportunities: []Opportunity{
					{
						OpportunityID:   "vault",
						AllowedCommands: []string{"inspect", "commit", "hold"},
						AllowedOptions:  []string{"north", "south", "west"},
					},
				},
				Scoring: ScoringPlan{
					Rules: []Rule{
						{
							Match:          ActionMatch{Command: "inspect", Target: "vault"},
							Delta:          ScoreDelta{Insight: 2},
							Label:          "inspect the vault seam",
							Classification: "miss",
						},
						{
							Match:          ActionMatch{Command: "commit", Target: "vault", Option: "north"},
							Delta:          ScoreDelta{Yield: 40},
							Label:          "lucky blind guess",
							Classification: "best",
						},
						{
							Match:          ActionMatch{Command: "commit", Target: "vault", Option: "south"},
							Delta:          ScoreDelta{Debt: 10},
							Label:          "bad blind guess",
							Classification: "bad",
						},
						{
							Match:          ActionMatch{Command: "commit", Target: "vault", Option: "west"},
							Delta:          ScoreDelta{MissPenalties: 3},
							Label:          "wasted blind guess",
							Classification: "miss",
						},
						{
							Match:          ActionMatch{Command: "hold"},
							Delta:          ScoreDelta{},
							Label:          "hold",
							Classification: "miss",
						},
					},
				},
			},
		},
	}

	report, err := Simulate(file)
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}

	if report.Baselines["visible_greedy"].Ledger.Score != 2 {
		t.Fatalf("expected visible greedy to take inspect, got score %d", report.Baselines["visible_greedy"].Ledger.Score)
	}
	if report.Baselines["greedy_best"].Ledger.Score != 40 {
		t.Fatalf("expected greedy best to take hidden best branch, got %d", report.Baselines["greedy_best"].Ledger.Score)
	}
}

func TestVisibleGreedyUsesPublicRequirementsAndExplicitState(t *testing.T) {
	file := File{
		SchemaVersion:   "v1alpha1",
		SeasonID:        "sim-visible-public-state",
		Title:           "sim visible public state",
		ScoreEpochTicks: 2,
		RevealLagTicks:  1,
		ShardCount:      1,
		Ticks: []TickDefinition{
			{
				TickID:     "S1-T0001",
				ClockClass: "standard",
				DurationMS: 1000,
				Annotations: Annotations{
					Family: "standing_work_loop",
				},
				Sources: []Source{
					{SourceID: "src.1", SourceType: "faction_notice", Text: "Registry shift available."},
				},
				Opportunities: []Opportunity{
					{
						OpportunityID:   "registry",
						AllowedCommands: []string{"commit", "hold"},
						AllowedOptions:  []string{"shift"},
					},
				},
				Scoring: ScoringPlan{
					Rules: []Rule{
						{
							Match:   ActionMatch{Command: "commit", Target: "registry", Option: "shift"},
							Effects: StateEffects{ReputationDelta: map[string]int64{"relay_guild": 5}},
							Delta:   ScoreDelta{Yield: 3},
							Label:   "take registry shift",
							Classification: "best",
						},
						{
							Match:          ActionMatch{Command: "hold"},
							Delta:          ScoreDelta{},
							Label:          "hold",
							Classification: "miss",
						},
					},
				},
			},
			{
				TickID:     "S1-T0002",
				ClockClass: "standard",
				DurationMS: 1000,
				Annotations: Annotations{
					Family: "reputation_ladder",
				},
				Sources: []Source{
					{SourceID: "src.2", SourceType: "faction_notice", Text: "Premium audit lane open to trusted runners."},
				},
				Opportunities: []Opportunity{
					{
						OpportunityID:   "audit",
						AllowedCommands: []string{"inspect", "commit", "hold"},
						AllowedOptions:  []string{"premium"},
						PublicRequirements: []PublicRequirement{
							{Metric: "reputation", Scope: "relay_guild", Operator: ">=", Value: 5, Label: "relay_guild reputation 5+"},
						},
					},
				},
				Scoring: ScoringPlan{
					Rules: []Rule{
						{
							Match:        ActionMatch{Command: "commit", Target: "audit", Option: "premium"},
							Requirements: RuleRequirements{MinReputation: map[string]int64{"relay_guild": 5}},
							Delta:        ScoreDelta{Yield: 20},
							Label:        "take premium audit lane",
							Classification: "best",
						},
						{
							Match:          ActionMatch{Command: "inspect", Target: "audit"},
							Delta:          ScoreDelta{Insight: 1},
							Label:          "inspect audit bulletin",
							Classification: "miss",
						},
						{
							Match:          ActionMatch{Command: "hold"},
							Delta:          ScoreDelta{},
							Label:          "hold",
							Classification: "miss",
						},
					},
				},
			},
		},
	}

	report, err := Simulate(file)
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}

	if report.Baselines["visible_greedy"].Ledger.Score != 23 {
		t.Fatalf("expected visible greedy to use explicit reputation state, got %d", report.Baselines["visible_greedy"].Ledger.Score)
	}
}

func maxMemoryDistance(ticks []SimulatedTick) int {
	maxDistance := 0
	for _, tick := range ticks {
		if tick.Annotations.MemoryDistanceMin > maxDistance {
			maxDistance = tick.Annotations.MemoryDistanceMin
		}
	}
	return maxDistance
}
