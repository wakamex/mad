package season

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCompileIRDeterministicAndPreservesElementOrder(t *testing.T) {
	ir := IRFile{
		SeasonID:    "ir-dev",
		Title:       "IR Dev",
		CompileSeed: 17,
		Elements: []StoryElement{
			{
				ElementID: "glass",
				Family:    "seed_clue_chain",
				Beats: []StoryBeat{
					testStoryBeat("glass.1", "glass.op.1"),
					testStoryBeat("glass.2", "glass.op.2"),
				},
			},
			{
				ElementID: "vault",
				Family:    "payoff_gate",
				Beats: []StoryBeat{
					testStoryBeat("vault.1", "vault.op.1"),
				},
			},
			{
				ElementID: "hazard",
				Family:    "preparedness_hazard",
				Beats: []StoryBeat{
					testStoryBeat("hazard.1", "hazard.op.1"),
					testStoryBeat("hazard.2", "hazard.op.2"),
				},
			},
		},
	}

	first, err := CompileIR(ir)
	if err != nil {
		t.Fatalf("compile ir: %v", err)
	}
	second, err := CompileIR(ir)
	if err != nil {
		t.Fatalf("recompile ir: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("compile should be deterministic for a fixed seed")
	}
	if first.SchemaVersion != "v1alpha1" {
		t.Fatalf("expected default schema_version, got %q", first.SchemaVersion)
	}

	assertBeatBefore(t, first, "glass.1", "glass.2")
	assertBeatBefore(t, first, "hazard.1", "hazard.2")
}

func TestCompileIRDerivesPrecursorAnnotations(t *testing.T) {
	ir := IRFile{
		SeasonID:    "precursor-dev",
		Title:       "Precursor Dev",
		CompileSeed: 9,
		Elements: []StoryElement{
			{
				ElementID: "glass",
				Family:    "seed_clue_chain",
				Beats: []StoryBeat{
					testStoryBeat("glass.clue", "glass.clue.op"),
				},
			},
			{
				ElementID: "vault",
				Family:    "payoff_gate",
				Beats: []StoryBeat{
					testStoryBeatWithTags("vault.challenge", "vault.challenge.op", []string{"glass.signal"}, nil, "glass.clue"),
				},
			},
		},
	}
	ir.Elements[0].Beats[0].ProducesTags = []string{"glass.signal"}

	compiled, err := CompileIR(ir)
	if err != nil {
		t.Fatalf("compile ir: %v", err)
	}

	clueIndex, clueTick := findBeat(t, compiled, "glass.clue")
	challengeIndex, challengeTick := findBeat(t, compiled, "vault.challenge")
	if challengeIndex <= clueIndex {
		t.Fatalf("expected precursor beat to compile before dependent beat")
	}

	annotations := challengeTick.Annotations
	if want := []string{clueTick.TickID}; !reflect.DeepEqual(annotations.PrecursorTickIDs, want) {
		t.Fatalf("unexpected precursor tick ids: got %#v want %#v", annotations.PrecursorTickIDs, want)
	}

	wantDistance := challengeIndex - clueIndex
	if annotations.MemoryDistanceMin != wantDistance {
		t.Fatalf("unexpected memory distance: got %d want %d", annotations.MemoryDistanceMin, wantDistance)
	}
}

func TestValidateIRRejectsUnknownPrecursor(t *testing.T) {
	err := ValidateIR(IRFile{
		SeasonID: "broken-ir",
		Elements: []StoryElement{
			{
				ElementID: "glass",
				Family:    "seed_clue_chain",
				Beats: []StoryBeat{
					testStoryBeat("glass.1", "glass.op.1", "missing"),
				},
			},
		},
	})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), `unknown precursor beat_id "missing"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateIRRejectsImpossibleElementOrder(t *testing.T) {
	err := ValidateIR(IRFile{
		SeasonID: "broken-order",
		Elements: []StoryElement{
			{
				ElementID: "glass",
				Family:    "seed_clue_chain",
				Beats: []StoryBeat{
					testStoryBeat("glass.1", "glass.op.1", "glass.2"),
					testStoryBeat("glass.2", "glass.op.2"),
				},
			},
		},
	})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), `must appear earlier in the same element`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateIRRejectsCycles(t *testing.T) {
	err := ValidateIR(IRFile{
		SeasonID: "broken-cycle",
		Elements: []StoryElement{
			{
				ElementID: "glass",
				Family:    "seed_clue_chain",
				Beats: []StoryBeat{
					testStoryBeat("glass.1", "glass.op.1", "vault.1"),
				},
			},
			{
				ElementID: "vault",
				Family:    "payoff_gate",
				Beats: []StoryBeat{
					testStoryBeat("vault.1", "vault.op.1", "glass.1"),
				},
			},
		},
	})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), `cycle detected in precursor graph`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadIRFileValidatesDevSeasonIR(t *testing.T) {
	loaded, err := LoadIRFile(filepath.Join("..", "..", "seasons", "dev", "season_ir.json"))
	if err != nil {
		t.Fatalf("load dev season ir: %v", err)
	}
	if loaded.SeasonID != "dev-season" {
		t.Fatalf("unexpected season: %s", loaded.SeasonID)
	}
	if len(loaded.Elements) == 0 {
		t.Fatalf("expected story elements")
	}
}

func TestValidateIRRejectsConsumedTagWithoutProducer(t *testing.T) {
	err := ValidateIR(IRFile{
		SeasonID: "broken-tags",
		Elements: []StoryElement{
			{
				ElementID: "vault",
				Family:    "payoff_gate",
				Beats: []StoryBeat{
					testStoryBeatWithTags("vault.1", "vault.op.1", []string{"missing.tag"}, nil),
				},
			},
		},
	})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), `consumes tag "missing.tag" but no beat produces it`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateIRRejectsConsumedTagWithoutGuaranteedProducer(t *testing.T) {
	err := ValidateIR(IRFile{
		SeasonID: "broken-reachability",
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
					testStoryBeatWithTags("gate.1", "gate.op.1", []string{"shared.tag"}, nil),
				},
			},
		},
	})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), `without a guaranteed earlier producer`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateIRAllowsConsumedTagWithGuaranteedProducer(t *testing.T) {
	err := ValidateIR(IRFile{
		SeasonID: "valid-reachability",
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
	})
	if err != nil {
		t.Fatalf("expected valid ir, got %v", err)
	}
}

func TestValidateIRRejectsConditionalTagProducerAsGuaranteed(t *testing.T) {
	err := ValidateIR(IRFile{
		SeasonID: "conditional-producer",
		Elements: []StoryElement{
			{
				ElementID: "seed",
				Family:    "seed_clue_chain",
				Beats: []StoryBeat{
					{
						BeatID:        "seed.1",
						ClockClass:    "standard",
						Sources:       []Source{{SourceID: "seed.1.source", SourceType: "test", Text: "seed"}},
						Opportunities: []Opportunity{{OpportunityID: "seed.op.1", AllowedCommands: []string{"commit", "hold"}}},
						Scoring: ScoringPlan{
							Rules: []Rule{
								{
									Match:          ActionMatch{Command: "commit", Target: "seed.op.1"},
									Effects:        StateEffects{AddTags: []string{"shared.tag"}},
									Delta:          ScoreDelta{Yield: 1},
									Label:          "conditional producer",
									Classification: "best",
								},
								{
									Match:          ActionMatch{Command: "hold"},
									Delta:          ScoreDelta{},
									Label:          "pass",
									Classification: "miss",
								},
							},
						},
					},
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
	})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), `without a guaranteed earlier producer`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func testStoryBeat(beatID, opportunityID string, precursors ...string) StoryBeat {
	return testStoryBeatWithTags(beatID, opportunityID, nil, nil, precursors...)
}

func testStoryBeatWithTags(beatID, opportunityID string, consumesTags, producesTags []string, precursors ...string) StoryBeat {
	return StoryBeat{
		BeatID:        beatID,
		ClockClass:    "standard",
		Sources:       []Source{{SourceID: beatID + ".source", SourceType: "test", Text: beatID + " source"}},
		Opportunities: []Opportunity{{OpportunityID: opportunityID, AllowedCommands: []string{"commit", "hold"}, AllowedOptions: []string{"good"}, TextSlot: false}},
		ProducesTags:  producesTags,
		ConsumesTags:  consumesTags,
		Scoring: ScoringPlan{
			Rules: []Rule{
				{
					Match:          ActionMatch{Command: "commit", Target: opportunityID, Option: "good"},
					Delta:          ScoreDelta{Yield: 1},
					Label:          "best",
					Classification: "best",
				},
				{
					Match:          ActionMatch{Command: "hold"},
					Delta:          ScoreDelta{MissPenalties: 1},
					Label:          "miss",
					Classification: "miss",
				},
			},
		},
		PrecursorBeatIDs: precursors,
	}
}

func assertBeatBefore(t *testing.T, compiled File, firstBeatID, secondBeatID string) {
	t.Helper()

	firstIndex, _ := findBeat(t, compiled, firstBeatID)
	secondIndex, _ := findBeat(t, compiled, secondBeatID)
	if firstIndex >= secondIndex {
		t.Fatalf("expected %s before %s, got indices %d >= %d", firstBeatID, secondBeatID, firstIndex, secondIndex)
	}
}

func findBeat(t *testing.T, compiled File, beatID string) (int, TickDefinition) {
	t.Helper()

	for i, tick := range compiled.Ticks {
		if tick.Annotations.BeatID == beatID {
			return i, tick
		}
	}
	t.Fatalf("beat %q not found", beatID)
	return -1, TickDefinition{}
}
