package game

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/mihai/mad/internal/season"
	"github.com/mihai/mad/internal/storage"
)

func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	loadedSeason, err := season.LoadFile(filepath.Join("..", "..", "seasons", "dev", "season.json"))
	if err != nil {
		t.Fatalf("load season: %v", err)
	}

	tmpDir := t.TempDir()
	wal, err := storage.NewWAL(filepath.Join(tmpDir, "actions.log"))
	if err != nil {
		t.Fatalf("new wal: %v", err)
	}
	t.Cleanup(func() { _ = wal.Close() })
	return NewEngine(loadedSeason, wal, 8)
}

// firstCommitOpportunity finds the first tick with a commit-able opportunity
// that has a positive net score (skips standing_work_loop ticks that score ~0).
func firstCommitOpportunity(s season.File) (tickIndex int, opp season.Opportunity, option string, ok bool) {
	for i, tick := range s.Ticks {
		for _, o := range tick.Opportunities {
			hasCommit := false
			for _, cmd := range o.AllowedCommands {
				if cmd == "commit" {
					hasCommit = true
					break
				}
			}
			if !hasCommit || len(o.AllowedOptions) == 0 {
				continue
			}
			// Check that at least one commit rule yields positive net score.
			for _, rule := range tick.Scoring.Rules {
				if rule.Match.Command == "commit" && rule.Match.Target == o.OpportunityID && rule.Classification == "best" {
					net := rule.Delta.Yield + rule.Delta.Insight + rule.Delta.Aura - rule.Delta.Debt - rule.Delta.MissPenalties
					if net > 5 {
						return i, o, rule.Match.Option, true
					}
				}
			}
		}
	}
	return 0, season.Opportunity{}, "", false
}

// firstDebtCommitOpportunity finds the first commit opportunity that incurs positive debt.
func firstDebtCommitOpportunity(s season.File) (tickIndex int, opp season.Opportunity, option string, ok bool) {
	for i, tick := range s.Ticks {
		for _, o := range tick.Opportunities {
			hasCommit := false
			for _, cmd := range o.AllowedCommands {
				if cmd == "commit" {
					hasCommit = true
					break
				}
			}
			if !hasCommit || len(o.AllowedOptions) == 0 {
				continue
			}
			for _, rule := range tick.Scoring.Rules {
				if rule.Match.Command == "commit" && rule.Match.Target == o.OpportunityID && rule.Delta.Debt > 0 {
					return i, o, rule.Match.Option, true
				}
			}
		}
	}
	return 0, season.Opportunity{}, "", false
}

func advanceEngineToOpportunity(t *testing.T, engine *Engine, opportunityID string) {
	t.Helper()

	for i := 0; i < len(engine.season.Ticks); i++ {
		tick := engine.season.Ticks[engine.currentIndex]
		for _, opportunity := range tick.Opportunities {
			if opportunity.OpportunityID == opportunityID {
				return
			}
		}
		engine.DebugForceClose(engine.currentEndsAt)
	}
	t.Fatalf("opportunity %q not found in season", opportunityID)
}

func TestSubmitAndScoreEpoch(t *testing.T) {
	engine := newTestEngine(t)
	_, opp, option, ok := firstCommitOpportunity(engine.season)
	if !ok {
		t.Fatalf("no commit opportunity found")
	}
	advanceEngineToOpportunity(t, engine, opp.OpportunityID)
	current := engine.Current()

	receipt, err := engine.Submit(engine.DevToken(1), ActionSubmission{
		TickID:  current.TickID,
		Command: "commit",
		Target:  opp.OpportunityID,
		Option:  option,
	}, time.Now().UTC())
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if receipt.Status != "accepted" {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}

	// Advance enough ticks for the first score epoch to publish.
	for i := 0; i < engine.season.ScoreEpochTicks; i++ {
		engine.DebugForceClose(time.Now().UTC())
	}

	epochID := engine.season.SeasonID + "-E001"
	epoch, ok := engine.ScoreEpoch(epochID)
	if !ok {
		t.Fatalf("score epoch not published")
	}
	if len(epoch.Top) == 0 {
		t.Fatalf("expected non-empty leaderboard")
	}
	if got := epoch.Top[0].PlayerID; got != engine.PublicID(1) {
		t.Fatalf("unexpected top player: %s", got)
	}
}

func TestRevealPublishedAfterLag(t *testing.T) {
	engine := newTestEngine(t)
	now := time.Now().UTC()

	_, opp, option, ok := firstCommitOpportunity(engine.season)
	if !ok {
		t.Fatalf("no commit opportunity found")
	}
	advanceEngineToOpportunity(t, engine, opp.OpportunityID)
	current := engine.Current()
	_, err := engine.Submit(engine.DevToken(2), ActionSubmission{
		TickID:  current.TickID,
		Command: "commit",
		Target:  opp.OpportunityID,
		Option:  option,
	}, now)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	engine.DebugForceClose(now)
	if _, ok := engine.Reveal(current.TickID); ok {
		t.Fatalf("reveal should not be published before lag")
	}

	// Advance enough ticks for the reveal to publish.
	for i := 1; i < engine.season.RevealLagTicks; i++ {
		engine.DebugForceClose(now)
	}
	reveal, ok := engine.Reveal(current.TickID)
	if !ok {
		t.Fatalf("expected reveal after lag")
	}
	if len(reveal.Resolutions) != 1 {
		t.Fatalf("unexpected reveal resolutions: %+v", reveal.Resolutions)
	}
}

func TestShardDeterminism(t *testing.T) {
	engine := newTestEngine(t)
	publicID := engine.PublicID(1)
	first := ShardForPublicID(publicID, 16)
	second := ShardForPublicID(publicID, 16)
	if first != second {
		t.Fatalf("expected stable shard, got %s and %s", first, second)
	}
}

func TestWALWritten(t *testing.T) {
	engine := newTestEngine(t)
	current := engine.Current()

	_, err := engine.Submit(engine.DevToken(1), ActionSubmission{
		TickID:  current.TickID,
		Command: "hold",
	}, time.Now().UTC())
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	engine.mu.RLock()
	defer engine.mu.RUnlock()
	if engine.wal == nil {
		t.Fatalf("expected wal")
	}
}

func TestSnapshotRoundTrip(t *testing.T) {
	engine := newTestEngine(t)
	now := time.Now().UTC()
	_, opp, option, ok := firstCommitOpportunity(engine.season)
	if !ok {
		t.Fatalf("no commit opportunity found")
	}
	advanceEngineToOpportunity(t, engine, opp.OpportunityID)
	current := engine.Current()
	_, err := engine.Submit(engine.DevToken(1), ActionSubmission{
		TickID:  current.TickID,
		Command: "commit",
		Target:  opp.OpportunityID,
		Option:  option,
	}, now)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	engine.DebugForceClose(now)

	snap := engine.Snapshot()
	restored := newTestEngine(t)
	if err := restored.RestoreSnapshot(snap); err != nil {
		t.Fatalf("restore snapshot: %v", err)
	}
	if restored.Current().TickID != engine.Current().TickID {
		t.Fatalf("current tick mismatch after restore: got %s want %s", restored.Current().TickID, engine.Current().TickID)
	}
	if restored.players[0].Score != engine.players[0].Score {
		t.Fatalf("restored score mismatch: got %d want %d", restored.players[0].Score, engine.players[0].Score)
	}
}

func TestSubmitIdempotentRetry(t *testing.T) {
	engine := newTestEngine(t)
	now := time.Now().UTC()
	action := ActionSubmission{
		TickID:       engine.Current().TickID,
		Command:      "hold",
		SubmissionID: "retry-1",
	}

	first, err := engine.Submit(engine.DevToken(1), action, now)
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}
	second, err := engine.Submit(engine.DevToken(1), action, now.Add(250*time.Millisecond))
	if err != nil {
		t.Fatalf("retry submit: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("retry receipt mismatch\nfirst=%+v\nsecond=%+v", first, second)
	}
}

func TestSubmitRejectsMutationAfterCommit(t *testing.T) {
	engine := newTestEngine(t)
	now := time.Now().UTC()
	_, opp, option, ok := firstCommitOpportunity(engine.season)
	if !ok {
		t.Fatalf("no commit opportunity found")
	}
	advanceEngineToOpportunity(t, engine, opp.OpportunityID)

	_, err := engine.Submit(engine.DevToken(1), ActionSubmission{
		TickID:       engine.Current().TickID,
		Command:      "hold",
		SubmissionID: "first",
	}, now)
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}

	_, err = engine.Submit(engine.DevToken(1), ActionSubmission{
		TickID:       engine.Current().TickID,
		Command:      "commit",
		Target:       opp.OpportunityID,
		Option:       option,
		SubmissionID: "second",
	}, now.Add(250*time.Millisecond))
	if !CheckErr(err, ErrorTickAlreadyCommitted()) {
		t.Fatalf("expected tick already committed, got %v", err)
	}
}

func TestSubmitRejectsConflictingSubmissionID(t *testing.T) {
	engine := newTestEngine(t)
	now := time.Now().UTC()
	_, opp, option, ok := firstCommitOpportunity(engine.season)
	if !ok {
		t.Fatalf("no commit opportunity found")
	}
	advanceEngineToOpportunity(t, engine, opp.OpportunityID)

	_, err := engine.Submit(engine.DevToken(1), ActionSubmission{
		TickID:       engine.Current().TickID,
		Command:      "commit",
		Target:       opp.OpportunityID,
		Option:       option,
		SubmissionID: "same-id",
	}, now)
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}

	// Submit a different option with the same submission ID.
	altOption := option + "-alt"
	_, err = engine.Submit(engine.DevToken(1), ActionSubmission{
		TickID:       engine.Current().TickID,
		Command:      "commit",
		Target:       opp.OpportunityID,
		Option:       altOption,
		SubmissionID: "same-id",
	}, now.Add(250*time.Millisecond))
	if !CheckErr(err, ErrorSubmissionIDConflict()) {
		t.Fatalf("expected submission id conflict, got %v", err)
	}
}

func TestRecoverFromSnapshotAndWAL(t *testing.T) {
	loadedSeason, err := season.LoadFile(filepath.Join("..", "..", "seasons", "dev", "season.json"))
	if err != nil {
		t.Fatalf("load season: %v", err)
	}

	_, opp, option, ok := firstCommitOpportunity(loadedSeason)
	if !ok {
		t.Fatalf("no commit opportunity found")
	}

	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "actions.log")
	wal, err := storage.NewWAL(walPath)
	if err != nil {
		t.Fatalf("new wal: %v", err)
	}
	defer wal.Close()

	engine := NewEngine(loadedSeason, wal, 8)
	base := time.Now().UTC()
	engine.mu.Lock()
	engine.startedAt = base
	engine.currentIndex = 0
	engine.currentEndsAt = base.Add(loadedSeason.DurationForTick(0))
	engine.mu.Unlock()

	firstTick := engine.Current().TickID
	firstActionAt := base.Add(-500 * time.Millisecond)
	if _, err := engine.Submit(engine.DevToken(1), ActionSubmission{
		TickID:  firstTick,
		Command: "commit",
		Target:  opp.OpportunityID,
		Option:  option,
	}, firstActionAt); err != nil {
		t.Fatalf("submit first action: %v", err)
	}

	snapshot := engine.Snapshot()

	secondActionAt := snapshot.SavedAtTime().Add(2 * time.Millisecond)
	if _, err := engine.Submit(engine.DevToken(2), ActionSubmission{
		TickID:  firstTick,
		Command: "commit",
		Target:  opp.OpportunityID,
		Option:  option,
	}, secondActionAt); err != nil {
		t.Fatalf("submit second action: %v", err)
	}

	firstDeadline := base.Add(loadedSeason.DurationForTick(0))
	engine.CloseCurrentTick(firstDeadline)

	secondTick := engine.Current().TickID
	// Find a valid opportunity for the second tick.
	var secondOpp season.Opportunity
	var secondOption string
	for _, tick := range loadedSeason.Ticks {
		if tick.TickID == secondTick {
			for _, o := range tick.Opportunities {
				for _, cmd := range o.AllowedCommands {
					if cmd == "commit" && len(o.AllowedOptions) > 0 {
						secondOpp = o
						secondOption = o.AllowedOptions[0]
					}
				}
			}
			break
		}
	}
	secondTickActionAt := firstDeadline.Add(750 * time.Millisecond)
	if secondOpp.OpportunityID != "" {
		if _, err := engine.Submit(engine.DevToken(1), ActionSubmission{
			TickID:  secondTick,
			Command: "commit",
			Target:  secondOpp.OpportunityID,
			Option:  secondOption,
		}, secondTickActionAt); err != nil {
			t.Fatalf("submit third action: %v", err)
		}
	} else {
		// Second tick may be observe-only; just submit hold.
		if _, err := engine.Submit(engine.DevToken(1), ActionSubmission{
			TickID:  secondTick,
			Command: "hold",
		}, secondTickActionAt); err != nil {
			t.Fatalf("submit hold on second tick: %v", err)
		}
	}

	secondDeadline := firstDeadline.Add(loadedSeason.DurationForTick(1))
	engine.CloseCurrentTick(secondDeadline)
	expected := engine.Snapshot()

	recoveryWAL, err := storage.NewWAL(walPath)
	if err != nil {
		t.Fatalf("reopen wal: %v", err)
	}
	defer recoveryWAL.Close()

	recovered := NewEngine(loadedSeason, recoveryWAL, 8)
	if err := recovered.RestoreSnapshot(snapshot); err != nil {
		t.Fatalf("restore snapshot: %v", err)
	}
	records, err := recoveryWAL.RecordsAfter(snapshot.SavedAtTime(), loadedSeason.SeasonID)
	if err != nil {
		t.Fatalf("load wal records: %v", err)
	}
	replayed, err := recovered.RecoverFromRecords(records, secondDeadline)
	if err != nil {
		t.Fatalf("recover from wal: %v", err)
	}
	if replayed != 2 {
		t.Fatalf("unexpected replay count: got %d want 2", replayed)
	}

	got := recovered.Snapshot()
	got.SavedAt = 0
	got.SavedAtUnixNano = 0
	expected.SavedAt = 0
	expected.SavedAtUnixNano = 0
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("recovered snapshot mismatch\n got=%+v\nwant=%+v", got, expected)
	}
}

func TestAbsentPlayerScheduledDebtInterest(t *testing.T) {
	engine := newTestEngine(t)
	now := time.Now().UTC()

	_, opp, option, ok := firstDebtCommitOpportunity(engine.season)
	if !ok {
		t.Fatalf("no debt-producing commit opportunity found")
	}
	advanceEngineToOpportunity(t, engine, opp.OpportunityID)
	_, err := engine.Submit(engine.DevToken(2), ActionSubmission{
		TickID:  engine.Current().TickID,
		Command: "commit",
		Target:  opp.OpportunityID,
		Option:  option,
	}, now)
	if err != nil {
		t.Fatalf("submit debt action: %v", err)
	}

	engine.CloseCurrentTick(now)

	player := engine.players[1]
	if player.Debt <= 0 {
		t.Fatalf("expected positive debt after action, got %d", player.Debt)
	}
	initialDebt := player.Debt
	nextDossierTick, ok := engine.nextTickSeqMatchingClassAfter(0, "dossier")
	if !ok {
		t.Fatalf("expected a future dossier tick")
	}
	if player.DebtDueTick != nextDossierTick {
		t.Fatalf("unexpected debt due tick: got %d want %d", player.DebtDueTick, nextDossierTick)
	}

	for engine.currentTickSeq <= player.DebtDueTick {
		engine.CloseCurrentTick(engine.currentEndsAt)
	}

	player = engine.players[1]
	if player.Debt <= initialDebt {
		t.Fatalf("expected debt to increase via interest: got %d, was %d", player.Debt, initialDebt)
	}
}

func BenchmarkSubmit(b *testing.B) {
	loadedSeason, err := season.LoadFile(filepath.Join("..", "..", "seasons", "dev", "season.json"))
	if err != nil {
		b.Fatal(err)
	}
	tmpDir := b.TempDir()
	wal, err := storage.NewWAL(filepath.Join(tmpDir, "actions.log"))
	if err != nil {
		b.Fatal(err)
	}
	defer wal.Close()
	engine := NewEngine(loadedSeason, wal, 100_000)
	current := engine.Current()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		token := engine.DevToken((i % 100_000) + 1)
		if _, err := engine.Submit(token, ActionSubmission{
			TickID:       current.TickID,
			Command:      "hold",
			SubmissionID: "bench",
		}, time.Now().UTC()); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCloseCurrentTickSparseDue(b *testing.B) {
	loadedSeason, err := season.LoadFile(filepath.Join("..", "..", "seasons", "dev", "season.json"))
	if err != nil {
		b.Fatal(err)
	}

	for i := 0; i < b.N; i++ {
		engine := NewEngine(loadedSeason, nil, 100_000)
		engine.season.ScoreEpochTicks = 1_000_000
		now := time.Now().UTC()

		engine.mu.Lock()
		engine.currentIndex = 1
		engine.currentTickSeq = 1
		engine.currentEndsAt = now
		for playerID := 0; playerID < 32; playerID++ {
			engine.players[playerID].Debt = 100
			engine.reconcilePlayerDueStateLocked(uint32(playerID), 0)
		}
		engine.mu.Unlock()

		b.StartTimer()
		engine.CloseCurrentTick(now)
		b.StopTimer()
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
