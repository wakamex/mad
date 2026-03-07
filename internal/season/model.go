package season

import "time"

type File struct {
	SchemaVersion   string           `json:"schema_version"`
	SeasonID        string           `json:"season_id"`
	Title           string           `json:"title"`
	ScoreEpochTicks int              `json:"score_epoch_ticks"`
	RevealLagTicks  int              `json:"reveal_lag_ticks"`
	ShardCount      int              `json:"shard_count"`
	Ticks           []TickDefinition `json:"ticks"`
}

type TickDefinition struct {
	TickID        string        `json:"tick_id"`
	ClockClass    string        `json:"clock_class"`
	DurationMS    int64         `json:"duration_ms"`
	Sources       []Source      `json:"sources"`
	Opportunities []Opportunity `json:"opportunities"`
	Scoring       ScoringPlan   `json:"scoring"`
	Annotations   Annotations   `json:"annotations,omitempty"`
}

type Source struct {
	SourceID   string `json:"source_id"`
	SourceType string `json:"source_type"`
	Text       string `json:"text"`
}

type Opportunity struct {
	OpportunityID   string   `json:"opportunity_id"`
	AllowedCommands []string `json:"allowed_commands"`
	AllowedOptions  []string `json:"allowed_options,omitempty"`
	TextSlot        bool     `json:"text_slot"`
	PhraseHint      string   `json:"phrase_hint,omitempty"`
}

type ScoringPlan struct {
	Rules []Rule `json:"rules"`
}

type Rule struct {
	Match          ActionMatch `json:"match"`
	Delta          ScoreDelta  `json:"delta"`
	Label          string      `json:"label"`
	Classification string      `json:"classification"`
}

type ActionMatch struct {
	Command string `json:"command"`
	Target  string `json:"target,omitempty"`
	Option  string `json:"option,omitempty"`
	Phrase  string `json:"phrase,omitempty"`
}

type ScoreDelta struct {
	Yield         int64 `json:"yield"`
	Insight       int64 `json:"insight"`
	Aura          int64 `json:"aura"`
	Debt          int64 `json:"debt"`
	MissPenalties int64 `json:"miss_penalties"`
}

type Annotations struct {
	ElementID         string   `json:"element_id,omitempty"`
	BeatID            string   `json:"beat_id,omitempty"`
	PrecursorBeatIDs  []string `json:"precursor_beat_ids,omitempty"`
	PrecursorTickIDs  []string `json:"precursor_tick_ids,omitempty"`
	MemoryDistanceMin int      `json:"memory_distance_min,omitempty"`
}

type PublicTick struct {
	TickID        string        `json:"tick_id"`
	ClockClass    string        `json:"clock_class"`
	DeadlineMS    int64         `json:"deadline_ms"`
	Sources       []Source      `json:"sources"`
	Opportunities []Opportunity `json:"opportunities"`
}

func (t TickDefinition) Public() PublicTick {
	return PublicTick{
		TickID:        t.TickID,
		ClockClass:    t.ClockClass,
		DeadlineMS:    t.DurationMS,
		Sources:       t.Sources,
		Opportunities: t.Opportunities,
	}
}

func (f File) DurationForTick(idx int) time.Duration {
	if idx < 0 || idx >= len(f.Ticks) {
		return 0
	}
	return time.Duration(f.Ticks[idx].DurationMS) * time.Millisecond
}
