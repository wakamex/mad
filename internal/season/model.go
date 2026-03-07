package season

import (
	"encoding/json"
	"time"
)

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
	TickID              string         `json:"tick_id"`
	ClockClass          string         `json:"clock_class"`
	DurationMS          int64          `json:"duration_ms"`
	Sources             []Source       `json:"sources"`
	ActiveSourceRegimes []SourceRegime `json:"active_source_regimes,omitempty"`
	Opportunities       []Opportunity  `json:"opportunities"`
	Scoring             ScoringPlan    `json:"scoring"`
	Annotations         Annotations    `json:"annotations,omitempty"`
}

type Source struct {
	SourceID   string `json:"source_id"`
	SourceType string `json:"source_type"`
	Text       string `json:"text"`
}

type SourceRegime struct {
	RegimeID            string   `json:"regime_id"`
	Label               string   `json:"label"`
	Description         string   `json:"description,omitempty"`
	AffectedSourceTypes []string `json:"affected_source_types,omitempty"`
}

type Opportunity struct {
	OpportunityID      string              `json:"opportunity_id"`
	AllowedCommands    []string            `json:"allowed_commands"`
	AllowedOptions     []string            `json:"allowed_options,omitempty"`
	TextSlot           bool                `json:"text_slot"`
	PhraseHint         string              `json:"phrase_hint,omitempty"`
	PublicRequirements []PublicRequirement `json:"public_requirements,omitempty"`
}

type PublicRequirement struct {
	Metric   string `json:"metric"`
	Scope    string `json:"scope,omitempty"`
	Operator string `json:"operator"`
	Value    int64  `json:"value,omitempty"`
	Tier     string `json:"tier,omitempty"`
	Label    string `json:"label,omitempty"`
}

type ScoringPlan struct {
	Rules []Rule `json:"rules"`
}

type Rule struct {
	Match          ActionMatch      `json:"match"`
	Requirements   RuleRequirements `json:"requirements,omitempty"`
	Effects        StateEffects     `json:"effects,omitempty"`
	Delta          ScoreDelta       `json:"delta"`
	Label          string           `json:"label"`
	Classification string           `json:"classification"`
}

type ActionMatch struct {
	Command string `json:"command"`
	Target  string `json:"target,omitempty"`
	Option  string `json:"option,omitempty"`
	Phrase  string `json:"phrase,omitempty"`
}

type RuleRequirements struct {
	RequiresAllTags []string         `json:"requires_all_tags,omitempty"`
	RequiresAnyTags []string         `json:"requires_any_tags,omitempty"`
	ForbidsTags     []string         `json:"forbids_tags,omitempty"`
	MaxDebt         int64            `json:"max_debt,omitempty"`
	MinAura         int64            `json:"min_aura,omitempty"`
	MinReputation   map[string]int64 `json:"min_reputation,omitempty"`
}

type StateEffects struct {
	AddTags           []string         `json:"add_tags,omitempty"`
	RemoveTags        []string         `json:"remove_tags,omitempty"`
	LockTicks         int              `json:"lock_ticks,omitempty"`
	AvailabilityDelta string           `json:"availability_delta,omitempty"`
	InventoryDelta    int              `json:"inventory_delta,omitempty"`
	ReputationDelta   map[string]int64 `json:"reputation_delta,omitempty"`
}

func (r RuleRequirements) IsZero() bool {
	return len(r.RequiresAllTags) == 0 &&
		len(r.RequiresAnyTags) == 0 &&
		len(r.ForbidsTags) == 0 &&
		r.MaxDebt == 0 &&
		r.MinAura == 0 &&
		len(r.MinReputation) == 0
}

func (s StateEffects) IsZero() bool {
	return len(s.AddTags) == 0 &&
		len(s.RemoveTags) == 0 &&
		s.LockTicks == 0 &&
		s.AvailabilityDelta == "" &&
		s.InventoryDelta == 0 &&
		len(s.ReputationDelta) == 0
}

func (r Rule) MarshalJSON() ([]byte, error) {
	type ruleJSON struct {
		Match          ActionMatch       `json:"match"`
		Requirements   *RuleRequirements `json:"requirements,omitempty"`
		Effects        *StateEffects     `json:"effects,omitempty"`
		Delta          ScoreDelta        `json:"delta"`
		Label          string            `json:"label"`
		Classification string            `json:"classification"`
	}

	value := ruleJSON{
		Match:          r.Match,
		Delta:          r.Delta,
		Label:          r.Label,
		Classification: r.Classification,
	}
	if !r.Requirements.IsZero() {
		req := r.Requirements
		value.Requirements = &req
	}
	if !r.Effects.IsZero() {
		effects := r.Effects
		value.Effects = &effects
	}
	return json.Marshal(value)
}

type ScoreDelta struct {
	Yield         int64 `json:"yield"`
	Insight       int64 `json:"insight"`
	Aura          int64 `json:"aura"`
	Debt          int64 `json:"debt"`
	MissPenalties int64 `json:"miss_penalties"`
}

type Annotations struct {
	Family            string   `json:"family,omitempty"`
	ElementID         string   `json:"element_id,omitempty"`
	BeatID            string   `json:"beat_id,omitempty"`
	PrecursorBeatIDs  []string `json:"precursor_beat_ids,omitempty"`
	PrecursorTickIDs  []string `json:"precursor_tick_ids,omitempty"`
	MemoryDistanceMin int      `json:"memory_distance_min,omitempty"`
}

type PublicTick struct {
	TickID              string         `json:"tick_id"`
	ClockClass          string         `json:"clock_class"`
	DeadlineMS          int64          `json:"deadline_ms"`
	Sources             []Source       `json:"sources"`
	ActiveSourceRegimes []SourceRegime `json:"active_source_regimes,omitempty"`
	Opportunities       []Opportunity  `json:"opportunities"`
}

func (t TickDefinition) Public() PublicTick {
	return PublicTick{
		TickID:              t.TickID,
		ClockClass:          t.ClockClass,
		DeadlineMS:          t.DurationMS,
		Sources:             t.Sources,
		ActiveSourceRegimes: t.ActiveSourceRegimes,
		Opportunities:       t.Opportunities,
	}
}

func (f File) DurationForTick(idx int) time.Duration {
	if idx < 0 || idx >= len(f.Ticks) {
		return 0
	}
	return time.Duration(f.Ticks[idx].DurationMS) * time.Millisecond
}
