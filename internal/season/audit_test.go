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

func TestAuditIRDoesNotFlagAvailabilityBeatAsFlatGreedy(t *testing.T) {
	ir := IRFile{
		SeasonID: "audit-availability",
		Elements: []StoryElement{
			{
				ElementID: "work",
				Family:    "standing_work_loop",
				Beats: []StoryBeat{
					{
						BeatID:        "work.1",
						ClockClass:    "standard",
						Sources:       []Source{{SourceID: "work.1.source", SourceType: "test", Text: "work"}},
						Opportunities: []Opportunity{{OpportunityID: "work.op.1", AllowedCommands: []string{"commit", "hold"}, AllowedOptions: []string{"shift"}}},
						Scoring: ScoringPlan{
							Rules: []Rule{
								{
									Match:          ActionMatch{Command: "commit", Target: "work.op.1", Option: "shift"},
									Effects:        StateEffects{AvailabilityDelta: "committed", LockTicks: 1},
									Delta:          ScoreDelta{Yield: 5},
									Label:          "shift",
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
				},
			},
		},
	}

	report := AuditIR(ir)
	if len(report.FlatGreedyBeats) != 0 {
		t.Fatalf("expected availability-changing beat not to be flat greedy: %#v", report.FlatGreedyBeats)
	}
}

func TestAuditIRFlagsWeakStandingWorkElement(t *testing.T) {
	ir := IRFile{
		SeasonID: "audit-standing-weak",
		Elements: []StoryElement{
			{
				ElementID: "standing",
				Family:    "standing_work_loop",
				Beats: []StoryBeat{
					{
						BeatID:        "standing.1",
						ClockClass:    "standard",
						ProducesTags:  []string{"standing.token"},
						Sources:       []Source{{SourceID: "standing.1.source", SourceType: "test", Text: "standing"}},
						Opportunities: []Opportunity{{OpportunityID: "standing.op.1", AllowedCommands: []string{"commit", "hold"}, AllowedOptions: []string{"work"}}},
						Scoring: ScoringPlan{
							Rules: []Rule{
								{
									Match:          ActionMatch{Command: "commit", Target: "standing.op.1", Option: "work"},
									Delta:          ScoreDelta{Yield: 5},
									Label:          "easy work",
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
				},
			},
			{
				ElementID: "payoff",
				Family:    "payoff_gate",
				Beats: []StoryBeat{
					testStoryBeatWithTags("payoff.1", "payoff.op.1", []string{"standing.token"}, nil),
				},
			},
		},
	}

	report := AuditIR(ir)
	if report.StandingWorkElements != 1 {
		t.Fatalf("unexpected standing work count: got %d want 1", report.StandingWorkElements)
	}
	if len(report.WeakStandingWorkElements) < 2 {
		t.Fatalf("expected standing work warnings, got %#v", report.WeakStandingWorkElements)
	}
}

func TestAuditIRAcceptsStandingWorkWithCostAndFanout(t *testing.T) {
	ir := IRFile{
		SeasonID: "audit-standing-strong",
		Elements: []StoryElement{
			{
				ElementID: "standing",
				Family:    "standing_work_loop",
				Beats: []StoryBeat{
					{
						BeatID:        "standing.1",
						ClockClass:    "standard",
						Sources:       []Source{{SourceID: "standing.1.source", SourceType: "test", Text: "standing"}},
						Opportunities: []Opportunity{{OpportunityID: "standing.op.1", AllowedCommands: []string{"commit", "hold"}, AllowedOptions: []string{"work"}}},
						Scoring: ScoringPlan{
							Rules: []Rule{
								{
									Match:          ActionMatch{Command: "commit", Target: "standing.op.1", Option: "work"},
									Effects:        StateEffects{AddTags: []string{"standing.token"}, LockTicks: 1, AvailabilityDelta: "committed"},
									Delta:          ScoreDelta{Debt: 1},
									Label:          "standing work",
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
				},
			},
			{
				ElementID: "payoff_a",
				Family:    "payoff_gate",
				Beats: []StoryBeat{
					testStoryBeatWithTags("payoff.a.1", "payoff.a.op.1", []string{"standing.token"}, nil),
				},
			},
			{
				ElementID: "payoff_b",
				Family:    "payoff_gate",
				Beats: []StoryBeat{
					testStoryBeatWithTags("payoff.b.1", "payoff.b.op.1", []string{"standing.token"}, nil),
				},
			},
		},
	}

	report := AuditIR(ir)
	if len(report.WeakStandingWorkElements) != 0 {
		t.Fatalf("expected standing work element to pass audit, got %#v", report.WeakStandingWorkElements)
	}
}
