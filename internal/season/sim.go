package season

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"slices"
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
	Breakdown  ScoreBreakdown  `json:"breakdown,omitempty"`
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
	P99Run          *SimulatedRandomRun `json:"p99_run,omitempty"`
	MinScore        int64    `json:"min_score"`
	MaxScore        int64    `json:"max_score"`
	PositiveRate    float64  `json:"positive_rate"`
	NonNegativeRate float64  `json:"non_negative_rate"`
	BeatBestRate    float64  `json:"beat_best_rate"`
	Warnings        []string `json:"warnings,omitempty"`
}

type SimulatedRandomRun struct {
	RunIndex   int            `json:"run_index"`
	Score      int64          `json:"score"`
	Breakdown  ScoreBreakdown `json:"breakdown,omitempty"`
	BeatBest   int            `json:"beat_best"`
	TickCount  int            `json:"tick_count"`
}

type SimulatedActionSurface struct {
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
}

type SimulatedBadAction struct {
	Command string `json:"command"`
	Option  string `json:"option,omitempty"`
	Outcome string `json:"outcome"`
}

type simulatedPlayerState struct {
	CurrentTick             int
	Ledger                  SimulatedLedger
	Tags                    map[string]struct{}
	Reputation              map[string]int64
	Availability            string
	AvailabilityResetTick   int
	AvailabilityBeforeLock  string
	CooldownReadyTickByName map[string]int
}

const defaultAvailability = "available"

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
			"`visible_greedy` is a constrained non-LLM baseline that only uses the public action surface, clock class, public requirements, and exact player state. It does not parse source text or inspect hidden scoring labels.",
		},
		Baselines: map[string]SimulatedBaseline{
			"greedy_best":   {},
			"always_hold":   {},
			"visible_greedy": {},
		},
		ActionSurface: SimulatedActionSurface{
			Distribution:  make(map[string]int),
			PerTickCounts: make(map[string]int, len(file.Ticks)),
		},
		Ticks: make([]SimulatedTick, 0, len(file.Ticks)),
	}

	greedyState := newSimulatedPlayerState()
	holdState := newSimulatedPlayerState()
	visibleState := newSimulatedPlayerState()
	greedyBreakdown := NewScoreBreakdownAccumulator()
	holdBreakdown := NewScoreBreakdownAccumulator()
	visibleBreakdown := NewScoreBreakdownAccumulator()
	var nowMS int64
	for i, tick := range file.Ticks {
		advanceSimulatedStateToTick(&greedyState, i)
		advanceSimulatedStateToTick(&holdState, i)
		advanceSimulatedStateToTick(&visibleState, i)
		resolution := simulateResolution(tick, greedyState)

		greedyRule := chooseGreedyRule(tick, greedyState)
		bestBaseline := report.Baselines["greedy_best"]
		advanceBaseline(&bestBaseline, &greedyState, greedyRule)
		report.Baselines["greedy_best"] = bestBaseline
		greedyBreakdown.Add(tick, greedyRule)

		holdRule := chooseHoldRule(tick, holdState)
		holdBaseline := report.Baselines["always_hold"]
		advanceBaseline(&holdBaseline, &holdState, holdRule)
		report.Baselines["always_hold"] = holdBaseline
		holdBreakdown.Add(tick, holdRule)

		visibleAction := chooseVisibleGreedyAction(tick, visibleState)
		visibleRule, _ := evaluateSimulatedAction(tick.Scoring, visibleAction, visibleState)
		visibleBaseline := report.Baselines["visible_greedy"]
		advanceBaseline(&visibleBaseline, &visibleState, visibleRule)
		report.Baselines["visible_greedy"] = visibleBaseline
		visibleBreakdown.Add(tick, visibleRule)

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
	bestBaseline := report.Baselines["greedy_best"]
	bestBaseline.Breakdown = greedyBreakdown.Materialize()
	report.Baselines["greedy_best"] = bestBaseline
	holdBaseline := report.Baselines["always_hold"]
	holdBaseline.Breakdown = holdBreakdown.Materialize()
	report.Baselines["always_hold"] = holdBaseline
	visibleBaseline := report.Baselines["visible_greedy"]
	visibleBaseline.Breakdown = visibleBreakdown.Materialize()
	report.Baselines["visible_greedy"] = visibleBaseline
	if options.RandomRuns > 0 {
		report.RandomAudit = simulateRandomAudit(file, options)
	}
	return report, nil
}

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

func chooseVisibleGreedyAction(tick TickDefinition, state simulatedPlayerState) SimulatedAction {
	bestAction := SimulatedAction{Command: "hold"}
	bestScore := visibleActionScore(tick.ClockClass, Opportunity{}, bestAction, state)
	bestTie := visibleActionTieBreak(bestAction)

	for _, opportunity := range tick.Opportunities {
		for _, command := range opportunity.AllowedCommands {
			if command == "hold" {
				continue
			}
			if len(opportunity.AllowedOptions) == 0 {
				action := SimulatedAction{Command: command, Target: opportunity.OpportunityID}
				score := visibleActionScore(tick.ClockClass, opportunity, action, state)
				tie := visibleActionTieBreak(action)
				if score > bestScore || (score == bestScore && tie < bestTie) {
					bestAction, bestScore, bestTie = action, score, tie
				}
				continue
			}
			for _, option := range opportunity.AllowedOptions {
				action := SimulatedAction{Command: command, Target: opportunity.OpportunityID, Option: option}
				score := visibleActionScore(tick.ClockClass, opportunity, action, state)
				tie := visibleActionTieBreak(action)
				if score > bestScore || (score == bestScore && tie < bestTie) {
					bestAction, bestScore, bestTie = action, score, tie
				}
			}
		}
	}
	return bestAction
}

func visibleActionScore(clockClass string, opportunity Opportunity, action SimulatedAction, state simulatedPlayerState) int {
	if action.Command == "hold" {
		score := 0
		if clockClass == "interrupt" {
			score -= 12
		}
		return score
	}

	score := 0
	switch action.Command {
	case "inspect":
		score = 12
	case "commit":
		score = 8
	default:
		score = 6
	}

	if clockClass == "interrupt" {
		if action.Command == "inspect" {
			score -= 8
		} else {
			score += 10
		}
	}
	if clockClass == "dossier" && action.Command == "inspect" {
		score += 4
	}

	if state.Availability != defaultAvailability && action.Command != "inspect" {
		score -= 10
	}

	satisfied, unmet := visibleRequirementStatus(opportunity.PublicRequirements, state)
	if len(opportunity.PublicRequirements) > 0 {
		score += satisfied * 4
		score -= unmet * 8
		if action.Command == "inspect" && unmet > 0 {
			score += 4
		}
	}

	optionCount := len(opportunity.AllowedOptions)
	if optionCount == 1 && action.Command != "inspect" {
		score += 5
	}
	if optionCount > 1 && action.Command != "inspect" {
		score -= 7
	}

	if state.Ledger.Debt >= 80 && action.Command == "commit" {
		score -= 2
	}

	return score
}

func visibleRequirementStatus(requirements []PublicRequirement, state simulatedPlayerState) (satisfied int, unmet int) {
	for _, requirement := range requirements {
		if publicRequirementSatisfied(requirement, state) {
			satisfied++
		} else {
			unmet++
		}
	}
	return satisfied, unmet
}

func publicRequirementSatisfied(requirement PublicRequirement, state simulatedPlayerState) bool {
	var current int64
	switch requirement.Metric {
	case "reputation":
		current = state.Reputation[requirement.Scope]
	case "debt":
		current = state.Ledger.Debt
	case "aura":
		current = state.Ledger.Aura
	default:
		return false
	}
	switch requirement.Operator {
	case ">=":
		return current >= requirement.Value
	case "<=":
		return current <= requirement.Value
	case ">":
		return current > requirement.Value
	case "<":
		return current < requirement.Value
	case "==":
		return current == requirement.Value
	default:
		return false
	}
}

func visibleActionTieBreak(action SimulatedAction) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(action.Command))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(action.Target))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(action.Option))
	return h.Sum64()
}

func advanceBaseline(baseline *SimulatedBaseline, state *simulatedPlayerState, rule Rule) {
	applyRuleToSimulatedState(state, rule)
	baseline.Ledger = state.Ledger
	baseline.ScoreTrace = append(baseline.ScoreTrace, baseline.Ledger.Score)
}

func simulateRandomAudit(file File, options SimulationOptions) *SimulatedRandomAudit {
	rng := rand.New(rand.NewSource(options.RandomSeed))
	scores := make([]int64, 0, options.RandomRuns)
	type randomRunScore struct {
		Score    int64
		RunIndex int
	}
	runScores := make([]randomRunScore, 0, options.RandomRuns)
	positive := 0
	nonNegative := 0
	totalBeatBest := 0
	totalChoices := 0

	for i := 0; i < options.RandomRuns; i++ {
		score, beatBestHits, _ := simulateRandomRun(file, rng, false)
		scores = append(scores, score)
		runScores = append(runScores, randomRunScore{Score: score, RunIndex: i})
		totalBeatBest += beatBestHits
		totalChoices += len(file.Ticks)
		if score > 0 {
			positive++
		}
		if score >= 0 {
			nonNegative++
		}
	}
	slices.Sort(scores)
	slices.SortStableFunc(runScores, func(a, b randomRunScore) int {
		switch {
		case a.Score < b.Score:
			return -1
		case a.Score > b.Score:
			return 1
		case a.RunIndex < b.RunIndex:
			return -1
		case a.RunIndex > b.RunIndex:
			return 1
		default:
			return 0
		}
	})

	mean := 0.0
	for _, score := range scores {
		mean += float64(score)
	}
	mean /= float64(len(scores))

	p99Index := quantileIndex(len(runScores), 0.99)
	p99RunIndex := runScores[p99Index].RunIndex
	p99SeedRNG := rand.New(rand.NewSource(options.RandomSeed))
	var p99Run *SimulatedRandomRun
	for i := 0; i <= p99RunIndex; i++ {
		score, beatBestHits, breakdown := simulateRandomRun(file, p99SeedRNG, i == p99RunIndex)
		if i == p99RunIndex {
			p99Run = &SimulatedRandomRun{
				RunIndex:  i,
				Score:     score,
				Breakdown: breakdown,
				BeatBest:  beatBestHits,
				TickCount: len(file.Ticks),
			}
			break
		}
	}

	audit := &SimulatedRandomAudit{
		Runs:            options.RandomRuns,
		Seed:            options.RandomSeed,
		MeanScore:       mean,
		MedianScore:     quantileScore(scores, 0.50),
		P90Score:        quantileScore(scores, 0.90),
		P99Score:        quantileScore(scores, 0.99),
		P99Run:          p99Run,
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

func simulateRandomRun(file File, rng *rand.Rand, captureBreakdown bool) (int64, int, ScoreBreakdown) {
	state := newSimulatedPlayerState()
	beatBestHits := 0
	var breakdown ScoreBreakdown
	var accumulator ScoreBreakdownAccumulator
	if captureBreakdown {
		accumulator = NewScoreBreakdownAccumulator()
	}
	for tickIndex, tick := range file.Ticks {
		advanceSimulatedStateToTick(&state, tickIndex)
		action := randomActionForTick(tick, rng)
		rule, isBest := evaluateSimulatedAction(tick.Scoring, action, state)
		applyRuleToSimulatedState(&state, rule)
		if captureBreakdown {
			accumulator.Add(tick, rule)
		}
		if isBest {
			beatBestHits++
		}
	}
	if captureBreakdown {
		breakdown = accumulator.Materialize()
	}
	return state.Ledger.Score, beatBestHits, breakdown
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
	return action
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
		Tags:                    make(map[string]struct{}),
		Reputation:              make(map[string]int64),
		Availability:            defaultAvailability,
		CooldownReadyTickByName: make(map[string]int),
	}
}

func advanceSimulatedStateToTick(state *simulatedPlayerState, tickIndex int) {
	state.CurrentTick = tickIndex
	if state.AvailabilityResetTick > 0 && tickIndex >= state.AvailabilityResetTick {
		if state.AvailabilityBeforeLock != "" {
			state.Availability = state.AvailabilityBeforeLock
		} else {
			state.Availability = defaultAvailability
		}
		state.AvailabilityBeforeLock = ""
		state.AvailabilityResetTick = 0
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
	if len(requirements.RequiresAvailability) > 0 && !contains(requirements.RequiresAvailability, state.Availability) {
		return false
	}
	if len(requirements.ForbidsAvailability) > 0 && contains(requirements.ForbidsAvailability, state.Availability) {
		return false
	}
	for _, cooldownName := range requirements.RequiresCooldownReady {
		if readyTick, ok := state.CooldownReadyTickByName[cooldownName]; ok && state.CurrentTick < readyTick {
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
	if rule.Effects.LockTicks > 0 {
		state.AvailabilityBeforeLock = state.Availability
		if rule.Effects.AvailabilityDelta != "" {
			state.Availability = rule.Effects.AvailabilityDelta
		} else {
			state.Availability = "locked"
		}
		state.AvailabilityResetTick = state.CurrentTick + rule.Effects.LockTicks + 1
	} else if rule.Effects.AvailabilityDelta != "" {
		state.Availability = rule.Effects.AvailabilityDelta
	}
	for cooldownName, cooldownTicks := range rule.Effects.SetCooldowns {
		if cooldownTicks <= 0 {
			delete(state.CooldownReadyTickByName, cooldownName)
			continue
		}
		state.CooldownReadyTickByName[cooldownName] = state.CurrentTick + cooldownTicks + 1
	}
	for faction, delta := range rule.Effects.ReputationDelta {
		state.Reputation[faction] += delta
	}
}

func quantileScore(sortedScores []int64, q float64) int64 {
	if len(sortedScores) == 0 {
		return 0
	}
	index := quantileIndex(len(sortedScores), q)
	return sortedScores[index]
}

func quantileIndex(length int, q float64) int {
	if length <= 0 {
		return 0
	}
	if q <= 0 {
		return 0
	}
	if q >= 1 {
		return length - 1
	}
	return int(float64(length-1) * q)
}
