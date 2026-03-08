package game

import (
	"context"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mihai/mad/internal/season"
	"github.com/mihai/mad/internal/storage"
)

var (
	errBadAuth              = errors.New("bad auth token")
	errWrongTick            = errors.New("submission for non-current tick")
	errDeadlineMiss         = errors.New("submission missed deadline")
	errInvalidBody          = errors.New("invalid action body")
	errSubmissionIDConflict = errors.New("submission_id reused with different body")
	errTickAlreadyCommitted = errors.New("player already committed an action for this tick")
)

const (
	defaultDueWheelSize  = 1024
	dueEventDebtInterest = "debt_interest"
)

type Engine struct {
	mu             sync.RWMutex
	season         season.File
	startedAt      time.Time
	currentIndex   int
	currentTickSeq uint64
	currentEndsAt  time.Time
	wal            *storage.WAL

	players           []PlayerState
	publicIDs         []string
	authTokens        map[string]uint32
	pending           []PendingAction
	pendingSubmission []string
	pendingHash       []uint64
	pendingReceipt    []ActionReceipt
	hasPending        []bool
	activePlayers     []uint32
	activeMarked      []bool
	dueWheel          [][]DueEvent

	scoreEpochs map[string]ScoreEpochSnapshot
	scoreShards map[string]map[string]ScoreShard
	reveals     map[string]RevealPacket
}

func NewEngine(seasonFile season.File, wal *storage.WAL, devPlayers int) *Engine {
	if devPlayers < 1 {
		devPlayers = 1
	}

	players := make([]PlayerState, devPlayers)
	publicIDs := make([]string, devPlayers)
	authTokens := make(map[string]uint32, devPlayers)
	pending := make([]PendingAction, devPlayers)
	pendingSubmission := make([]string, devPlayers)
	pendingHash := make([]uint64, devPlayers)
	pendingReceipt := make([]ActionReceipt, devPlayers)
	hasPending := make([]bool, devPlayers)
	activeMarked := make([]bool, devPlayers)

	for i := range devPlayers {
		publicID := fmt.Sprintf("p_%06d", i+1)
		token := fmt.Sprintf("dev-token-%06d", i+1)
		publicIDs[i] = publicID
		players[i] = PlayerState{PlayerID: uint32(i), PublicID: publicID}
		authTokens[token] = uint32(i)
	}

	now := time.Now().UTC()
	return &Engine{
		season:            seasonFile,
		startedAt:         now,
		currentIndex:      0,
		currentEndsAt:     now.Add(seasonFile.DurationForTick(0)),
		wal:               wal,
		players:           players,
		publicIDs:         publicIDs,
		authTokens:        authTokens,
		pending:           pending,
		pendingSubmission: pendingSubmission,
		pendingHash:       pendingHash,
		pendingReceipt:    pendingReceipt,
		hasPending:        hasPending,
		activePlayers:     make([]uint32, 0, devPlayers/8+1),
		activeMarked:      activeMarked,
		dueWheel:          make([][]DueEvent, defaultDueWheelSize),
		scoreEpochs:       make(map[string]ScoreEpochSnapshot),
		scoreShards:       make(map[string]map[string]ScoreShard),
		reveals:           make(map[string]RevealPacket),
	}
}

func (e *Engine) Manifest() Manifest {
	return Manifest{
		SeasonID:        e.season.SeasonID,
		SchemaVersion:   e.season.SchemaVersion,
		ScoreEpochTicks: e.season.ScoreEpochTicks,
		RevealLagTicks:  e.season.RevealLagTicks,
		HashShardCount:  e.season.ShardCount,
	}
}

func (e *Engine) Current() Current {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.currentLocked(time.Now().UTC())
}

func (e *Engine) currentLocked(now time.Time) Current {
	tick := e.season.Ticks[e.currentIndex]
	nextPoll := max(250, time.Until(e.currentEndsAt).Milliseconds())
	currentEpoch := latestEpochID(e.scoreEpochs)
	current := Current{
		SeasonID:        e.season.SeasonID,
		TickID:          tick.TickID,
		TickURL:         fmt.Sprintf("/ticks/%s.json", tick.TickID),
		NextTickAt:      e.currentEndsAt.Unix(),
		NextPollAfterMS: nextPoll,
	}
	if currentEpoch != "" {
		current.CurrentScoreEpoch = currentEpoch
		current.ScoreEpochURL = fmt.Sprintf("/score-epochs/%s/top.json", currentEpoch)
	}
	return current
}

func (e *Engine) PublicTick(tickID string) (season.PublicTick, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, tick := range e.season.Ticks {
		if tick.TickID == tickID {
			return tick.Public(), true
		}
	}
	return season.PublicTick{}, false
}

func (e *Engine) Reveal(tickID string) (RevealPacket, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	reveal, ok := e.reveals[tickID]
	return reveal, ok
}

func (e *Engine) ScoreEpoch(epochID string) (ScoreEpochSnapshot, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	snap, ok := e.scoreEpochs[epochID]
	return snap, ok
}

func (e *Engine) ScoreShard(epochID, shardID string) (ScoreShard, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	shards, ok := e.scoreShards[epochID]
	if !ok {
		return ScoreShard{}, false
	}
	shard, ok := shards[shardID]
	return shard, ok
}

func (e *Engine) Submit(token string, action ActionSubmission, now time.Time) (ActionReceipt, error) {
	playerID, ok := e.authTokens[token]
	if !ok {
		return ActionReceipt{}, errBadAuth
	}
	if err := validateAction(action); err != nil {
		return ActionReceipt{}, errInvalidBody
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	currentTick := e.season.Ticks[e.currentIndex]
	if action.TickID != currentTick.TickID {
		return ActionReceipt{}, errWrongTick
	}
	if now.After(e.currentEndsAt) {
		return ActionReceipt{}, errDeadlineMiss
	}
	fingerprint := actionFingerprint(action)
	if e.hasPending[playerID] {
		if action.SubmissionID != "" && e.pendingSubmission[playerID] == action.SubmissionID {
			if e.pendingHash[playerID] != fingerprint {
				return ActionReceipt{}, errSubmissionIDConflict
			}
			return e.pendingReceipt[playerID], nil
		}
		return ActionReceipt{}, errTickAlreadyCommitted
	}

	receipt := ActionReceipt{
		Status:       "accepted",
		TickID:       action.TickID,
		ReceivedAt:   now.Unix(),
		SubmissionID: action.SubmissionID,
	}
	e.storePendingLocked(playerID, action, receipt, fingerprint, now)

	if e.wal != nil {
		_ = e.wal.Append(storage.ActionRecord{
			SeasonID:     e.season.SeasonID,
			PlayerID:     playerID,
			PublicID:     e.publicIDs[playerID],
			SubmissionID: action.SubmissionID,
			TickID:       action.TickID,
			Command:      action.Command,
			Target:       action.Target,
			Option:       action.Option,
			ReceivedAt:   now,
		})
	}

	return receipt, nil
}

func (e *Engine) RunScheduler(ctx context.Context) {
	e.mu.RLock()
	nextClose := e.currentEndsAt
	e.mu.RUnlock()

	timer := time.NewTimer(time.Until(nextClose))
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			e.CloseCurrentTick(nextClose)
			e.mu.RLock()
			nextClose = e.currentEndsAt
			nextDelay := time.Until(nextClose)
			e.mu.RUnlock()
			if nextDelay < 0 {
				nextDelay = 0
			}
			timer.Reset(nextDelay)
		}
	}
}

func (e *Engine) Snapshot() Snapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()

	savedAt := time.Now().UTC()

	players := make([]PlayerState, len(e.players))
	copy(players, e.players)

	pending := make([]SnapshotPending, 0, len(e.activePlayers))
	for _, playerID := range e.activePlayers {
		if !e.hasPending[playerID] {
			continue
		}
		entry := e.pending[playerID]
		pending = append(pending, SnapshotPending{
			PlayerID:   playerID,
			Action:     entry.Action,
			ReceivedAt: entry.ReceivedAt.UnixNano(),
		})
	}

	dueEvents := make([]DueEvent, 0)
	for _, bucket := range e.dueWheel {
		if len(bucket) == 0 {
			continue
		}
		dueEvents = append(dueEvents, bucket...)
	}

	scoreEpochs := make(map[string]ScoreEpochSnapshot, len(e.scoreEpochs))
	for k, v := range e.scoreEpochs {
		scoreEpochs[k] = v
	}

	scoreShards := make(map[string]map[string]ScoreShard, len(e.scoreShards))
	for epochID, shards := range e.scoreShards {
		copiedShards := make(map[string]ScoreShard, len(shards))
		for shardID, shard := range shards {
			copiedShards[shardID] = shard
		}
		scoreShards[epochID] = copiedShards
	}

	reveals := make(map[string]RevealPacket, len(e.reveals))
	for k, v := range e.reveals {
		reveals[k] = v
	}

	return Snapshot{
		SeasonID:        e.season.SeasonID,
		SchemaVersion:   e.season.SchemaVersion,
		SavedAt:         savedAt.Unix(),
		SavedAtUnixNano: savedAt.UnixNano(),
		CurrentIndex:    e.currentIndex,
		CurrentTickSeq:  e.currentTickSeq,
		CurrentEndsAt:   e.currentEndsAt.Unix(),
		Players:         players,
		Pending:         pending,
		DueEvents:       dueEvents,
		ScoreEpochs:     scoreEpochs,
		ScoreShards:     scoreShards,
		Reveals:         reveals,
	}
}

func (e *Engine) RestoreSnapshot(snapshot Snapshot) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if snapshot.SeasonID != e.season.SeasonID {
		return fmt.Errorf("snapshot season mismatch: got %s want %s", snapshot.SeasonID, e.season.SeasonID)
	}
	if len(snapshot.Players) != len(e.players) {
		return fmt.Errorf("snapshot player count mismatch: got %d want %d", len(snapshot.Players), len(e.players))
	}
	if snapshot.CurrentIndex < 0 || snapshot.CurrentIndex >= len(e.season.Ticks) {
		return fmt.Errorf("snapshot current index out of range: %d", snapshot.CurrentIndex)
	}

	copy(e.players, snapshot.Players)
	e.currentIndex = snapshot.CurrentIndex
	e.currentTickSeq = snapshot.CurrentTickSeq
	e.currentEndsAt = time.Unix(snapshot.CurrentEndsAt, 0).UTC()
	clear(e.pending)
	clear(e.pendingSubmission)
	clear(e.pendingHash)
	clear(e.pendingReceipt)
	clear(e.hasPending)
	clear(e.activeMarked)
	e.activePlayers = e.activePlayers[:0]
	e.clearDueWheelLocked()
	for _, entry := range snapshot.Pending {
		if entry.PlayerID >= uint32(len(e.players)) {
			continue
		}
		receipt := ActionReceipt{
			Status:       "accepted",
			TickID:       entry.Action.TickID,
			ReceivedAt:   time.Unix(0, entry.ReceivedAt).UTC().Unix(),
			SubmissionID: entry.Action.SubmissionID,
		}
		e.storePendingLocked(entry.PlayerID, entry.Action, receipt, actionFingerprint(entry.Action), time.Unix(0, entry.ReceivedAt).UTC())
	}
	for _, event := range snapshot.DueEvents {
		e.pushDueEventLocked(event)
	}
	e.scoreEpochs = snapshot.ScoreEpochs
	if e.scoreEpochs == nil {
		e.scoreEpochs = make(map[string]ScoreEpochSnapshot)
	}
	e.scoreShards = snapshot.ScoreShards
	if e.scoreShards == nil {
		e.scoreShards = make(map[string]map[string]ScoreShard)
	}
	e.reveals = snapshot.Reveals
	if e.reveals == nil {
		e.reveals = make(map[string]RevealPacket)
	}
	return nil
}

func (e *Engine) RecoverFromRecords(records []storage.ActionRecord, until time.Time) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	sort.SliceStable(records, func(i, j int) bool {
		return records[i].ReceivedAt.Before(records[j].ReceivedAt)
	})

	replayed := 0
	for _, record := range records {
		if record.PlayerID >= uint32(len(e.players)) {
			continue
		}

		action := ActionSubmission{
			TickID:       record.TickID,
			Command:      record.Command,
			Target:       record.Target,
			Option:       record.Option,
			SubmissionID: record.SubmissionID,
		}
		if err := validateAction(action); err != nil {
			continue
		}

		e.advanceUntilBeforeLocked(record.ReceivedAt)

		currentTick := e.season.Ticks[e.currentIndex]
		if action.TickID != currentTick.TickID || record.ReceivedAt.After(e.currentEndsAt) {
			continue
		}

		if e.hasPending[record.PlayerID] {
			if action.SubmissionID != "" && e.pendingSubmission[record.PlayerID] == action.SubmissionID && e.pendingHash[record.PlayerID] == actionFingerprint(action) {
				continue
			}
			continue
		}
		receipt := ActionReceipt{
			Status:       "accepted",
			TickID:       action.TickID,
			ReceivedAt:   record.ReceivedAt.Unix(),
			SubmissionID: action.SubmissionID,
		}
		e.storePendingLocked(record.PlayerID, action, receipt, actionFingerprint(action), record.ReceivedAt)
		replayed++
	}

	e.advanceExpiredTicksLocked(until)
	return replayed, nil
}

func (e *Engine) CloseCurrentTick(now time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.closeCurrentTickLocked(now)
}

func (e *Engine) closeCurrentTickLocked(now time.Time) {
	tick := e.season.Ticks[e.currentIndex]
	tickSeq := e.currentTickSeq
	submissionCount := int64(0)
	correctCount := int64(0)
	wrongOptions := make(map[string]int)

	for _, playerID := range e.activePlayers {
		if !e.hasPending[playerID] {
			continue
		}
		submissionCount++
		action := e.pending[playerID].Action
		delta, label, correct := evaluateAction(tick.Scoring, action)
		player := &e.players[playerID]
		applyDelta(player, tick.TickID, delta)
		e.reconcilePlayerDueStateLocked(playerID, tickSeq)
		if correct {
			correctCount++
		} else {
			if action.Option != "" {
				wrongOptions[action.Option]++
			}
		}
		e.pending[playerID] = PendingAction{}
		e.pendingSubmission[playerID] = ""
		e.pendingHash[playerID] = 0
		e.pendingReceipt[playerID] = ActionReceipt{}
		e.hasPending[playerID] = false
		e.activeMarked[playerID] = false
		_ = label
	}
	e.activePlayers = e.activePlayers[:0]
	e.processDueEventsLocked(tick, tickSeq)

	if (e.currentIndex+1)%e.season.ScoreEpochTicks == 0 {
		e.publishScoreEpochLocked(now)
	}

	revealTickIndex := e.currentIndex - e.season.RevealLagTicks + 1
	if revealTickIndex >= 0 {
		revealTick := e.season.Ticks[revealTickIndex]
		e.reveals[revealTick.TickID] = buildRevealPacket(
			revealTick,
			e.season.RevealLagTicks,
			now,
			submissionCount,
			correctCount,
			mostCommonWrongOption(wrongOptions),
		)
	}

	e.currentTickSeq++
	e.currentIndex = (e.currentIndex + 1) % len(e.season.Ticks)
	e.currentEndsAt = now.Add(e.season.DurationForTick(e.currentIndex))
}

func (e *Engine) storePendingLocked(playerID uint32, action ActionSubmission, receipt ActionReceipt, fingerprint uint64, receivedAt time.Time) {
	if !e.activeMarked[playerID] {
		e.activePlayers = append(e.activePlayers, playerID)
		e.activeMarked[playerID] = true
	}
	e.pending[playerID] = PendingAction{Action: action, ReceivedAt: receivedAt}
	e.pendingSubmission[playerID] = action.SubmissionID
	e.pendingHash[playerID] = fingerprint
	e.pendingReceipt[playerID] = receipt
	e.hasPending[playerID] = true
}

func actionFingerprint(action ActionSubmission) uint64 {
	sum := sha1.Sum([]byte(strings.Join([]string{
		action.TickID,
		action.Command,
		action.Target,
		action.Option,
		action.Theory,
	}, "\x1f")))
	return binary.BigEndian.Uint64(sum[:8])
}

func (e *Engine) clearDueWheelLocked() {
	for i := range e.dueWheel {
		e.dueWheel[i] = e.dueWheel[i][:0]
	}
}

func (e *Engine) pushDueEventLocked(event DueEvent) {
	if len(e.dueWheel) == 0 {
		return
	}
	slot := int(event.DueTick % uint64(len(e.dueWheel)))
	e.dueWheel[slot] = append(e.dueWheel[slot], event)
}

func (e *Engine) processDueEventsLocked(tick season.TickDefinition, tickSeq uint64) {
	if len(e.dueWheel) == 0 {
		return
	}
	slot := int(tickSeq % uint64(len(e.dueWheel)))
	bucket := e.dueWheel[slot]
	if len(bucket) == 0 {
		return
	}

	survivors := bucket[:0]
	for _, event := range bucket {
		if event.DueTick != tickSeq {
			survivors = append(survivors, event)
			continue
		}
		switch event.Kind {
		case dueEventDebtInterest:
			e.applyDebtInterestLocked(event.PlayerID, event.Generation, tick, tickSeq)
		}
	}
	e.dueWheel[slot] = survivors
}

func (e *Engine) applyDebtInterestLocked(playerID uint32, generation uint32, tick season.TickDefinition, tickSeq uint64) {
	if playerID >= uint32(len(e.players)) {
		return
	}
	player := &e.players[playerID]
	if player.Debt <= 0 || player.DebtDueTick != tickSeq || player.DebtGen != generation {
		return
	}

	interest := debtInterestDelta(player.Debt)
	player.Debt += interest
	player.Score -= interest
	player.LastTickID = tick.TickID
	player.DebtDueTick = 0

	e.reconcilePlayerDueStateLocked(playerID, tickSeq)
}

func (e *Engine) reconcilePlayerDueStateLocked(playerID uint32, afterTickSeq uint64) {
	player := &e.players[playerID]
	if player.Debt <= 0 {
		if player.DebtDueTick != 0 {
			player.DebtDueTick = 0
			player.DebtGen++
		}
		return
	}
	if player.DebtDueTick > afterTickSeq {
		return
	}

	nextDueTick, ok := e.nextTickSeqMatchingClassAfter(afterTickSeq, "dossier")
	if !ok {
		return
	}
	player.DebtGen++
	player.DebtDueTick = nextDueTick
	e.pushDueEventLocked(DueEvent{
		DueTick:    nextDueTick,
		PlayerID:   playerID,
		Kind:       dueEventDebtInterest,
		Generation: player.DebtGen,
	})
}

func (e *Engine) nextTickSeqMatchingClassAfter(afterTickSeq uint64, class string) (uint64, bool) {
	if len(e.season.Ticks) == 0 {
		return 0, false
	}
	for offset := uint64(1); offset <= uint64(len(e.season.Ticks))*2; offset++ {
		seq := afterTickSeq + offset
		idx := int(seq % uint64(len(e.season.Ticks)))
		if e.season.Ticks[idx].ClockClass == class {
			return seq, true
		}
	}
	return 0, false
}

func debtInterestDelta(currentDebt int64) int64 {
	if currentDebt <= 0 {
		return 0
	}
	interest := currentDebt / 10
	if interest < 1 {
		interest = 1
	}
	return interest
}

func (e *Engine) advanceUntilBeforeLocked(target time.Time) {
	for target.After(e.currentEndsAt) {
		e.closeCurrentTickLocked(e.currentEndsAt)
	}
}

func (e *Engine) advanceExpiredTicksLocked(target time.Time) {
	for !target.Before(e.currentEndsAt) {
		e.closeCurrentTickLocked(e.currentEndsAt)
	}
}

func (e *Engine) publishScoreEpochLocked(now time.Time) {
	epochID := fmt.Sprintf("%s-E%03d", e.season.SeasonID, len(e.scoreEpochs)+1)

	rows := make([]ScoreboardRow, 0, len(e.players))
	for _, player := range e.players {
		rows = append(rows, ScoreboardRow{
			PlayerID: player.PublicID,
			Score:    player.Score,
			Aura:     player.Aura,
			Debt:     player.Debt,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Score == rows[j].Score {
			return rows[i].PlayerID < rows[j].PlayerID
		}
		return rows[i].Score > rows[j].Score
	})

	topN := min(10, len(rows))
	top := make([]ScoreboardRow, topN)
	copy(top, rows[:topN])
	for i := range top {
		top[i].Rank = i + 1
	}

	shards := make(map[string]ScoreShard, e.season.ShardCount)
	shardURLs := make([]string, 0, e.season.ShardCount)
	for i := 0; i < e.season.ShardCount; i++ {
		shardID := fmt.Sprintf("%02x", i)
		shards[shardID] = ScoreShard{ScoreEpoch: epochID, ShardID: shardID}
		shardURLs = append(shardURLs, fmt.Sprintf("/score-epochs/%s/shards/%s.json", epochID, shardID))
	}

	for _, row := range rows {
		shardID := playerShard(row.PlayerID, e.season.ShardCount)
		shard := shards[shardID]
		shard.Players = append(shard.Players, row)
		shards[shardID] = shard
	}

	e.scoreShards[epochID] = shards
	e.scoreEpochs[epochID] = ScoreEpochSnapshot{
		ScoreEpoch:  epochID,
		PublishedAt: now.Unix(),
		Top:         top,
		Shards:      shardURLs,
	}
}

func playerShard(publicID string, shardCount int) string {
	sum := sha1.Sum([]byte(publicID))
	idx := int(sum[0]) % shardCount
	return fmt.Sprintf("%02x", idx)
}

func applyDelta(player *PlayerState, tickID string, delta season.ScoreDelta) {
	player.Yield += delta.Yield
	player.Insight += delta.Insight
	player.Aura += delta.Aura
	player.Debt += delta.Debt
	player.MissPenalties += delta.MissPenalties
	player.Score = player.Yield + player.Insight + player.Aura - player.Debt - player.MissPenalties
	player.LastTickID = tickID
}

func evaluateAction(plan season.ScoringPlan, action ActionSubmission) (season.ScoreDelta, string, bool) {
	for _, rule := range plan.Rules {
		if matches(rule.Match, action) {
			return rule.Delta, rule.Label, rule.Classification == "best"
		}
	}
	for _, rule := range plan.Rules {
		if rule.Match.Command == "hold" {
			return rule.Delta, rule.Label, rule.Classification == "best"
		}
	}
	return season.ScoreDelta{}, "default", false
}

func matches(match season.ActionMatch, action ActionSubmission) bool {
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

func buildRevealPacket(tick season.TickDefinition, lag int, now time.Time, submissions, correct int64, mostCommonWrong string) RevealPacket {
	bestRule := season.Rule{}
	var badRules []season.Rule
	for _, rule := range tick.Scoring.Rules {
		switch rule.Classification {
		case "best":
			bestRule = rule
		case "bad", "miss":
			badRules = append(badRules, rule)
		}
	}

	correctPct := 0.0
	if submissions > 0 {
		correctPct = float64(correct) / float64(submissions) * 100
	}

	bad := make([]RevealBadAction, 0, len(badRules))
	for _, rule := range badRules {
		bad = append(bad, RevealBadAction{
			Command: rule.Match.Command,
			Option:  rule.Match.Option,
			Outcome: rule.Label,
		})
	}

	opportunityID := ""
	if len(tick.Opportunities) > 0 {
		opportunityID = tick.Opportunities[0].OpportunityID
	}

	return RevealPacket{
		TickID:         tick.TickID,
		RevealLagTicks: lag,
		PublishedAt:    now.Unix(),
		Resolutions: []RevealResolution{
			{
				OpportunityID: opportunityID,
				BestKnownAction: RevealAction{
					Command: bestRule.Match.Command,
					Option:  bestRule.Match.Option,
					Target:  bestRule.Match.Target,
				},
				BadActionClasses:  bad,
				PublicExplanation: bestRule.Label,
				Stats: RevealStats{
					Submissions:           submissions,
					CorrectPct:            correctPct,
					MostCommonWrongOption: mostCommonWrong,
				},
			},
		},
	}
}

func validateAction(action ActionSubmission) error {
	if action.TickID == "" || action.Command == "" {
		return errInvalidBody
	}
	if len(action.Theory) > 512 {
		return errInvalidBody
	}
	return nil
}

func latestEpochID(epochs map[string]ScoreEpochSnapshot) string {
	if len(epochs) == 0 {
		return ""
	}
	keys := make([]string, 0, len(epochs))
	for key := range epochs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys[len(keys)-1]
}

func mostCommonWrongOption(counts map[string]int) string {
	best := ""
	bestN := 0
	for option, n := range counts {
		if n > bestN || (n == bestN && option < best) {
			best = option
			bestN = n
		}
	}
	return best
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func (e *Engine) DebugForceClose(now time.Time) {
	e.CloseCurrentTick(now)
}

func (e *Engine) DevToken(player int) string {
	return fmt.Sprintf("dev-token-%06d", player)
}

func (e *Engine) PublicID(player int) string {
	return e.publicIDs[player-1]
}

func (e *Engine) PlayerCount() int {
	return len(e.players)
}

func ShardForPublicID(publicID string, shardCount int) string {
	return playerShard(publicID, shardCount)
}

func CheckErr(err error, target error) bool {
	return errors.Is(err, target)
}

func ErrorBadAuth() error              { return errBadAuth }
func ErrorWrongTick() error            { return errWrongTick }
func ErrorDeadlineMiss() error         { return errDeadlineMiss }
func ErrorInvalidBody() error          { return errInvalidBody }
func ErrorSubmissionIDConflict() error { return errSubmissionIDConflict }
func ErrorTickAlreadyCommitted() error { return errTickAlreadyCommitted }

func EncodeShardHint(publicID string, shardCount int) string {
	return hex.EncodeToString([]byte(playerShard(publicID, shardCount)))
}
