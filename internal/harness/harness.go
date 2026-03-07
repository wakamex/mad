package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	StepCallback      func(RunResult) error `json:"-"`
}

type PromptPacket struct {
	SeasonID      string                      `json:"season_id"`
	SeasonTitle   string                      `json:"season_title"`
	TickIndex     int                         `json:"tick_index"`
	TickCount     int                         `json:"tick_count"`
	CurrentTick   season.PublicTick           `json:"current_tick"`
	CurrentState  season.HarnessStateSnapshot `json:"current_state"`
	RecentReveals []season.SimulatedReveal    `json:"recent_reveals,omitempty"`
	Notes         string                      `json:"notes,omitempty"`
}

type Decision struct {
	Command    string  `json:"command"`
	Target     string  `json:"target,omitempty"`
	Option     string  `json:"option,omitempty"`
	Phrase     string  `json:"phrase,omitempty"`
	Confidence float64 `json:"confidence"`
	Theory     string  `json:"theory"`
	Notes      string  `json:"notes"`
}

type ScorePoint struct {
	TickIndex int                    `json:"tick_index"`
	TickID    string                 `json:"tick_id"`
	Ledger    season.SimulatedLedger `json:"ledger"`
}

type SessionInfo struct {
	Workdir           string `json:"workdir,omitempty"`
	ProviderSessionID string `json:"provider_session_id,omitempty"`
	NativeProjectDir  string `json:"native_project_dir,omitempty"`
	NativeSessionPath string `json:"native_session_path,omitempty"`
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
	if spec.Provider != "codex" && spec.Provider != "claude" {
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

func BuildPrompt(packet PromptPacket, maxNotesChars int) (string, error) {
	if maxNotesChars <= 0 {
		maxNotesChars = defaultMaxNotesChars
	}
	body, err := json.MarshalIndent(packet, "", "  ")
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.WriteString("You are playing Mutual Agent Destruction (MAD) offline.\n")
	builder.WriteString("Return only JSON matching the provided schema.\n")
	builder.WriteString("Goal: maximize long-run score, not immediate reward.\n")
	builder.WriteString("If you are uncertain, choose hold.\n")
	builder.WriteString("Player-owned state is exact. Source regimes are public source-bias periods visible to everyone.\n")
	builder.WriteString("Use target = opportunity_id for non-hold actions. Only set option or phrase if the chosen opportunity allows them.\n")
	builder.WriteString("Always emit every schema field. Use empty strings for unused target, option, or phrase.\n")
	builder.WriteString(fmt.Sprintf("Keep notes concise and durable; hard cap %d characters.\n", maxNotesChars))
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
		Session:        runner.SessionInfo(),
		SeasonID:       file.SeasonID,
		SeasonTitle:    file.Title,
		StartTick:      startTick,
		EndTick:        endTick,
		RequestedTicks: options.MaxTicks,
		StartedAt:      time.Now().UTC(),
	}

	state := season.NewHarnessState()
	visibleReveals, revealsByStart := revealWindows(report, startTick, options.RecentRevealCount)
	notes := ""
	persistNotes := runner.Spec().ContextMode != ContextModeEphemeral

	for tickIndex := startTick; tickIndex < endTick; tickIndex++ {
		for _, reveal := range revealsByStart[tickIndex] {
			visibleReveals = append(visibleReveals, reveal)
		}
		if len(visibleReveals) > options.RecentRevealCount {
			visibleReveals = visibleReveals[len(visibleReveals)-options.RecentRevealCount:]
		}

		state.AdvanceToTick(tickIndex)
		tick := file.Ticks[tickIndex]
		packet := PromptPacket{
			SeasonID:      file.SeasonID,
			SeasonTitle:   file.Title,
			TickIndex:     tickIndex,
			TickCount:     len(file.Ticks),
			CurrentTick:   tick.Public(),
			CurrentState:  state.Snapshot(),
			RecentReveals: cloneReveals(visibleReveals),
		}
		if persistNotes {
			packet.Notes = notes
		}
		prompt, err := BuildPrompt(packet, options.MaxNotesChars)
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

		outcome := state.ApplyAction(tick, season.SimulatedAction{
			Command: decision.Command,
			Target:  decision.Target,
			Option:  decision.Option,
			Phrase:  decision.Phrase,
		})

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
	decision = validateDecisionAgainstTick(decision, tick)
	if decision.Command == "" {
		decision.Command = "hold"
	}
	if decision.Command == "hold" {
		decision.Target = ""
		decision.Option = ""
		decision.Phrase = ""
	}
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

	var wrapped map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &wrapped); err == nil {
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

	return Decision{}, fmt.Errorf("could not decode decision from %q", string(trimmed))
}

func looksLikeDecision(decision Decision) bool {
	return strings.TrimSpace(decision.Command) != "" ||
		strings.TrimSpace(decision.Theory) != "" ||
		strings.TrimSpace(decision.Notes) != ""
}

func fallbackDecision(priorNotes string, reason string) Decision {
	return Decision{
		Command:    "hold",
		Confidence: 0,
		Theory:     clampText(reason, 240),
		Notes:      priorNotes,
	}
}

func sanitizeDecision(decision Decision, priorNotes string) Decision {
	decision.Command = strings.TrimSpace(strings.ToLower(decision.Command))
	decision.Target = strings.TrimSpace(decision.Target)
	decision.Option = strings.TrimSpace(decision.Option)
	decision.Phrase = strings.TrimSpace(decision.Phrase)
	decision.Theory = strings.TrimSpace(decision.Theory)
	decision.Notes = strings.TrimSpace(decision.Notes)
	if decision.Notes == "" {
		decision.Notes = priorNotes
	}
	if decision.Confidence < 0 {
		decision.Confidence = 0
	}
	if decision.Confidence > 1 {
		decision.Confidence = 1
	}
	return decision
}

func validateDecisionAgainstTick(decision Decision, tick season.TickDefinition) Decision {
	if decision.Command == "hold" {
		return decision
	}
	for _, opportunity := range tick.Opportunities {
		if decision.Target != opportunity.OpportunityID {
			continue
		}
		if !containsString(opportunity.AllowedCommands, decision.Command) {
			return fallbackDecision(decision.Notes, "invalid command for target")
		}
		if len(opportunity.AllowedOptions) > 0 && decision.Option != "" && !containsString(opportunity.AllowedOptions, decision.Option) {
			return fallbackDecision(decision.Notes, "invalid option for target")
		}
		if !opportunity.TextSlot {
			decision.Phrase = ""
		}
		return decision
	}
	return fallbackDecision(decision.Notes, "unknown target for current tick")
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
