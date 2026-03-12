package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"time"

	"github.com/mihai/mad/internal/season"
)

const (
	defaultRecentRevealWindow = 6
	defaultMaxNotesChars      = 1600
)

type MemoryMode string

const (
	MemoryModeInherit MemoryMode = "inherit"
	MemoryModeOn      MemoryMode = "on"
	MemoryModeOff     MemoryMode = "off"
)

type ContextMode string

const (
	ContextModePersistent ContextMode = "persistent"
	ContextModeEphemeral  ContextMode = "ephemeral"
)

type ServiceTier string

const (
	ServiceTierInherit ServiceTier = "inherit"
	ServiceTierFast    ServiceTier = "fast"
	ServiceTierFlex    ServiceTier = "flex"
)

type TextMode string

const (
	TextModeFull        TextMode = "full"
	TextModeSourceTypes TextMode = "source-types"
	TextModeRedacted    TextMode = "redacted"
)

type RunnerSpec struct {
	Provider    string      `json:"provider"`
	Model       string      `json:"model"`
	Effort      string      `json:"effort,omitempty"`
	MemoryMode  MemoryMode  `json:"memory_mode,omitempty"`
	ContextMode ContextMode `json:"context_mode,omitempty"`
	ServiceTier ServiceTier `json:"service_tier,omitempty"`
}

func (s RunnerSpec) Label() string {
	base := ""
	if s.Effort == "" {
		base = fmt.Sprintf("%s:%s", s.Provider, s.Model)
	} else {
		base = fmt.Sprintf("%s:%s@%s", s.Provider, s.Model, s.Effort)
	}
	if s.MemoryMode != "" && s.MemoryMode != MemoryModeInherit {
		base = fmt.Sprintf("%s+mem-%s", base, s.MemoryMode)
	}
	if s.ContextMode != "" && s.ContextMode != ContextModePersistent {
		base = fmt.Sprintf("%s+ctx-%s", base, s.ContextMode)
	}
	if s.ServiceTier != "" && s.ServiceTier != ServiceTierInherit {
		base = fmt.Sprintf("%s+tier-%s", base, s.ServiceTier)
	}
	return base
}

func ParseMemoryMode(raw string) (MemoryMode, error) {
	mode := MemoryMode(strings.ToLower(strings.TrimSpace(raw)))
	switch mode {
	case "", MemoryModeInherit:
		return MemoryModeInherit, nil
	case MemoryModeOn, MemoryModeOff:
		return mode, nil
	default:
		return "", fmt.Errorf("unsupported memory mode %q", raw)
	}
}

func ParseContextMode(raw string) (ContextMode, error) {
	mode := ContextMode(strings.ToLower(strings.TrimSpace(raw)))
	switch mode {
	case "", ContextModePersistent:
		return ContextModePersistent, nil
	case ContextModeEphemeral:
		return mode, nil
	default:
		return "", fmt.Errorf("unsupported context mode %q", raw)
	}
}

func ParseServiceTier(raw string) (ServiceTier, error) {
	tier := ServiceTier(strings.ToLower(strings.TrimSpace(raw)))
	switch tier {
	case "", ServiceTierInherit:
		return ServiceTierInherit, nil
	case ServiceTierFast, ServiceTierFlex:
		return tier, nil
	default:
		return "", fmt.Errorf("unsupported service tier %q", raw)
	}
}

func ParseTextMode(raw string) (TextMode, error) {
	mode := TextMode(strings.ToLower(strings.TrimSpace(raw)))
	switch mode {
	case "", TextModeFull:
		return TextModeFull, nil
	case TextModeSourceTypes, TextModeRedacted:
		return mode, nil
	default:
		return "", fmt.Errorf("unsupported text mode %q", raw)
	}
}

type Runner interface {
	Spec() RunnerSpec
	Decide(ctx context.Context, prompt string) ([]byte, error)
	Probe(ctx context.Context) error
	Close() error
	SessionInfo() SessionInfo
}

type RunOptions struct {
	StartTick         int                   `json:"start_tick"`
	MaxTicks          int                   `json:"max_ticks,omitempty"`
	RecentRevealCount int                   `json:"recent_reveal_count,omitempty"`
	MaxNotesChars     int                   `json:"max_notes_chars,omitempty"`
	DecisionTimeout   time.Duration         `json:"decision_timeout,omitempty"`
	TextMode          TextMode              `json:"text_mode,omitempty"`
	StepCallback      func(RunResult) error `json:"-"`
}

type PromptPacket struct {
	SeasonID      string                      `json:"season_id"`
	SeasonTitle   string                      `json:"season_title"`
	TickIndex     int                         `json:"tick_index"`
	TickCount     int                         `json:"tick_count"`
	Observations  []season.PublicTick         `json:"observations,omitempty"`
	CurrentTick   season.PublicTick           `json:"current_tick"`
	ActionChoices []PromptActionChoice        `json:"action_choices"`
	CurrentState  season.HarnessStateSnapshot `json:"current_state"`
	RecentReveals []season.SimulatedReveal    `json:"recent_reveals,omitempty"`
	Notes         string                      `json:"notes,omitempty"`
}

type PromptActionChoice struct {
	Label   string `json:"label,omitempty"`
	Index   int    `json:"index"`
	Summary string `json:"summary"`
	Command string `json:"command"`
	Target  string `json:"target,omitempty"`
	Option  string `json:"option,omitempty"`
}

type Decision struct {
	ActionIndex int    `json:"action_index"`
	Notes       string `json:"notes,omitempty"`
}

type ScorePoint struct {
	TickIndex int                    `json:"tick_index"`
	TickID    string                 `json:"tick_id"`
	Ledger    season.SimulatedLedger `json:"ledger"`
}

type SessionInfo struct {
	Workdir           string `json:"workdir,omitempty"`
	ProviderSessionID string `json:"provider_session_id,omitempty"`
	NativeHomeDir     string `json:"native_home_dir,omitempty"`
	NativeProjectDir  string `json:"native_project_dir,omitempty"`
	NativeSessionPath string `json:"native_session_path,omitempty"`
	NativeMemoryPath  string `json:"native_memory_path,omitempty"`
}

type StepTrace struct {
	TickIndex   int                    `json:"tick_index"`
	TickID      string                 `json:"tick_id"`
	Prompt      PromptPacket           `json:"prompt"`
	Decision    Decision               `json:"decision"`
	Outcome     season.HarnessOutcome  `json:"outcome"`
	Score       season.SimulatedLedger `json:"score"`
	NotesBefore string                 `json:"notes_before,omitempty"`
	NotesAfter  string                 `json:"notes_after,omitempty"`
	RawResponse string                 `json:"raw_response,omitempty"`
	DurationMS  int64                  `json:"duration_ms"`
	Error       string                 `json:"error,omitempty"`
}

type RunResult struct {
	Runner         RunnerSpec                  `json:"runner"`
	TextMode       TextMode                    `json:"text_mode,omitempty"`
	RunNumber      int                         `json:"run_number,omitempty"`
	RunCount       int                         `json:"run_count,omitempty"`
	Session        SessionInfo                 `json:"session,omitempty"`
	SeasonID       string                      `json:"season_id"`
	SeasonTitle    string                      `json:"season_title"`
	StartTick      int                         `json:"start_tick"`
	EndTick        int                         `json:"end_tick"`
	RequestedTicks int                         `json:"requested_ticks,omitempty"`
	StepCount      int                         `json:"step_count"`
	StartedAt      time.Time                   `json:"started_at"`
	CompletedAt    time.Time                   `json:"completed_at"`
	FinalState     season.HarnessStateSnapshot `json:"final_state"`
	FinalScore     season.SimulatedLedger      `json:"final_score"`
	Breakdown      season.ScoreBreakdown       `json:"breakdown,omitempty"`
	ScoreTrace     []ScorePoint                `json:"score_trace"`
	Steps          []StepTrace                 `json:"steps"`
	Errors         []string                    `json:"errors,omitempty"`
}

type ProbeResult struct {
	Runner      RunnerSpec  `json:"runner"`
	Session     SessionInfo `json:"session,omitempty"`
	StartedAt   time.Time   `json:"started_at"`
	CompletedAt time.Time   `json:"completed_at"`
	DurationMS  int64       `json:"duration_ms"`
	OK          bool        `json:"ok"`
	Error       string      `json:"error,omitempty"`
}

type SuiteReport struct {
	GeneratedAt time.Time     `json:"generated_at"`
	SeasonPath  string        `json:"season_path,omitempty"`
	SeasonID    string        `json:"season_id,omitempty"`
	SeasonTitle string        `json:"season_title,omitempty"`
	Runs        []RunResult   `json:"runs,omitempty"`
	RunGroups   []RunGroup    `json:"run_groups,omitempty"`
	Probes      []ProbeResult `json:"probes,omitempty"`
}

type RunGroup struct {
	Runner          RunnerSpec `json:"runner"`
	RunCount        int        `json:"run_count"`
	MeanScore       float64    `json:"mean_score"`
	MedianScore     float64    `json:"median_score"`
	P90Score        float64    `json:"p90_score"`
	MinScore        int64      `json:"min_score"`
	MaxScore        int64      `json:"max_score"`
	MeanWallMS      int64      `json:"mean_wall_ms"`
	MeanTicksPerMin float64    `json:"mean_ticks_per_min"`
}

func ParseRunnerSpec(raw string) (RunnerSpec, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return RunnerSpec{}, errors.New("runner spec is empty")
	}

	provider, rest, ok := strings.Cut(raw, ":")
	if !ok || strings.TrimSpace(rest) == "" {
		return RunnerSpec{}, fmt.Errorf("runner spec %q must be provider:model[@effort]", raw)
	}
	spec := RunnerSpec{
		Provider: strings.ToLower(strings.TrimSpace(provider)),
	}
	if spec.Provider != "codex" && spec.Provider != "claude" && spec.Provider != "openrouter" {
		return RunnerSpec{}, fmt.Errorf("unsupported provider %q", spec.Provider)
	}

	model, effort, _ := strings.Cut(rest, "@")
	spec.Model = strings.TrimSpace(model)
	spec.Effort = strings.TrimSpace(effort)
	if spec.Model == "" {
		return RunnerSpec{}, fmt.Errorf("runner spec %q has empty model", raw)
	}
	return spec, nil
}

func DefaultRunnerSpecs() []RunnerSpec {
	return []RunnerSpec{
		{Provider: "codex", Model: "gpt-5.1-codex-mini", Effort: "medium"},
		{Provider: "codex", Model: "gpt-5.2-codex", Effort: "high"},
		{Provider: "codex", Model: "gpt-5.4", Effort: "medium"},
		{Provider: "claude", Model: "haiku", Effort: "low"},
		{Provider: "claude", Model: "sonnet", Effort: "medium"},
		{Provider: "claude", Model: "opus", Effort: "high"},
	}
}

func RunnerWarnings(spec RunnerSpec) []string {
	var warnings []string
	if spec.Provider == "openrouter" {
		if spec.MemoryMode != MemoryModeOff {
			warnings = append(
				warnings,
				fmt.Sprintf(
					"%s: memory=%s has no provider-native effect for OpenRouter; the current harness behaves like memory-off for this provider",
					spec.Label(),
					firstNonEmptyString(string(spec.MemoryMode), string(MemoryModeInherit)),
				),
			)
		}
		if spec.ContextMode == ContextModePersistent {
			warnings = append(
				warnings,
				fmt.Sprintf(
					"%s: context=persistent only carries harness notes forward; there is no provider-native OpenRouter session continuity",
					spec.Label(),
				),
			)
		}
		if openRouterUsesLogprobChoice(spec.Model) && spec.ServiceTier == ServiceTierFast {
			warnings = append(
				warnings,
				fmt.Sprintf(
					"%s: service-tier=fast is ignored in OpenRouter logprob mode so the harness can stay on a provider path that returns top_logprobs",
					spec.Label(),
				),
			)
		}
	}
	return warnings
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func BuildPrompt(packet PromptPacket, maxNotesChars int, allowNotes bool, allowProviderMemory bool, style ActionLabelStyle) (string, error) {
	if maxNotesChars <= 0 {
		maxNotesChars = defaultMaxNotesChars
	}
	body, err := json.MarshalIndent(packet, "", "  ")
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.WriteString("You are playing Mutual Agent Destruction (MAD) offline.\n")
	builder.WriteString("Choose exactly one action from ActionChoices.\n")
	if style == ActionLabelLetters {
		builder.WriteString("Reply with only the action letter, for example: A\n")
	} else {
		builder.WriteString("Reply with only the action number, for example: 1\n")
	}
	builder.WriteString("Do not explain. Do not output JSON. Do not output markdown. Do not repeat the prompt.\n")
	builder.WriteString("Goal: maximize long-run score, not immediate reward.\n")
	builder.WriteString("If you are uncertain, choose 1 for hold.\n")
	builder.WriteString("Player-owned state is exact. Source regimes are public source-bias periods visible to everyone.\n")
	if allowProviderMemory {
		builder.WriteString("If your provider offers persistent memory, you may store short stable cross-tick facts there when they are likely to help future decisions.\n")
	}
	if allowNotes {
		builder.WriteString("You may optionally add a second line starting with 'Notes:' to store a short reminder.\n")
		builder.WriteString(fmt.Sprintf("If you add notes, keep them concise and durable; hard cap %d characters.\n", maxNotesChars))
	}
	builder.WriteString("Packet:\n")
	builder.Write(body)
	builder.WriteString("\n")
	return builder.String(), nil
}

func RunSeason(ctx context.Context, file season.File, report season.SimulationReport, runner Runner, options RunOptions) (RunResult, error) {
	options = normalizeRunOptions(options)
	startTick, endTick := clampTickRange(options, len(file.Ticks))

	result := RunResult{
		Runner:         runner.Spec(),
		TextMode:       options.TextMode,
		Session:        runner.SessionInfo(),
		SeasonID:       file.SeasonID,
		SeasonTitle:    file.Title,
		StartTick:      startTick,
		EndTick:        endTick,
		RequestedTicks: options.MaxTicks,
		StartedAt:      time.Now().UTC(),
	}

	state := season.NewHarnessState()
	breakdown := season.NewScoreBreakdownAccumulator()
	visibleReveals, revealsByStart := revealWindows(report, startTick, options.RecentRevealCount)
	notes := ""
	persistNotes := runner.Spec().ContextMode != ContextModeEphemeral
	actionStyle := actionLabelStyleForRunner(runner.Spec(), tickActionCount(file))

	var pendingObservations []season.PublicTick

	for tickIndex := startTick; tickIndex < endTick; tickIndex++ {
		for _, reveal := range revealsByStart[tickIndex] {
			visibleReveals = append(visibleReveals, reveal)
		}
		if len(visibleReveals) > options.RecentRevealCount {
			visibleReveals = visibleReveals[len(visibleReveals)-options.RecentRevealCount:]
		}

		state.AdvanceToTick(tickIndex)
		tick := file.Ticks[tickIndex]

		// Observe-only ticks (no opportunities): buffer prose for the next action tick.
		if len(tick.Opportunities) == 0 {
			pendingObservations = append(pendingObservations, applyTextModeTick(tick.Public(), options.TextMode))
			continue
		}

		packet := PromptPacket{
			SeasonID:      file.SeasonID,
			SeasonTitle:   file.Title,
			TickIndex:     tickIndex,
			TickCount:     len(file.Ticks),
			Observations:  pendingObservations,
			CurrentTick:   applyTextModeTick(tick.Public(), options.TextMode),
			ActionChoices: buildActionChoices(runner.Spec(), tick, actionStyle),
			CurrentState:  state.Snapshot(),
			RecentReveals: applyTextModeReveals(cloneReveals(visibleReveals), options.TextMode),
		}
		pendingObservations = nil
		if persistNotes {
			packet.Notes = notes
		}
		prompt, err := BuildPrompt(packet, options.MaxNotesChars, persistNotes, runner.Spec().MemoryMode == MemoryModeOn, actionStyle)
		if err != nil {
			return result, err
		}

		stepStarted := time.Now()
		raw, decision, decisionErr := runDecision(ctx, runner, prompt, tick, notes, options)
		if decisionErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", tick.TickID, decisionErr))
		}

		notesBefore := notes
		if persistNotes {
			if trimmed := strings.TrimSpace(decision.Notes); trimmed != "" {
				notes = clampText(trimmed, options.MaxNotesChars)
			}
		} else {
			notes = ""
		}

		outcome := state.ApplyAction(tick, resolveDecisionAction(decision, tick, runner.Spec()))
		breakdown.Add(tick, outcome.AppliedRule)

		step := StepTrace{
			TickIndex:   tickIndex,
			TickID:      tick.TickID,
			Prompt:      packet,
			Decision:    decision,
			Outcome:     outcome,
			Score:       outcome.State.Ledger,
			NotesBefore: notesBefore,
			NotesAfter:  notes,
			RawResponse: raw,
			DurationMS:  time.Since(stepStarted).Milliseconds(),
		}
		if decisionErr != nil {
			step.Error = decisionErr.Error()
		}
		result.Steps = append(result.Steps, step)
		result.ScoreTrace = append(result.ScoreTrace, ScorePoint{
			TickIndex: tickIndex,
			TickID:    tick.TickID,
			Ledger:    outcome.State.Ledger,
		})
		result.StepCount = len(result.Steps)
		result.Session = runner.SessionInfo()
		result.FinalState = state.Snapshot()
		result.FinalScore = result.FinalState.Ledger
		result.Breakdown = breakdown.Materialize()
		result.CompletedAt = time.Now().UTC()
		if options.StepCallback != nil {
			if err := options.StepCallback(result); err != nil {
				return result, err
			}
		}
	}

	result.StepCount = len(result.Steps)
	result.Session = runner.SessionInfo()
	result.FinalState = state.Snapshot()
	result.FinalScore = result.FinalState.Ledger
	result.Breakdown = breakdown.Materialize()
	result.CompletedAt = time.Now().UTC()
	return result, nil
}

func RunProbe(ctx context.Context, runner Runner) ProbeResult {
	started := time.Now()
	err := runner.Probe(ctx)
	result := ProbeResult{
		Runner:      runner.Spec(),
		Session:     runner.SessionInfo(),
		StartedAt:   started.UTC(),
		CompletedAt: time.Now().UTC(),
		DurationMS:  time.Since(started).Milliseconds(),
		OK:          err == nil,
	}
	if err != nil {
		result.Error = err.Error()
	}
	return result
}

func normalizeRunOptions(options RunOptions) RunOptions {
	if options.RecentRevealCount <= 0 {
		options.RecentRevealCount = defaultRecentRevealWindow
	}
	if options.MaxNotesChars <= 0 {
		options.MaxNotesChars = defaultMaxNotesChars
	}
	if options.TextMode == "" {
		options.TextMode = TextModeFull
	}
	return options
}

func clampTickRange(options RunOptions, tickCount int) (int, int) {
	start := options.StartTick
	if start < 0 {
		start = 0
	}
	if start > tickCount {
		start = tickCount
	}

	end := tickCount
	if options.MaxTicks > 0 && start+options.MaxTicks < end {
		end = start + options.MaxTicks
	}
	return start, end
}

func revealWindows(report season.SimulationReport, startTick int, recentRevealCount int) ([]season.SimulatedReveal, map[int][]season.SimulatedReveal) {
	initial := make([]season.SimulatedReveal, 0, recentRevealCount)
	byStart := make(map[int][]season.SimulatedReveal)
	for _, reveal := range report.Reveals {
		if reveal.PublishedAfterIndex < startTick {
			initial = append(initial, reveal)
			continue
		}
		visibleOnTick := reveal.PublishedAfterIndex + 1
		byStart[visibleOnTick] = append(byStart[visibleOnTick], reveal)
	}
	if len(initial) > recentRevealCount {
		initial = initial[len(initial)-recentRevealCount:]
	}
	return initial, byStart
}

func cloneReveals(input []season.SimulatedReveal) []season.SimulatedReveal {
	if len(input) == 0 {
		return nil
	}
	out := make([]season.SimulatedReveal, len(input))
	copy(out, input)
	return out
}

func applyTextModeTick(tick season.PublicTick, mode TextMode) season.PublicTick {
	if mode == TextModeFull {
		return tick
	}
	out := tick
	out.Sources = make([]season.Source, len(tick.Sources))
	for i, source := range tick.Sources {
		out.Sources[i] = source
		out.Sources[i].Text = transformTextByMode(source.SourceType, source.Text, mode)
	}
	return out
}

func applyTextModeReveals(reveals []season.SimulatedReveal, mode TextMode) []season.SimulatedReveal {
	if mode == TextModeFull || len(reveals) == 0 {
		return reveals
	}
	out := make([]season.SimulatedReveal, len(reveals))
	for i, reveal := range reveals {
		out[i] = reveal
		if reveal.ResolutionPreview == nil {
			continue
		}
		preview := *reveal.ResolutionPreview
		preview.PublicExplanation = transformTextByMode("reveal", preview.PublicExplanation, mode)
		out[i].ResolutionPreview = &preview
	}
	return out
}

func transformTextByMode(sourceType, text string, mode TextMode) string {
	switch mode {
	case TextModeSourceTypes:
		if sourceType == "" {
			return "[source text omitted; source type unavailable]"
		}
		return fmt.Sprintf("[%s text omitted]", sourceType)
	case TextModeRedacted:
		return "[text redacted]"
	default:
		return text
	}
}

func runDecision(ctx context.Context, runner Runner, prompt string, tick season.TickDefinition, priorNotes string, options RunOptions) (string, Decision, error) {
	runCtx := ctx
	cancel := func() {}
	if options.DecisionTimeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, options.DecisionTimeout)
	}
	defer cancel()

	raw, err := runner.Decide(runCtx, prompt)
	if err != nil {
		return string(bytes.TrimSpace(raw)), fallbackDecision(priorNotes, fmt.Sprintf("runner error: %v", err)), err
	}

	decision, err := decodeDecision(raw)
	if err != nil {
		return string(bytes.TrimSpace(raw)), fallbackDecision(priorNotes, fmt.Sprintf("decode error: %v", err)), err
	}
	decision = sanitizeDecision(decision, priorNotes)
	decision = validateDecisionAgainstTick(decision, tick, runner.Spec())
	return string(bytes.TrimSpace(raw)), decision, nil
}

func decodeDecision(raw []byte) (Decision, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return Decision{}, errors.New("empty response")
	}

	var direct Decision
	if err := json.Unmarshal(trimmed, &direct); err == nil && looksLikeDecision(direct) {
		return direct, nil
	}

	var directIndex int
	if err := json.Unmarshal(trimmed, &directIndex); err == nil {
		return Decision{ActionIndex: directIndex}, nil
	}

	var wrapped map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &wrapped); err == nil {
		indexKeys := []string{"action_index", "choice", "index", "action"}
		for _, key := range indexKeys {
			payload, ok := wrapped[key]
			if !ok {
				continue
			}
			if decision, err := decodeDecision(payload); err == nil {
				if notesRaw, ok := wrapped["notes"]; ok {
					var notes string
					if err := json.Unmarshal(notesRaw, &notes); err == nil {
						decision.Notes = strings.TrimSpace(notes)
					}
				}
				return decision, nil
			}
		}
		for _, key := range []string{"structured_output", "result", "response", "content", "output"} {
			payload, ok := wrapped[key]
			if !ok {
				continue
			}
			if decision, err := decodeDecision(payload); err == nil {
				return decision, nil
			}
			var content string
			if err := json.Unmarshal(payload, &content); err == nil {
				if decision, err := decodeDecision([]byte(content)); err == nil {
					return decision, nil
				}
			}
		}
	}

	var content string
	if err := json.Unmarshal(trimmed, &content); err == nil {
		return decodeDecision([]byte(content))
	}

	if decision, err := decodePlainDecision(string(trimmed)); err == nil {
		return decision, nil
	}

	return Decision{}, fmt.Errorf("could not decode decision from %q", string(trimmed))
}

func looksLikeDecision(decision Decision) bool {
	return decision.ActionIndex > 0 || strings.TrimSpace(decision.Notes) != ""
}

func fallbackDecision(priorNotes string, reason string) Decision {
	return Decision{
		ActionIndex: 1,
		Notes:       clampText(firstNonEmptyString(priorNotes, reason), defaultMaxNotesChars),
	}
}

func sanitizeDecision(decision Decision, priorNotes string) Decision {
	decision.Notes = strings.TrimSpace(decision.Notes)
	if decision.Notes == "" {
		decision.Notes = priorNotes
	}
	return decision
}

func validateDecisionAgainstTick(decision Decision, tick season.TickDefinition, spec RunnerSpec) Decision {
	if decision.ActionIndex <= 0 {
		return fallbackDecision(decision.Notes, "missing action index")
	}
	actions := actionsForRunnerTick(spec, tick)
	if decision.ActionIndex > len(actions) {
		return fallbackDecision(decision.Notes, "action index out of range")
	}
	return decision
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func clampText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return strings.TrimSpace(value[:limit])
}

type ActionLabelStyle string

const (
	ActionLabelNumbers ActionLabelStyle = "numbers"
	ActionLabelLetters ActionLabelStyle = "letters"
)

func actionLabelStyleForRunner(spec RunnerSpec, maxActions int) ActionLabelStyle {
	if openRouterUsesLogprobChoice(spec.Model) && maxActions <= 26 {
		return ActionLabelLetters
	}
	return ActionLabelNumbers
}

func tickActionCount(file season.File) int {
	maxActions := 1
	for _, tick := range file.Ticks {
		if n := len(season.EnumerateActions(tick)); n > maxActions {
			maxActions = n
		}
	}
	return maxActions
}

func buildActionChoices(spec RunnerSpec, tick season.TickDefinition, style ActionLabelStyle) []PromptActionChoice {
	actions := actionsForRunnerTick(spec, tick)
	opportunityByID := make(map[string]season.Opportunity, len(tick.Opportunities))
	for _, opportunity := range tick.Opportunities {
		opportunityByID[opportunity.OpportunityID] = opportunity
	}
	choices := make([]PromptActionChoice, 0, len(actions))
	for i, action := range actions {
		opportunity := opportunityByID[action.Target]
		choices = append(choices, PromptActionChoice{
			Label:   actionChoiceLabel(i, style),
			Index:   i + 1,
			Summary: actionSummary(action, opportunity),
			Command: action.Command,
			Target:  action.Target,
			Option:  action.Option,
		})
	}
	return choices
}

func actionsForRunnerTick(spec RunnerSpec, tick season.TickDefinition) []season.SimulatedAction {
	actions := season.EnumerateActions(tick)
	if !openRouterUsesLogprobChoice(spec.Model) || len(actions) <= 1 {
		return actions
	}
	out := make([]season.SimulatedAction, len(actions))
	copy(out, actions)
	permuteActionsDeterministically(out, spec.Model, tick.TickID)
	return out
}

func permuteActionsDeterministically(actions []season.SimulatedAction, model string, tickID string) {
	if len(actions) <= 1 {
		return
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(model))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(tickID))
	state := h.Sum64()
	for i := len(actions) - 1; i > 0; i-- {
		state ^= state << 13
		state ^= state >> 7
		state ^= state << 17
		j := int(state % uint64(i+1))
		actions[i], actions[j] = actions[j], actions[i]
	}
	if actions[0].Command == "hold" && len(actions) > 1 {
		state ^= state << 13
		state ^= state >> 7
		state ^= state << 17
		j := int(state%uint64(len(actions)-1)) + 1
		actions[0], actions[j] = actions[j], actions[0]
	}
}

func actionChoiceLabel(index int, style ActionLabelStyle) string {
	if style == ActionLabelLetters && index >= 0 && index < 26 {
		return string(rune('A' + index))
	}
	return strconv.Itoa(index + 1)
}

func actionSummary(action season.SimulatedAction, opportunity season.Opportunity) string {
	if action.Command == "hold" {
		return "hold"
	}
	summary := action.Command
	if action.Target != "" {
		summary += " " + action.Target
	}
	if action.Option != "" {
		summary += " [" + action.Option + "]"
	}
	if len(opportunity.PublicRequirements) > 0 {
		labels := make([]string, 0, len(opportunity.PublicRequirements))
		for _, requirement := range opportunity.PublicRequirements {
			if requirement.Label != "" {
				labels = append(labels, requirement.Label)
			}
		}
		if len(labels) > 0 {
			summary += " | req: " + strings.Join(labels, "; ")
		}
	}
	return summary
}

func resolveDecisionAction(decision Decision, tick season.TickDefinition, spec RunnerSpec) season.SimulatedAction {
	actions := actionsForRunnerTick(spec, tick)
	if decision.ActionIndex <= 0 || decision.ActionIndex > len(actions) {
		return season.SimulatedAction{Command: "hold"}
	}
	return actions[decision.ActionIndex-1]
}

func decodePlainDecision(raw string) (Decision, error) {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	selection := ""
	notes := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "notes:") {
			notes = strings.TrimSpace(line[len("notes:"):])
			continue
		}
		if selection == "" {
			selection = line
		}
	}
	if selection == "" {
		return Decision{}, errors.New("missing action selection")
	}
	index, err := parseActionIndex(selection)
	if err != nil {
		return Decision{}, err
	}
	return Decision{ActionIndex: index, Notes: notes}, nil
}

func parseActionIndex(selection string) (int, error) {
	trimmed := strings.TrimSpace(selection)
	trimmed = strings.Trim(trimmed, "[](){}.,:;`'\"")
	lower := strings.ToLower(trimmed)
	for _, prefix := range []string{"action:", "action", "choice:", "choice", "#"} {
		if strings.HasPrefix(lower, prefix) {
			trimmed = strings.TrimSpace(trimmed[len(prefix):])
			trimmed = strings.Trim(trimmed, "[](){}.,:;`'\"")
			lower = strings.ToLower(trimmed)
			break
		}
	}
	if trimmed == "" {
		return 0, errors.New("empty action selection")
	}
	if idx, err := strconv.Atoi(trimmed); err == nil && idx > 0 {
		return idx, nil
	}
	if len(trimmed) == 1 {
		ch := trimmed[0]
		if ch >= 'A' && ch <= 'Z' {
			return int(ch-'A') + 1, nil
		}
		if ch >= 'a' && ch <= 'z' {
			return int(ch-'a') + 1, nil
		}
	}
	fields := strings.Fields(trimmed)
	if len(fields) > 0 {
		if fields[0] == trimmed {
			return 0, fmt.Errorf("invalid action selection %q", selection)
		}
		return parseActionIndex(fields[0])
	}
	return 0, fmt.Errorf("invalid action selection %q", selection)
}
