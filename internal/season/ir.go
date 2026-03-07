package season

import (
	"fmt"
	"math/rand"
)

type IRFile struct {
	SchemaVersion   string           `json:"schema_version"`
	SeasonID        string           `json:"season_id"`
	Title           string           `json:"title"`
	CompileSeed     int64            `json:"compile_seed"`
	ScoreEpochTicks int              `json:"score_epoch_ticks"`
	RevealLagTicks  int              `json:"reveal_lag_ticks"`
	ShardCount      int              `json:"shard_count"`
	ClockDefaults   map[string]int64 `json:"clock_defaults"`
	Elements        []StoryElement   `json:"elements"`
}

type StoryElement struct {
	ElementID string      `json:"element_id"`
	Beats     []StoryBeat `json:"beats"`
}

type StoryBeat struct {
	BeatID           string        `json:"beat_id"`
	ClockClass       string        `json:"clock_class"`
	Sources          []Source      `json:"sources"`
	Opportunities    []Opportunity `json:"opportunities"`
	Scoring          ScoringPlan   `json:"scoring"`
	PrecursorBeatIDs []string      `json:"precursor_beat_ids,omitempty"`
}

func CompileIR(ir IRFile) (File, error) {
	if len(ir.Elements) == 0 {
		return File{}, fmt.Errorf("at least one story element is required")
	}

	if ir.SchemaVersion == "" {
		ir.SchemaVersion = "v1alpha1"
	}
	if ir.ScoreEpochTicks <= 0 {
		ir.ScoreEpochTicks = 2
	}
	if ir.RevealLagTicks <= 0 {
		ir.RevealLagTicks = ir.ScoreEpochTicks
	}
	if ir.ShardCount <= 0 {
		ir.ShardCount = 16
	}
	if ir.ClockDefaults == nil {
		ir.ClockDefaults = map[string]int64{
			"standard":  90_000,
			"dossier":   300_000,
			"interrupt": 30_000,
		}
	}

	if err := ValidateIR(ir); err != nil {
		return File{}, err
	}

	totalBeats := totalBeatCount(ir.Elements)
	rng := rand.New(rand.NewSource(ir.CompileSeed))
	cursors := make([]int, len(ir.Elements))
	compiled := make([]TickDefinition, 0, totalBeats)
	beatToTick := make(map[string]string, totalBeats)
	beatToIndex := make(map[string]int, totalBeats)

	for len(compiled) < totalBeats {
		available := make([]int, 0, len(ir.Elements))
		for i, element := range ir.Elements {
			if cursors[i] >= len(element.Beats) {
				continue
			}
			beat := element.Beats[cursors[i]]
			if precursorsSatisfied(beat.PrecursorBeatIDs, beatToTick) {
				available = append(available, i)
			}
		}
		if len(available) == 0 {
			return File{}, fmt.Errorf("cannot weave season ir: no schedulable beats remain; check precursor dependencies and element order")
		}
		chosen := available[rng.Intn(len(available))]
		element := ir.Elements[chosen]
		beat := element.Beats[cursors[chosen]]
		cursors[chosen]++

		tickID := fmt.Sprintf("S1-T%04d", len(compiled)+1)
		tick := TickDefinition{
			TickID:        tickID,
			ClockClass:    beat.ClockClass,
			DurationMS:    durationForClockClass(ir.ClockDefaults, beat.ClockClass),
			Sources:       beat.Sources,
			Opportunities: beat.Opportunities,
			Scoring:       beat.Scoring,
			Annotations: Annotations{
				ElementID:        element.ElementID,
				BeatID:           beat.BeatID,
				PrecursorBeatIDs: append([]string(nil), beat.PrecursorBeatIDs...),
			},
		}
		compiled = append(compiled, tick)
		beatToTick[beat.BeatID] = tickID
		beatToIndex[beat.BeatID] = len(compiled) - 1
	}

	for i := range compiled {
		annotation := &compiled[i].Annotations
		if len(annotation.PrecursorBeatIDs) == 0 {
			continue
		}
		minDistance := -1
		for _, precursorBeatID := range annotation.PrecursorBeatIDs {
			if precursorTickID, ok := beatToTick[precursorBeatID]; ok {
				annotation.PrecursorTickIDs = append(annotation.PrecursorTickIDs, precursorTickID)
				distance := i - beatToIndex[precursorBeatID]
				if minDistance == -1 || distance < minDistance {
					minDistance = distance
				}
			}
		}
		if minDistance > 0 {
			annotation.MemoryDistanceMin = minDistance
		}
	}

	file := File{
		SchemaVersion:   ir.SchemaVersion,
		SeasonID:        ir.SeasonID,
		Title:           ir.Title,
		ScoreEpochTicks: ir.ScoreEpochTicks,
		RevealLagTicks:  ir.RevealLagTicks,
		ShardCount:      ir.ShardCount,
		Ticks:           compiled,
	}
	return file, Validate(file)
}

func ValidateIR(ir IRFile) error {
	if ir.SeasonID == "" {
		return fmt.Errorf("season_id is required")
	}
	seenElements := make(map[string]struct{}, len(ir.Elements))
	seenBeats := make(map[string]struct{}, totalBeatCount(ir.Elements))
	beatLocations := make(map[string]beatLocation, totalBeatCount(ir.Elements))

	for elementIndex, element := range ir.Elements {
		if element.ElementID == "" {
			return fmt.Errorf("element[%d]: element_id is required", elementIndex)
		}
		if _, exists := seenElements[element.ElementID]; exists {
			return fmt.Errorf("element[%d]: duplicate element_id %q", elementIndex, element.ElementID)
		}
		seenElements[element.ElementID] = struct{}{}
		if len(element.Beats) == 0 {
			return fmt.Errorf("element[%d]: at least one beat is required", elementIndex)
		}
		for beatIndex, beat := range element.Beats {
			if beat.BeatID == "" {
				return fmt.Errorf("element[%d] beat[%d]: beat_id is required", elementIndex, beatIndex)
			}
			if _, exists := seenBeats[beat.BeatID]; exists {
				return fmt.Errorf("element[%d] beat[%d]: duplicate beat_id %q", elementIndex, beatIndex, beat.BeatID)
			}
			seenBeats[beat.BeatID] = struct{}{}
			beatLocations[beat.BeatID] = beatLocation{elementIndex: elementIndex, beatIndex: beatIndex}
			if beat.ClockClass == "" {
				return fmt.Errorf("element[%d] beat[%d]: clock_class is required", elementIndex, beatIndex)
			}
			if len(beat.Opportunities) == 0 {
				return fmt.Errorf("element[%d] beat[%d]: at least one opportunity is required", elementIndex, beatIndex)
			}
			if len(beat.Scoring.Rules) == 0 {
				return fmt.Errorf("element[%d] beat[%d]: at least one scoring rule is required", elementIndex, beatIndex)
			}
		}
	}

	for elementIndex, element := range ir.Elements {
		for beatIndex, beat := range element.Beats {
			for _, precursorBeatID := range beat.PrecursorBeatIDs {
				location, exists := beatLocations[precursorBeatID]
				if !exists {
					return fmt.Errorf("element[%d] beat[%d]: unknown precursor beat_id %q", elementIndex, beatIndex, precursorBeatID)
				}
				if location.elementIndex == elementIndex && location.beatIndex >= beatIndex {
					return fmt.Errorf("element[%d] beat[%d]: precursor beat_id %q must appear earlier in the same element", elementIndex, beatIndex, precursorBeatID)
				}
			}
		}
	}
	if err := validateBeatGraph(ir); err != nil {
		return err
	}
	return nil
}

type beatLocation struct {
	elementIndex int
	beatIndex    int
}

func validateBeatGraph(ir IRFile) error {
	visiting := make(map[string]bool, totalBeatCount(ir.Elements))
	visited := make(map[string]bool, totalBeatCount(ir.Elements))
	edges := make(map[string][]string, totalBeatCount(ir.Elements))
	for _, element := range ir.Elements {
		for _, beat := range element.Beats {
			edges[beat.BeatID] = append([]string(nil), beat.PrecursorBeatIDs...)
		}
	}

	var visit func(string) error
	visit = func(beatID string) error {
		if visited[beatID] {
			return nil
		}
		if visiting[beatID] {
			return fmt.Errorf("cycle detected in precursor graph at beat %q", beatID)
		}
		visiting[beatID] = true
		for _, precursorBeatID := range edges[beatID] {
			if err := visit(precursorBeatID); err != nil {
				return err
			}
		}
		visiting[beatID] = false
		visited[beatID] = true
		return nil
	}

	for beatID := range edges {
		if err := visit(beatID); err != nil {
			return err
		}
	}
	return nil
}

func precursorsSatisfied(precursorBeatIDs []string, beatToTick map[string]string) bool {
	for _, precursorBeatID := range precursorBeatIDs {
		if _, ok := beatToTick[precursorBeatID]; !ok {
			return false
		}
	}
	return true
}

func totalBeatCount(elements []StoryElement) int {
	total := 0
	for _, element := range elements {
		total += len(element.Beats)
	}
	return total
}

func durationForClockClass(defaults map[string]int64, class string) int64 {
	if duration, ok := defaults[class]; ok && duration > 0 {
		return duration
	}
	switch class {
	case "dossier":
		return 300_000
	case "interrupt":
		return 30_000
	default:
		return 90_000
	}
}
