package season

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFileValidatesDevSeason(t *testing.T) {
	loaded, err := LoadFile(filepath.Join("..", "..", "seasons", "dev", "season.json"))
	if err != nil {
		t.Fatalf("load dev season: %v", err)
	}
	if loaded.SeasonID == "" {
		t.Fatalf("expected non-empty season ID")
	}
}

func TestValidateRejectsBrokenSeason(t *testing.T) {
	err := Validate(File{
		SchemaVersion:   "v1alpha1",
		SeasonID:        "broken",
		ScoreEpochTicks: 1,
		RevealLagTicks:  1,
		ShardCount:      1,
		Ticks: []TickDefinition{
			{
				TickID:     "T1",
				DurationMS: 1000,
				Opportunities: []Opportunity{
					{
						OpportunityID:   "op.1",
						AllowedCommands: []string{"commit"},
						AllowedOptions:  []string{"good"},
					},
				},
				Scoring: ScoringPlan{
					Rules: []Rule{
						{
							Match: ActionMatch{
								Command: "commit",
								Target:  "op.1",
								Option:  "bad",
							},
							Classification: "best",
						},
					},
				},
			},
		},
	})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	got := err.Error()
	for _, want := range []string{
		`option "bad" is not allowed`,
		"scoring must include a hold fallback rule",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("validation error missing %q in %q", want, got)
		}
	}
}
