package season

import "fmt"

type SimulationReport struct {
	SchemaVersion string                       `json:"schema_version"`
	SeasonID      string                       `json:"season_id"`
	Title         string                       `json:"title"`
	TickCount     int                          `json:"tick_count"`
	TotalDuration int64                        `json:"total_duration_ms"`
	Baselines     map[string]SimulatedBaseline `json:"baselines"`
	Ticks         []SimulatedTick              `json:"ticks"`
	Reveals       []SimulatedReveal            `json:"reveals"`
}

type SimulatedBaseline struct {
	Ledger     SimulatedLedger `json:"ledger"`
	ScoreTrace []int64         `json:"score_trace"`
}

type SimulatedLedger struct {
	Score         int64 `json:"score"`
	Yield         int64 `json:"yield"`
	Insight       int64 `json:"insight"`
	Aura          int64 `json:"aura"`
	Debt          int64 `json:"debt"`
	MissPenalties int64 `json:"miss_penalties"`
}

type SimulatedTick struct {
	Index                 int                  `json:"index"`
	TickID                string               `json:"tick_id"`
	ClockClass            string               `json:"clock_class"`
	DurationMS            int64                `json:"duration_ms"`
	StartsAtMS            int64                `json:"starts_at_ms"`
	EndsAtMS              int64                `json:"ends_at_ms"`
	RevealPublishesAtTick string               `json:"reveal_publishes_at_tick,omitempty"`
	Annotations           Annotations          `json:"annotations,omitempty"`
	ResolutionPreview     *SimulatedResolution `json:"resolution_preview,omitempty"`
}

type SimulatedReveal struct {
	TickID               string               `json:"tick_id"`
	RevealLagTicks       int                  `json:"reveal_lag_ticks"`
	PublishedAfterTickID string               `json:"published_after_tick_id"`
	PublishedAfterIndex  int                  `json:"published_after_index"`
	ResolutionPreview    *SimulatedResolution `json:"resolution_preview,omitempty"`
}

type SimulatedResolution struct {
	OpportunityID     string               `json:"opportunity_id,omitempty"`
	BestKnownAction   SimulatedAction      `json:"best_known_action"`
	BadActionClasses  []SimulatedBadAction `json:"bad_action_classes,omitempty"`
	PublicExplanation string               `json:"public_explanation,omitempty"`
}

type SimulatedAction struct {
	Command string `json:"command"`
	Target  string `json:"target,omitempty"`
	Option  string `json:"option,omitempty"`
	Phrase  string `json:"phrase,omitempty"`
}

type SimulatedBadAction struct {
	Command string `json:"command"`
	Option  string `json:"option,omitempty"`
	Outcome string `json:"outcome"`
}

func Simulate(file File) (SimulationReport, error) {
	if err := Validate(file); err != nil {
		return SimulationReport{}, err
	}

	report := SimulationReport{
		SchemaVersion: file.SchemaVersion,
		SeasonID:      file.SeasonID,
		Title:         file.Title,
		TickCount:     len(file.Ticks),
		Baselines: map[string]SimulatedBaseline{
			"perfect_best": {},
			"always_hold":  {},
		},
		Ticks: make([]SimulatedTick, 0, len(file.Ticks)),
	}

	var nowMS int64
	for i, tick := range file.Ticks {
		bestBaseline := report.Baselines["perfect_best"]
		updateBaseline(&bestBaseline, bestDeltaForTick(tick))
		report.Baselines["perfect_best"] = bestBaseline

		holdBaseline := report.Baselines["always_hold"]
		updateBaseline(&holdBaseline, holdDeltaForTick(tick))
		report.Baselines["always_hold"] = holdBaseline

		resolution := simulateResolution(tick)
		simTick := SimulatedTick{
			Index:             i,
			TickID:            tick.TickID,
			ClockClass:        tick.ClockClass,
			DurationMS:        tick.DurationMS,
			StartsAtMS:        nowMS,
			EndsAtMS:          nowMS + tick.DurationMS,
			Annotations:       tick.Annotations,
			ResolutionPreview: resolution,
		}
		publishIndex := i + file.RevealLagTicks - 1
		if file.RevealLagTicks > 0 && publishIndex >= 0 && publishIndex < len(file.Ticks) {
			simTick.RevealPublishesAtTick = file.Ticks[publishIndex].TickID
			report.Reveals = append(report.Reveals, SimulatedReveal{
				TickID:               tick.TickID,
				RevealLagTicks:       file.RevealLagTicks,
				PublishedAfterTickID: file.Ticks[publishIndex].TickID,
				PublishedAfterIndex:  publishIndex,
				ResolutionPreview:    resolution,
			})
		}
		report.Ticks = append(report.Ticks, simTick)
		nowMS += tick.DurationMS
	}
	report.TotalDuration = nowMS
	return report, nil
}

func simulateResolution(tick TickDefinition) *SimulatedResolution {
	bestRule, ok := bestRuleForTick(tick)
	if !ok {
		return nil
	}

	resolution := &SimulatedResolution{
		PublicExplanation: bestRule.Label,
		BestKnownAction: SimulatedAction{
			Command: bestRule.Match.Command,
			Target:  bestRule.Match.Target,
			Option:  bestRule.Match.Option,
			Phrase:  bestRule.Match.Phrase,
		},
	}
	if len(tick.Opportunities) > 0 {
		resolution.OpportunityID = tick.Opportunities[0].OpportunityID
	}

	for _, rule := range tick.Scoring.Rules {
		switch rule.Classification {
		case "bad", "miss":
			resolution.BadActionClasses = append(resolution.BadActionClasses, SimulatedBadAction{
				Command: rule.Match.Command,
				Option:  rule.Match.Option,
				Outcome: rule.Label,
			})
		}
	}
	return resolution
}

func bestRuleForTick(tick TickDefinition) (Rule, bool) {
	for _, rule := range tick.Scoring.Rules {
		if rule.Classification == "best" {
			return rule, true
		}
	}
	return Rule{}, false
}

func (r SimulationReport) Summary() string {
	return fmt.Sprintf("%s: %d ticks, %d reveals, %dms total duration", r.SeasonID, r.TickCount, len(r.Reveals), r.TotalDuration)
}

func bestDeltaForTick(tick TickDefinition) ScoreDelta {
	if bestRule, ok := bestRuleForTick(tick); ok {
		return bestRule.Delta
	}
	return holdDeltaForTick(tick)
}

func holdDeltaForTick(tick TickDefinition) ScoreDelta {
	for _, rule := range tick.Scoring.Rules {
		if rule.Match.Command == "hold" {
			return rule.Delta
		}
	}
	return ScoreDelta{}
}

func updateBaseline(baseline *SimulatedBaseline, delta ScoreDelta) {
	baseline.Ledger.Yield += delta.Yield
	baseline.Ledger.Insight += delta.Insight
	baseline.Ledger.Aura += delta.Aura
	baseline.Ledger.Debt += delta.Debt
	baseline.Ledger.MissPenalties += delta.MissPenalties
	baseline.Ledger.Score = baseline.Ledger.Yield + baseline.Ledger.Insight + baseline.Ledger.Aura - baseline.Ledger.Debt - baseline.Ledger.MissPenalties
	baseline.ScoreTrace = append(baseline.ScoreTrace, baseline.Ledger.Score)
}
