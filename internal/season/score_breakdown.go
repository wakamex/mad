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

type ScoreDecomposition struct {
	ExplicitVisibleTotal  float64                   `json:"explicit_visible_total"`
	HiddenOrNonlocalTotal float64                   `json:"hidden_or_nonlocal_premium_total"`
	ExplicitVisibleShare  float64                   `json:"explicit_visible_share_of_greedy,omitempty"`
	ByFamily              []ScoreDecompositionEntry `json:"by_family,omitempty"`
	ByElement             []ScoreDecompositionEntry `json:"by_element,omitempty"`
	BySourceType          []ScoreDecompositionEntry `json:"by_source_type,omitempty"`
}

type ScoreDecompositionEntry struct {
	Key              string  `json:"key"`
	ExplicitVisible  float64 `json:"explicit_visible"`
	HiddenOrNonlocal float64 `json:"hidden_or_nonlocal_premium"`
	GreedyBest       float64 `json:"greedy_best"`
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

func DeriveScoreDecomposition(greedy, visible ScoreBreakdown, greedyTotal, visibleTotal int64) ScoreDecomposition {
	decomposition := ScoreDecomposition{
		ExplicitVisibleTotal:  float64(visibleTotal),
		HiddenOrNonlocalTotal: float64(greedyTotal - visibleTotal),
		ByFamily:              deriveScoreDecompositionEntries(greedy.ByFamily, visible.ByFamily),
		ByElement:             deriveScoreDecompositionEntries(greedy.ByElement, visible.ByElement),
		BySourceType:          deriveScoreDecompositionEntries(greedy.BySourceType, visible.BySourceType),
	}
	if greedyTotal != 0 {
		decomposition.ExplicitVisibleShare = float64(visibleTotal) / float64(greedyTotal)
	}
	return decomposition
}

func deriveScoreDecompositionEntries(greedyEntries, visibleEntries []ScoreBreakdownEntry) []ScoreDecompositionEntry {
	if len(greedyEntries) == 0 && len(visibleEntries) == 0 {
		return nil
	}

	greedy := make(map[string]float64, len(greedyEntries))
	for _, entry := range greedyEntries {
		greedy[entry.Key] = entry.Score
	}
	visible := make(map[string]float64, len(visibleEntries))
	for _, entry := range visibleEntries {
		visible[entry.Key] = entry.Score
	}

	keys := make([]string, 0, maxInt(len(greedy), len(visible)))
	seen := make(map[string]struct{}, len(greedy)+len(visible))
	for key := range greedy {
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	for key := range visible {
		if _, ok := seen[key]; ok {
			continue
		}
		keys = append(keys, key)
	}

	entries := make([]ScoreDecompositionEntry, 0, len(keys))
	for _, key := range keys {
		greedyScore := greedy[key]
		visibleScore := visible[key]
		entries = append(entries, ScoreDecompositionEntry{
			Key:              key,
			ExplicitVisible:  visibleScore,
			HiddenOrNonlocal: greedyScore - visibleScore,
			GreedyBest:       greedyScore,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		left := math.Abs(entries[i].HiddenOrNonlocal)
		right := math.Abs(entries[j].HiddenOrNonlocal)
		if left == right {
			return entries[i].Key < entries[j].Key
		}
		return left > right
	})
	return entries
}
