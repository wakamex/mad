package season

import "testing"

func TestAuditIRDetectsFlatGreedyBeat(t *testing.T) {
	ir := IRFile{
		SeasonID: "audit-dev",
		Elements: []StoryElement{
			{
				ElementID: "seed",
				Family:    "seed_clue_chain",
				Beats: []StoryBeat{
					testStoryBeat("seed.1", "seed.op.1"),
				},
			},
		},
	}

	report := AuditIR(ir)
	if len(report.FlatGreedyBeats) != 1 || report.FlatGreedyBeats[0] != "seed.1" {
		t.Fatalf("unexpected flat greedy beats: %#v", report.FlatGreedyBeats)
	}
	if len(report.Warnings) == 0 {
		t.Fatalf("expected warnings")
	}
}

func TestAuditIRCountsCrossElementDependencies(t *testing.T) {
	ir := IRFile{
		SeasonID: "audit-cross",
		Elements: []StoryElement{
			{
				ElementID: "seed",
				Family:    "seed_clue_chain",
				Beats: []StoryBeat{
					testStoryBeatWithTags("seed.1", "seed.op.1", nil, []string{"shared.tag"}),
				},
			},
			{
				ElementID: "gate",
				Family:    "payoff_gate",
				Beats: []StoryBeat{
					testStoryBeatWithTags("gate.1", "gate.op.1", []string{"shared.tag"}, nil, "seed.1"),
				},
			},
		},
	}

	report := AuditIR(ir)
	if report.CrossElementDependencyBeats != 1 {
		t.Fatalf("unexpected cross-element dependency count: got %d want 1", report.CrossElementDependencyBeats)
	}
	if report.TagConsumingBeats != 1 {
		t.Fatalf("unexpected tag-consuming beat count: got %d want 1", report.TagConsumingBeats)
	}
}

func TestAuditIRCountsTransitiveCrossElementDependencies(t *testing.T) {
	ir := IRFile{
		SeasonID: "audit-transitive",
		Elements: []StoryElement{
			{
				ElementID: "seed",
				Family:    "seed_clue_chain",
				Beats: []StoryBeat{
					testStoryBeat("seed.1", "seed.op.1"),
				},
			},
			{
				ElementID: "gate",
				Family:    "payoff_gate",
				Beats: []StoryBeat{
					testStoryBeat("gate.0", "gate.op.0", "seed.1"),
					testStoryBeat("gate.1", "gate.op.1", "gate.0"),
				},
			},
		},
	}

	report := AuditIR(ir)
	if report.CrossElementDependencyBeats != 2 {
		t.Fatalf("unexpected cross-element dependency count: got %d want 2", report.CrossElementDependencyBeats)
	}
}
