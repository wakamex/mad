package season

import (
	"math"
	"sort"
)

type ScoreBreakdown struct {
	ByFamily     []ScoreBreakdownEntry `json:"by_family,omitempty"`
	ByElement    []ScoreBreakdownEntry `json:"by_element,omitempty"`
	BySourceType []ScoreBreakdownEntry `json:"by_source_type,omitempty"`
}

type ScoreBreakdownEntry struct {
	Key           string  `json:"key"`
	Score         float64 `json:"score"`
	Yield         float64 `json:"yield"`
	Insight       float64 `json:"insight"`
	Aura          float64 `json:"aura"`
	Debt          float64 `json:"debt"`
	MissPenalties float64 `json:"miss_penalties"`
	Occurrences   int     `json:"occurrences"`
	BestCount     int     `json:"best_count"`
	BadCount      int     `json:"bad_count"`
	MissCount     int     `json:"miss_count"`
}

type ScoreBreakdownAccumulator struct {
	byFamily     map[string]*ScoreBreakdownEntry
	byElement    map[string]*ScoreBreakdownEntry
	bySourceType map[string]*ScoreBreakdownEntry
}

func NewScoreBreakdownAccumulator() ScoreBreakdownAccumulator {
	return ScoreBreakdownAccumulator{
		byFamily:     make(map[string]*ScoreBreakdownEntry),
		byElement:    make(map[string]*ScoreBreakdownEntry),
		bySourceType: make(map[string]*ScoreBreakdownEntry),
	}
}

func (a *ScoreBreakdownAccumulator) Add(tick TickDefinition, rule Rule) {
	a.add(a.byFamily, tick.Annotations.Family, rule, 1.0)
	a.add(a.byElement, tick.Annotations.ElementID, rule, 1.0)

	sourceTypes := distinctSourceTypes(tick.Sources)
	if len(sourceTypes) == 0 {
		return
	}
	share := 1.0 / float64(len(sourceTypes))
	for _, sourceType := range sourceTypes {
		a.add(a.bySourceType, sourceType, rule, share)
	}
}

func (a *ScoreBreakdownAccumulator) Materialize() ScoreBreakdown {
	return ScoreBreakdown{
		ByFamily:     materializeBreakdownEntries(a.byFamily),
		ByElement:    materializeBreakdownEntries(a.byElement),
		BySourceType: materializeBreakdownEntries(a.bySourceType),
	}
}

func (a *ScoreBreakdownAccumulator) add(group map[string]*ScoreBreakdownEntry, key string, rule Rule, weight float64) {
	if key == "" {
		return
	}
	entry, ok := group[key]
	if !ok {
		entry = &ScoreBreakdownEntry{Key: key}
		group[key] = entry
	}
	entry.Score += float64(scalarScore(rule.Delta)) * weight
	entry.Yield += float64(rule.Delta.Yield) * weight
	entry.Insight += float64(rule.Delta.Insight) * weight
	entry.Aura += float64(rule.Delta.Aura) * weight
	entry.Debt += float64(rule.Delta.Debt) * weight
	entry.MissPenalties += float64(rule.Delta.MissPenalties) * weight
	entry.Occurrences++
	switch rule.Classification {
	case "best":
		entry.BestCount++
	case "bad":
		entry.BadCount++
	case "miss":
		entry.MissCount++
	}
}

func materializeBreakdownEntries(group map[string]*ScoreBreakdownEntry) []ScoreBreakdownEntry {
	if len(group) == 0 {
		return nil
	}
	entries := make([]ScoreBreakdownEntry, 0, len(group))
	for _, entry := range group {
		entries = append(entries, *entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		left := math.Abs(entries[i].Score)
		right := math.Abs(entries[j].Score)
		if left == right {
			return entries[i].Key < entries[j].Key
		}
		return left > right
	})
	return entries
}

func distinctSourceTypes(sources []Source) []string {
	if len(sources) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(sources))
	values := make([]string, 0, len(sources))
	for _, source := range sources {
		if source.SourceType == "" {
			continue
		}
		if _, ok := seen[source.SourceType]; ok {
			continue
		}
		seen[source.SourceType] = struct{}{}
		values = append(values, source.SourceType)
	}
	sort.Strings(values)
	return values
}
