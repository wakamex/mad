package season

import (
	"fmt"
	"math/rand"
	"slices"
	"strings"
)

type SimulationReport struct {
	SchemaVersion string                       `json:"schema_version"`
	SeasonID      string                       `json:"season_id"`
	Title         string                       `json:"title"`
	TickCount     int                          `json:"tick_count"`
	TotalDuration int64                        `json:"total_duration_ms"`
	Notes         []string                     `json:"notes,omitempty"`
	Baselines     map[string]SimulatedBaseline `json:"baselines"`
	RandomAudit   *SimulatedRandomAudit        `json:"random_audit,omitempty"`
	ActionSurface SimulatedActionSurface       `json:"action_surface"`
	Ticks         []SimulatedTick              `json:"ticks"`
	Reveals       []SimulatedReveal            `json:"reveals"`
}

type SimulationOptions struct {
	RandomRuns int   `json:"random_runs,omitempty"`
	RandomSeed int64 `json:"random_seed,omitempty"`
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

type SimulatedRandomAudit struct {
	Runs            int      `json:"runs"`
	Seed            int64    `json:"seed"`
	MeanScore       float64  `json:"mean_score"`
	MedianScore     int64    `json:"median_score"`
	P90Score        int64    `json:"p90_score"`
	P99Score        int64    `json:"p99_score"`
	MinScore        int64    `json:"min_score"`
	MaxScore        int64    `json:"max_score"`
	PositiveRate    float64  `json:"positive_rate"`
	NonNegativeRate float64  `json:"non_negative_rate"`
	BeatBestRate    float64  `json:"beat_best_rate"`
	Warnings        []string `json:"warnings,omitempty"`
}

type SimulatedActionSurface struct {
	PhraseVariantCount int            `json:"phrase_variant_count"`
	Distribution       map[string]int `json:"distribution"`
	PerTickCounts      map[string]int `json:"per_tick_counts"`
}

type SimulatedTick struct {
	Index                 int                  `json:"index"`
	TickID                string               `json:"tick_id"`
	ClockClass            string               `json:"clock_class"`
	DurationMS            int64                `json:"duration_ms"`
	StartsAtMS            int64                `json:"starts_at_ms"`
	EndsAtMS              int64                `json:"ends_at_ms"`
	RandomActionCount     int                  `json:"random_action_count"`
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

type simulatedPlayerState struct {
	Ledger       SimulatedLedger
	Tags         map[string]struct{}
	Reputation   map[string]int64
	Availability string
	Inventory    int
}

func Simulate(file File) (SimulationReport, error) {
	return SimulateWithOptions(file, SimulationOptions{})
}

func SimulateWithOptions(file File, options SimulationOptions) (SimulationReport, error) {
	if err := Validate(file); err != nil {
		return SimulationReport{}, err
	}

	report := SimulationReport{
		SchemaVersion: file.SchemaVersion,
		SeasonID:      file.SeasonID,
		Title:         file.Title,
		TickCount:     len(file.Ticks),
		Notes: []string{
			"`greedy_best` is a tick-local baseline derived from rules classified as `best`; it is not a globally optimal season policy once lock-ins, opportunity costs, or stateful commitments exist.",
		},
		Baselines: map[string]SimulatedBaseline{
			"greedy_best": {},
			"always_hold": {},
		},
		ActionSurface: SimulatedActionSurface{
			PhraseVariantCount: randomPhraseVariantCount,
			Distribution:       make(map[string]int),
			PerTickCounts:      make(map[string]int, len(file.Ticks)),
		},
		Ticks: make([]SimulatedTick, 0, len(file.Ticks)),
	}

	greedyState := newSimulatedPlayerState()
	holdState := newSimulatedPlayerState()
	var nowMS int64
	for i, tick := range file.Ticks {
		resolution := simulateResolution(tick, greedyState)

		bestBaseline := report.Baselines["greedy_best"]
		advanceBaseline(&bestBaseline, &greedyState, chooseGreedyRule(tick, greedyState))
		report.Baselines["greedy_best"] = bestBaseline

		holdBaseline := report.Baselines["always_hold"]
		advanceBaseline(&holdBaseline, &holdState, chooseHoldRule(tick, holdState))
		report.Baselines["always_hold"] = holdBaseline

		simTick := SimulatedTick{
			Index:             i,
			TickID:            tick.TickID,
			ClockClass:        tick.ClockClass,
			DurationMS:        tick.DurationMS,
			StartsAtMS:        nowMS,
			EndsAtMS:          nowMS + tick.DurationMS,
			RandomActionCount: randomActionCountForTick(tick),
			Annotations:       tick.Annotations,
			ResolutionPreview: resolution,
		}
		report.ActionSurface.PerTickCounts[tick.TickID] = simTick.RandomActionCount
		report.ActionSurface.Distribution[fmt.Sprintf("%d", simTick.RandomActionCount)]++
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
	if options.RandomRuns > 0 {
		report.RandomAudit = simulateRandomAudit(file, options)
	}
	return report, nil
}

const randomPhraseVariantCount = 8 * 8 * 8

func simulateResolution(tick TickDefinition, state simulatedPlayerState) *SimulatedResolution {
	bestRule, ok := bestRuleForTick(tick, state)
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
		if !requirementsMet(rule.Requirements, state) {
			continue
		}
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

func bestRuleForTick(tick TickDefinition, state simulatedPlayerState) (Rule, bool) {
	var bestRule Rule
	var bestScore int64
	found := false
	for _, rule := range tick.Scoring.Rules {
		if rule.Classification != "best" || !requirementsMet(rule.Requirements, state) {
			continue
		}
		score := scalarScore(rule.Delta)
		if !found || score > bestScore {
			bestRule = rule
			bestScore = score
			found = true
		}
	}
	return bestRule, found
}

func (r SimulationReport) Summary() string {
	return fmt.Sprintf("%s: %d ticks, %d reveals, %dms total duration", r.SeasonID, r.TickCount, len(r.Reveals), r.TotalDuration)
}

func chooseGreedyRule(tick TickDefinition, state simulatedPlayerState) Rule {
	if bestRule, ok := bestRuleForTick(tick, state); ok {
		return bestRule
	}
	return chooseHoldRule(tick, state)
}

func chooseHoldRule(tick TickDefinition, state simulatedPlayerState) Rule {
	for _, rule := range tick.Scoring.Rules {
		if rule.Match.Command == "hold" && requirementsMet(rule.Requirements, state) {
			return rule
		}
	}
	return Rule{
		Match:          ActionMatch{Command: "hold"},
		Delta:          ScoreDelta{},
		Label:          "No eligible hold rule matched.",
		Classification: "miss",
	}
}

func advanceBaseline(baseline *SimulatedBaseline, state *simulatedPlayerState, rule Rule) {
	applyRuleToSimulatedState(state, rule)
	baseline.Ledger = state.Ledger
	baseline.ScoreTrace = append(baseline.ScoreTrace, baseline.Ledger.Score)
}

func simulateRandomAudit(file File, options SimulationOptions) *SimulatedRandomAudit {
	rng := rand.New(rand.NewSource(options.RandomSeed))
	scores := make([]int64, 0, options.RandomRuns)
	positive := 0
	nonNegative := 0
	totalBeatBest := 0
	totalChoices := 0

	for i := 0; i < options.RandomRuns; i++ {
		state := newSimulatedPlayerState()
		beatBestHits := 0
		for _, tick := range file.Ticks {
			action := randomActionForTick(tick, rng)
			rule, isBest := evaluateSimulatedAction(tick.Scoring, action, state)
			applyRuleToSimulatedState(&state, rule)
			if isBest {
				beatBestHits++
			}
		}
		scores = append(scores, state.Ledger.Score)
		totalBeatBest += beatBestHits
		totalChoices += len(file.Ticks)
		if state.Ledger.Score > 0 {
			positive++
		}
		if state.Ledger.Score >= 0 {
			nonNegative++
		}
	}
	slices.Sort(scores)

	mean := 0.0
	for _, score := range scores {
		mean += float64(score)
	}
	mean /= float64(len(scores))

	audit := &SimulatedRandomAudit{
		Runs:            options.RandomRuns,
		Seed:            options.RandomSeed,
		MeanScore:       mean,
		MedianScore:     quantileScore(scores, 0.50),
		P90Score:        quantileScore(scores, 0.90),
		P99Score:        quantileScore(scores, 0.99),
		MinScore:        scores[0],
		MaxScore:        scores[len(scores)-1],
		PositiveRate:    float64(positive) / float64(len(scores)),
		NonNegativeRate: float64(nonNegative) / float64(len(scores)),
		BeatBestRate:    float64(totalBeatBest) / float64(totalChoices),
	}
	switch {
	case audit.MeanScore >= 0:
		audit.Warnings = append(audit.Warnings, "random mean score is non-negative")
	case audit.MeanScore > -25:
		audit.Warnings = append(audit.Warnings, "random mean score is only mildly negative")
	}
	if audit.PositiveRate > 0.10 {
		audit.Warnings = append(audit.Warnings, "more than 10% of random legal runs finish positive")
	}
	if audit.P90Score > 0 {
		audit.Warnings = append(audit.Warnings, "p90 random legal run finishes above zero")
	}
	return audit
}

func randomActionForTick(tick TickDefinition, rng *rand.Rand) SimulatedAction {
	if len(tick.Opportunities) == 0 {
		return SimulatedAction{Command: "hold"}
	}

	opportunity := tick.Opportunities[rng.Intn(len(tick.Opportunities))]
	command := opportunity.AllowedCommands[rng.Intn(len(opportunity.AllowedCommands))]
	action := SimulatedAction{
		Command: command,
		Target:  opportunity.OpportunityID,
	}
	if len(opportunity.AllowedOptions) > 0 && command != "hold" {
		action.Option = opportunity.AllowedOptions[rng.Intn(len(opportunity.AllowedOptions))]
	}
	if opportunity.TextSlot && command != "hold" {
		action.Phrase = randomPhrase(rng)
	}
	return action
}

func randomPhrase(rng *rand.Rand) string {
	words := []string{"amber", "static", "mirror", "ledger", "signal", "dust", "chorus", "gamma"}
	return strings.Join([]string{
		words[rng.Intn(len(words))],
		words[rng.Intn(len(words))],
		words[rng.Intn(len(words))],
	}, " ")
}

func randomActionCountForTick(tick TickDefinition) int {
	if len(tick.Opportunities) == 0 {
		return 1
	}

	total := 0
	for _, opportunity := range tick.Opportunities {
		for _, command := range opportunity.AllowedCommands {
			if command == "hold" {
				total++
				continue
			}
			count := 1
			if len(opportunity.AllowedOptions) > 0 {
				count *= len(opportunity.AllowedOptions)
			}
			if opportunity.TextSlot {
				count *= randomPhraseVariantCount
			}
			total += count
		}
	}
	return total
}

func evaluateSimulatedAction(plan ScoringPlan, action SimulatedAction, state simulatedPlayerState) (Rule, bool) {
	for _, rule := range plan.Rules {
		if matchesSimulatedAction(rule.Match, action) && requirementsMet(rule.Requirements, state) {
			return rule, rule.Classification == "best"
		}
	}
	for _, rule := range plan.Rules {
		if rule.Match.Command == "hold" && requirementsMet(rule.Requirements, state) {
			return rule, rule.Classification == "best"
		}
	}
	return Rule{
		Match:          ActionMatch{Command: "hold"},
		Delta:          ScoreDelta{},
		Label:          "No eligible fallback rule matched.",
		Classification: "miss",
	}, false
}

func matchesSimulatedAction(match ActionMatch, action SimulatedAction) bool {
	if match.Command != "" && match.Command != action.Command {
		return false
	}
	if match.Target != "" && match.Target != action.Target {
		return false
	}
	if match.Option != "" && match.Option != action.Option {
		return false
	}
	if match.Phrase != "" && strings.TrimSpace(strings.ToLower(match.Phrase)) != strings.TrimSpace(strings.ToLower(action.Phrase)) {
		return false
	}
	return true
}

func applyLedgerDelta(ledger *SimulatedLedger, delta ScoreDelta) {
	ledger.Yield += delta.Yield
	ledger.Insight += delta.Insight
	ledger.Aura += delta.Aura
	ledger.Debt += delta.Debt
	ledger.MissPenalties += delta.MissPenalties
	ledger.Score = ledger.Yield + ledger.Insight + ledger.Aura - ledger.Debt - ledger.MissPenalties
}

func newSimulatedPlayerState() simulatedPlayerState {
	return simulatedPlayerState{
		Tags:       make(map[string]struct{}),
		Reputation: make(map[string]int64),
	}
}

func requirementsMet(requirements RuleRequirements, state simulatedPlayerState) bool {
	for _, tag := range requirements.RequiresAllTags {
		if _, ok := state.Tags[tag]; !ok {
			return false
		}
	}
	if len(requirements.RequiresAnyTags) > 0 {
		any := false
		for _, tag := range requirements.RequiresAnyTags {
			if _, ok := state.Tags[tag]; ok {
				any = true
				break
			}
		}
		if !any {
			return false
		}
	}
	for _, tag := range requirements.ForbidsTags {
		if _, ok := state.Tags[tag]; ok {
			return false
		}
	}
	if requirements.MaxDebt != 0 && state.Ledger.Debt > requirements.MaxDebt {
		return false
	}
	if requirements.MinAura != 0 && state.Ledger.Aura < requirements.MinAura {
		return false
	}
	for faction, minimum := range requirements.MinReputation {
		if state.Reputation[faction] < minimum {
			return false
		}
	}
	return true
}

func applyRuleToSimulatedState(state *simulatedPlayerState, rule Rule) {
	applyLedgerDelta(&state.Ledger, rule.Delta)
	for _, tag := range rule.Effects.AddTags {
		state.Tags[tag] = struct{}{}
	}
	for _, tag := range rule.Effects.RemoveTags {
		delete(state.Tags, tag)
	}
	if rule.Effects.AvailabilityDelta != "" {
		state.Availability = rule.Effects.AvailabilityDelta
	}
	state.Inventory += rule.Effects.InventoryDelta
	for faction, delta := range rule.Effects.ReputationDelta {
		state.Reputation[faction] += delta
	}
}

func quantileScore(sortedScores []int64, q float64) int64 {
	if len(sortedScores) == 0 {
		return 0
	}
	if q <= 0 {
		return sortedScores[0]
	}
	if q >= 1 {
		return sortedScores[len(sortedScores)-1]
	}
	index := int(float64(len(sortedScores)-1) * q)
	return sortedScores[index]
}
