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
