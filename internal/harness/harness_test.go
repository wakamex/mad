package harness

import (
	"context"
	"path/filepath"
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
		return []byte(`{"command":"hold","confidence":0,"theory":"fallback","notes":""}`), nil
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
	raw := []byte(`{"result":"{\"command\":\"commit\",\"target\":\"broker.1\",\"option\":\"take\",\"confidence\":0.7,\"theory\":\"try it\",\"notes\":\"remember broker\"}"}`)
	decision, err := decodeDecision(raw)
	if err != nil {
		t.Fatalf("decode decision: %v", err)
	}
	if decision.Command != "commit" || decision.Target != "broker.1" || decision.Option != "take" {
		t.Fatalf("unexpected decision: %#v", decision)
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
			[]byte(`{"command":"commit","target":"choir.offer","option":"accept","confidence":0.8,"theory":"start reputation path","notes":"accepted choir offer"}`),
			[]byte(`{"command":"hold","confidence":0.2,"theory":"skip","notes":"accepted choir offer"}`),
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
			[]byte(`{"command":"commit","target":"choir.offer","option":"accept","confidence":0.8,"theory":"start reputation path","notes":"remember this"}`),
			[]byte(`{"command":"hold","confidence":0.2,"theory":"skip","notes":"still remembering"}`),
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
