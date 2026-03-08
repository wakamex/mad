package harness

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mihai/mad/internal/season"
)

type fakeRunner struct {
	spec      RunnerSpec
	session   SessionInfo
	responses [][]byte
	index     int
}

func (r *fakeRunner) Spec() RunnerSpec { return r.spec }

func (r *fakeRunner) Decide(_ context.Context, _ string) ([]byte, error) {
	if r.index >= len(r.responses) {
		return []byte("1"), nil
	}
	response := r.responses[r.index]
	r.index++
	return response, nil
}

func (r *fakeRunner) Probe(_ context.Context) error { return nil }

func (r *fakeRunner) Close() error { return nil }

func (r *fakeRunner) SessionInfo() SessionInfo { return r.session }

func TestParseRunnerSpec(t *testing.T) {
	spec, err := ParseRunnerSpec("codex:gpt-5.2-codex@high")
	if err != nil {
		t.Fatalf("parse spec: %v", err)
	}
	if spec.Provider != "codex" || spec.Model != "gpt-5.2-codex" || spec.Effort != "high" {
		t.Fatalf("unexpected spec: %#v", spec)
	}
}

func TestDecodeDecisionSupportsWrappedJSON(t *testing.T) {
	raw := []byte(`{"result":"{\"action_index\":4,\"notes\":\"remember broker\"}"}`)
	decision, err := decodeDecision(raw)
	if err != nil {
		t.Fatalf("decode decision: %v", err)
	}
	if decision.ActionIndex != 4 || decision.Notes != "remember broker" {
		t.Fatalf("unexpected decision: %#v", decision)
	}
}

func TestDecodeDecisionSupportsPlainNumberAndNotes(t *testing.T) {
	raw := []byte("3\nNotes: watch the choir debt cap")
	decision, err := decodeDecision(raw)
	if err != nil {
		t.Fatalf("decode decision: %v", err)
	}
	if decision.ActionIndex != 3 || decision.Notes != "watch the choir debt cap" {
		t.Fatalf("unexpected decision: %#v", decision)
	}
}

func TestDecodeDecisionSupportsLetterChoice(t *testing.T) {
	raw := []byte("B")
	decision, err := decodeDecision(raw)
	if err != nil {
		t.Fatalf("decode decision: %v", err)
	}
	if decision.ActionIndex != 2 {
		t.Fatalf("unexpected decision: %#v", decision)
	}
}

func TestDecodeDecisionSupportsNumberWithColon(t *testing.T) {
	raw := []byte("2:")
	decision, err := decodeDecision(raw)
	if err != nil {
		t.Fatalf("decode decision: %v", err)
	}
	if decision.ActionIndex != 2 {
		t.Fatalf("unexpected decision: %#v", decision)
	}
}

func TestDecodeDecisionSupportsPrefixedNumber(t *testing.T) {
	raw := []byte("choice 2")
	decision, err := decodeDecision(raw)
	if err != nil {
		t.Fatalf("decode decision: %v", err)
	}
	if decision.ActionIndex != 2 {
		t.Fatalf("unexpected decision: %#v", decision)
	}
}

func TestDecodeDecisionSupportsLetterWithColon(t *testing.T) {
	raw := []byte("A:")
	decision, err := decodeDecision(raw)
	if err != nil {
		t.Fatalf("decode decision: %v", err)
	}
	if decision.ActionIndex != 1 {
		t.Fatalf("unexpected decision: %#v", decision)
	}
}

func TestBuildPromptOmitsNotesInstructionsWhenDisabled(t *testing.T) {
	packet := PromptPacket{}
	prompt, err := BuildPrompt(packet, 100, false, false, ActionLabelNumbers)
	if err != nil {
		t.Fatalf("build prompt: %v", err)
	}
	if strings.Contains(prompt, "Notes:") {
		t.Fatalf("expected prompt to omit notes instructions, got %q", prompt)
	}
	if !strings.Contains(prompt, "Reply with only the action number") {
		t.Fatalf("expected strict action-number instruction, got %q", prompt)
	}
}

func TestBuildPromptIncludesNotesInstructionsWhenEnabled(t *testing.T) {
	packet := PromptPacket{}
	prompt, err := BuildPrompt(packet, 100, true, false, ActionLabelNumbers)
	if err != nil {
		t.Fatalf("build prompt: %v", err)
	}
	if !strings.Contains(prompt, "Notes:") {
		t.Fatalf("expected prompt to include notes instructions, got %q", prompt)
	}
}

func TestBuildPromptIncludesProviderMemoryHintWhenEnabled(t *testing.T) {
	packet := PromptPacket{}
	prompt, err := BuildPrompt(packet, 100, false, true, ActionLabelNumbers)
	if err != nil {
		t.Fatalf("build prompt: %v", err)
	}
	if !strings.Contains(prompt, "provider offers persistent memory") {
		t.Fatalf("expected prompt to include provider memory hint, got %q", prompt)
	}
}

func TestBuildPromptUsesLetterInstructionWhenRequested(t *testing.T) {
	packet := PromptPacket{}
	prompt, err := BuildPrompt(packet, 100, false, false, ActionLabelLetters)
	if err != nil {
		t.Fatalf("build prompt: %v", err)
	}
	if !strings.Contains(prompt, "Reply with only the action letter") {
		t.Fatalf("expected prompt to require action letter, got %q", prompt)
	}
}

func TestApplyTextModeTick(t *testing.T) {
	tick := season.PublicTick{
		Sources: []season.Source{
			{SourceType: "critical_broadcast", Text: "hazard text"},
			{SourceType: "market_gossip", Text: "market text"},
		},
	}
	sourceTypes := applyTextModeTick(tick, TextModeSourceTypes)
	if got := sourceTypes.Sources[0].Text; got != "[critical_broadcast text omitted]" {
		t.Fatalf("unexpected source-types text: %q", got)
	}
	redacted := applyTextModeTick(tick, TextModeRedacted)
	if got := redacted.Sources[1].Text; got != "[text redacted]" {
		t.Fatalf("unexpected redacted text: %q", got)
	}
}

func TestApplyTextModeReveals(t *testing.T) {
	reveals := []season.SimulatedReveal{
		{
			TickID: "S1-T0001",
			ResolutionPreview: &season.SimulatedResolution{
				PublicExplanation: "the dossier confirmed the broker route",
			},
		},
	}
	sourceTypes := applyTextModeReveals(reveals, TextModeSourceTypes)
	if got := sourceTypes[0].ResolutionPreview.PublicExplanation; got != "[reveal text omitted]" {
		t.Fatalf("unexpected source-types reveal text: %q", got)
	}
	redacted := applyTextModeReveals(reveals, TextModeRedacted)
	if got := redacted[0].ResolutionPreview.PublicExplanation; got != "[text redacted]" {
		t.Fatalf("unexpected redacted reveal text: %q", got)
	}
}

func TestBuildActionChoicesPermutesOpenRouterLogprobOrder(t *testing.T) {
	tick := season.TickDefinition{
		TickID: "S1-T0001",
		Opportunities: []season.Opportunity{
			{
				OpportunityID:   "opp.one",
				AllowedCommands: []string{"commit"},
				AllowedOptions:  []string{"alpha"},
			},
		},
	}
	choices := buildActionChoices(
		RunnerSpec{Provider: "openrouter", Model: "openai/gpt-4o-mini"},
		tick,
		ActionLabelLetters,
	)
	if len(choices) != 2 {
		t.Fatalf("expected 2 choices, got %d", len(choices))
	}
	if choices[0].Summary == "hold" {
		t.Fatalf("expected first choice to be permuted away from hold: %#v", choices)
	}
	if choices[0].Label != "A" || choices[1].Label != "B" {
		t.Fatalf("expected letter labels to remain positional: %#v", choices)
	}
}

func TestRunSeasonCapturesScoreTrace(t *testing.T) {
	file, err := season.LoadFile(filepath.Join("..", "..", "seasons", "dev", "season.json"))
	if err != nil {
		t.Fatalf("load season: %v", err)
	}
	report, err := season.Simulate(file)
	if err != nil {
		t.Fatalf("simulate season: %v", err)
	}

	runner := &fakeRunner{
		spec: RunnerSpec{Provider: "fake", Model: "stub"},
		session: SessionInfo{
			Workdir:           "/code/mad",
			ProviderSessionID: "fake-session",
			NativeSessionPath: "/tmp/fake-session.jsonl",
		},
		responses: [][]byte{
			[]byte("2\nNotes: accepted choir offer"),
			[]byte("1\nNotes: accepted choir offer"),
		},
	}

	result, err := RunSeason(context.Background(), file, report, runner, RunOptions{MaxTicks: 2})
	if err != nil {
		t.Fatalf("run season: %v", err)
	}
	if result.StepCount != 2 {
		t.Fatalf("unexpected step count: got %d want 2", result.StepCount)
	}
	if len(result.ScoreTrace) != 2 {
		t.Fatalf("unexpected score trace length: got %d want 2", len(result.ScoreTrace))
	}
	if result.ScoreTrace[0].TickID == "" || result.ScoreTrace[1].TickID == "" {
		t.Fatalf("expected tick ids in score trace")
	}
	if result.Steps[0].Outcome.ScoreAfter != result.ScoreTrace[0].Ledger.Score {
		t.Fatalf("score trace and outcome diverged on first step")
	}
	if result.Steps[1].NotesAfter != "accepted choir offer" {
		t.Fatalf("expected notes to persist across steps, got %q", result.Steps[1].NotesAfter)
	}
	if result.Session.ProviderSessionID != "fake-session" {
		t.Fatalf("expected session info to propagate, got %#v", result.Session)
	}
	if len(result.Breakdown.ByFamily) == 0 {
		t.Fatalf("expected run breakdown by family")
	}
}

func TestRunSeasonEphemeralContextDoesNotPersistNotes(t *testing.T) {
	file, err := season.LoadFile(filepath.Join("..", "..", "seasons", "dev", "season.json"))
	if err != nil {
		t.Fatalf("load season: %v", err)
	}
	report, err := season.Simulate(file)
	if err != nil {
		t.Fatalf("simulate season: %v", err)
	}

	runner := &fakeRunner{
		spec: RunnerSpec{Provider: "fake", Model: "stub", ContextMode: ContextModeEphemeral},
		responses: [][]byte{
			[]byte("2\nNotes: remember this"),
			[]byte("1\nNotes: still remembering"),
		},
	}

	result, err := RunSeason(context.Background(), file, report, runner, RunOptions{MaxTicks: 2})
	if err != nil {
		t.Fatalf("run season: %v", err)
	}
	if got := result.Steps[0].NotesAfter; got != "" {
		t.Fatalf("expected ephemeral run to clear notes after tick, got %q", got)
	}
	if got := result.Steps[1].Prompt.Notes; got != "" {
		t.Fatalf("expected next prompt notes to be empty, got %q", got)
	}
}
