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

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
