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

func TestSubmitAndScoreEpoch(t *testing.T) {
	engine := newTestEngine(t)
	current := engine.Current()

	receipt, err := engine.Submit(engine.DevToken(1), ActionSubmission{
		TickID:     current.TickID,
		Command:    "commit",
		Target:     "quest.glass_choir.7",
		Option:     "broker",
		Confidence: 0.80,
	}, time.Now().UTC())
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if receipt.Status != "accepted" {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}

	engine.DebugForceClose(time.Now().UTC())
	engine.DebugForceClose(time.Now().UTC())

	epoch, ok := engine.ScoreEpoch("dev-season-E001")
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

	current := engine.Current()
	_, err := engine.Submit(engine.DevToken(2), ActionSubmission{
		TickID:     current.TickID,
		Command:    "commit",
		Target:     "quest.glass_choir.7",
		Option:     "smuggler",
		Confidence: 0.95,
	}, now)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	engine.DebugForceClose(now)
	if _, ok := engine.Reveal(current.TickID); ok {
		t.Fatalf("reveal should not be published before lag")
	}

	engine.DebugForceClose(now)
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
		TickID:     current.TickID,
		Command:    "hold",
		Confidence: 0,
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
	current := engine.Current()
	_, err := engine.Submit(engine.DevToken(1), ActionSubmission{
		TickID:     current.TickID,
		Command:    "commit",
		Target:     "quest.glass_choir.7",
		Option:     "broker",
		Confidence: 0.75,
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
		Confidence:   0,
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

	_, err := engine.Submit(engine.DevToken(1), ActionSubmission{
		TickID:       engine.Current().TickID,
		Command:      "hold",
		Confidence:   0,
		SubmissionID: "first",
	}, now)
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}

	_, err = engine.Submit(engine.DevToken(1), ActionSubmission{
		TickID:       engine.Current().TickID,
		Command:      "commit",
		Target:       "quest.glass_choir.7",
		Option:       "broker",
		Confidence:   0.8,
		SubmissionID: "second",
	}, now.Add(250*time.Millisecond))
	if !CheckErr(err, ErrorTickAlreadyCommitted()) {
		t.Fatalf("expected tick already committed, got %v", err)
	}
}

func TestSubmitRejectsConflictingSubmissionID(t *testing.T) {
	engine := newTestEngine(t)
	now := time.Now().UTC()

	_, err := engine.Submit(engine.DevToken(1), ActionSubmission{
		TickID:       engine.Current().TickID,
		Command:      "commit",
		Target:       "quest.glass_choir.7",
		Option:       "broker",
		Confidence:   0.8,
		SubmissionID: "same-id",
	}, now)
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}

	_, err = engine.Submit(engine.DevToken(1), ActionSubmission{
		TickID:       engine.Current().TickID,
		Command:      "commit",
		Target:       "quest.glass_choir.7",
		Option:       "smuggler",
		Confidence:   0.8,
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
		TickID:     firstTick,
		Command:    "commit",
		Target:     "quest.glass_choir.7",
		Option:     "broker",
		Confidence: 0.80,
	}, firstActionAt); err != nil {
		t.Fatalf("submit first action: %v", err)
	}

	snapshot := engine.Snapshot()

	secondActionAt := snapshot.SavedAtTime().Add(2 * time.Millisecond)
	if _, err := engine.Submit(engine.DevToken(2), ActionSubmission{
		TickID:     firstTick,
		Command:    "commit",
		Target:     "quest.glass_choir.7",
		Option:     "smuggler",
		Confidence: 0.95,
	}, secondActionAt); err != nil {
		t.Fatalf("submit second action: %v", err)
	}

	firstDeadline := base.Add(loadedSeason.DurationForTick(0))
	engine.CloseCurrentTick(firstDeadline)

	secondTick := engine.Current().TickID
	secondTickActionAt := firstDeadline.Add(750 * time.Millisecond)
	if _, err := engine.Submit(engine.DevToken(1), ActionSubmission{
		TickID:     secondTick,
		Command:    "commit",
		Target:     "auth.vault.3",
		Option:     "authorize",
		Phrase:     "green rain broker",
		Confidence: 0.90,
	}, secondTickActionAt); err != nil {
		t.Fatalf("submit third action: %v", err)
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

	_, err := engine.Submit(engine.DevToken(2), ActionSubmission{
		TickID:     engine.Current().TickID,
		Command:    "commit",
		Target:     "quest.glass_choir.7",
		Option:     "smuggler",
		Confidence: 0.95,
	}, now)
	if err != nil {
		t.Fatalf("submit debt action: %v", err)
	}

	engine.CloseCurrentTick(now)

	player := engine.players[1]
	if player.Debt != 50 {
		t.Fatalf("unexpected debt after bad action: got %d want 50", player.Debt)
	}
	if player.DebtDueTick != 1 {
		t.Fatalf("unexpected debt due tick: got %d want 1", player.DebtDueTick)
	}

	dossierClose := now.Add(engine.season.DurationForTick(1))
	engine.CloseCurrentTick(dossierClose)

	player = engine.players[1]
	if player.Debt != 55 {
		t.Fatalf("unexpected debt after scheduled interest: got %d want 55", player.Debt)
	}
	if player.Score != -65 {
		t.Fatalf("unexpected score after scheduled interest: got %d want -65", player.Score)
	}
	if player.LastTickID != "S1-T0002" {
		t.Fatalf("unexpected last tick after scheduled interest: got %s", player.LastTickID)
	}
	if player.DebtDueTick != 5 {
		t.Fatalf("unexpected rescheduled debt due tick: got %d want 5", player.DebtDueTick)
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
			Confidence:   0,
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
