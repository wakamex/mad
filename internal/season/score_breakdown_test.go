package season

import "testing"

func TestScoreBreakdownAccumulatorAttributesFamilyElementAndSourceTypes(t *testing.T) {
	acc := NewScoreBreakdownAccumulator()
	tick := TickDefinition{
		Sources: []Source{
			{SourceType: "official_bulletin"},
			{SourceType: "market_gossip"},
			{SourceType: "market_gossip"},
		},
		Annotations: Annotations{
			Family:    "payoff_gate",
			ElementID: "cluster_001_payoff",
		},
	}
	rule := Rule{
		Delta:          ScoreDelta{Yield: 100, Insight: 10, Debt: 20, MissPenalties: 5},
		Classification: "best",
	}

	acc.Add(tick, rule)
	breakdown := acc.Materialize()

	if len(breakdown.ByFamily) != 1 || breakdown.ByFamily[0].Key != "payoff_gate" {
		t.Fatalf("unexpected family breakdown: %#v", breakdown.ByFamily)
	}
	if breakdown.ByFamily[0].Score != 85 {
		t.Fatalf("unexpected family score: got %f want 85", breakdown.ByFamily[0].Score)
	}
	if len(breakdown.ByElement) != 1 || breakdown.ByElement[0].Key != "cluster_001_payoff" {
		t.Fatalf("unexpected element breakdown: %#v", breakdown.ByElement)
	}
	if len(breakdown.BySourceType) != 2 {
		t.Fatalf("unexpected source-type breakdown: %#v", breakdown.BySourceType)
	}
	for _, entry := range breakdown.BySourceType {
		if entry.Score != 42.5 {
			t.Fatalf("expected split source score of 42.5, got %f for %s", entry.Score, entry.Key)
		}
		if entry.BestCount != 1 {
			t.Fatalf("expected best_count=1 for %s, got %d", entry.Key, entry.BestCount)
		}
	}
}

func TestDeriveScoreDecomposition(t *testing.T) {
	greedy := ScoreBreakdown{
		ByFamily: []ScoreBreakdownEntry{
			{Key: "hazard", Score: 100},
			{Key: "clue", Score: 20},
		},
		BySourceType: []ScoreBreakdownEntry{
			{Key: "critical_broadcast", Score: 100},
			{Key: "archive_fragment", Score: 20},
		},
	}
	visible := ScoreBreakdown{
		ByFamily: []ScoreBreakdownEntry{
			{Key: "hazard", Score: 30},
			{Key: "clue", Score: 20},
		},
		BySourceType: []ScoreBreakdownEntry{
			{Key: "critical_broadcast", Score: 30},
			{Key: "archive_fragment", Score: 20},
		},
	}

	decomposition := DeriveScoreDecomposition(greedy, visible, 120, 50)
	if decomposition.ExplicitVisibleTotal != 50 {
		t.Fatalf("unexpected explicit total: got %f want 50", decomposition.ExplicitVisibleTotal)
	}
	if decomposition.HiddenOrNonlocalTotal != 70 {
		t.Fatalf("unexpected hidden/nonlocal total: got %f want 70", decomposition.HiddenOrNonlocalTotal)
	}
	if decomposition.ExplicitVisibleShare != 50.0/120.0 {
		t.Fatalf("unexpected explicit share: got %f", decomposition.ExplicitVisibleShare)
	}
	if len(decomposition.ByFamily) != 2 {
		t.Fatalf("unexpected family decomposition: %#v", decomposition.ByFamily)
	}
	if decomposition.ByFamily[0].Key != "hazard" || decomposition.ByFamily[0].HiddenOrNonlocal != 70 {
		t.Fatalf("unexpected top family decomposition: %#v", decomposition.ByFamily[0])
	}
	if decomposition.BySourceType[0].Key != "critical_broadcast" || decomposition.BySourceType[0].ExplicitVisible != 30 {
		t.Fatalf("unexpected source decomposition: %#v", decomposition.BySourceType[0])
	}
}
