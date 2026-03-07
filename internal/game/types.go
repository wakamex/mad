package game

import "time"

type Manifest struct {
	SeasonID        string `json:"season_id"`
	SchemaVersion   string `json:"schema_version"`
	ScoreEpochTicks int    `json:"score_epoch_ticks"`
	RevealLagTicks  int    `json:"reveal_lag_ticks"`
	HashShardCount  int    `json:"hash_shard_count"`
}

type Current struct {
	SeasonID          string `json:"season_id"`
	TickID            string `json:"tick_id"`
	TickURL           string `json:"tick_url"`
	NextTickAt        int64  `json:"next_tick_at"`
	NextPollAfterMS   int64  `json:"next_poll_after_ms"`
	CurrentScoreEpoch string `json:"current_score_epoch,omitempty"`
	ScoreEpochURL     string `json:"score_epoch_url,omitempty"`
}

type ActionSubmission struct {
	TickID       string  `json:"tick_id"`
	Command      string  `json:"command"`
	Target       string  `json:"target,omitempty"`
	Option       string  `json:"option,omitempty"`
	Confidence   float64 `json:"confidence,omitempty"`
	Phrase       string  `json:"phrase,omitempty"`
	Theory       string  `json:"theory,omitempty"`
	SubmissionID string  `json:"submission_id,omitempty"`
}

type ActionReceipt struct {
	Status       string `json:"status"`
	TickID       string `json:"tick_id"`
	ReceivedAt   int64  `json:"received_at"`
	SubmissionID string `json:"submission_id,omitempty"`
}

type PendingAction struct {
	Action     ActionSubmission
	ReceivedAt time.Time
}

type PlayerState struct {
	PlayerID      uint32 `json:"player_id"`
	PublicID      string `json:"player_id_public"`
	Score         int64  `json:"score"`
	Yield         int64  `json:"yield"`
	Insight       int64  `json:"insight"`
	Aura          int64  `json:"aura"`
	Debt          int64  `json:"debt"`
	MissPenalties int64  `json:"miss_penalties"`
	LastTickID    string `json:"last_tick_id,omitempty"`
	DebtDueTick   uint64 `json:"debt_due_tick,omitempty"`
	DebtGen       uint32 `json:"debt_gen,omitempty"`
}

type ScoreEpochSnapshot struct {
	ScoreEpoch  string          `json:"score_epoch"`
	PublishedAt int64           `json:"published_at"`
	Top         []ScoreboardRow `json:"top"`
	Shards      []string        `json:"shards"`
}

type ScoreboardRow struct {
	Rank     int    `json:"rank,omitempty"`
	PlayerID string `json:"player_id"`
	Score    int64  `json:"score"`
	Aura     int64  `json:"aura"`
	Debt     int64  `json:"debt"`
}

type ScoreShard struct {
	ScoreEpoch string          `json:"score_epoch"`
	ShardID    string          `json:"shard_id"`
	Players    []ScoreboardRow `json:"players"`
}

type Snapshot struct {
	SeasonID        string                           `json:"season_id"`
	SchemaVersion   string                           `json:"schema_version"`
	SavedAt         int64                            `json:"saved_at"`
	SavedAtUnixNano int64                            `json:"saved_at_unix_nano,omitempty"`
	CurrentIndex    int                              `json:"current_index"`
	CurrentTickSeq  uint64                           `json:"current_tick_seq"`
	CurrentEndsAt   int64                            `json:"current_ends_at"`
	Players         []PlayerState                    `json:"players"`
	Pending         []SnapshotPending                `json:"pending,omitempty"`
	DueEvents       []DueEvent                       `json:"due_events,omitempty"`
	ScoreEpochs     map[string]ScoreEpochSnapshot    `json:"score_epochs"`
	ScoreShards     map[string]map[string]ScoreShard `json:"score_shards"`
	Reveals         map[string]RevealPacket          `json:"reveals"`
}

type SnapshotPending struct {
	PlayerID   uint32           `json:"player_id"`
	Action     ActionSubmission `json:"action"`
	ReceivedAt int64            `json:"received_at_unix_nano"`
}

type DueEvent struct {
	DueTick    uint64 `json:"due_tick"`
	PlayerID   uint32 `json:"player_id"`
	Kind       string `json:"kind"`
	Generation uint32 `json:"generation,omitempty"`
}

type RevealPacket struct {
	TickID         string             `json:"tick_id"`
	RevealLagTicks int                `json:"reveal_lag_ticks"`
	PublishedAt    int64              `json:"published_at"`
	Resolutions    []RevealResolution `json:"resolutions"`
}

type RevealResolution struct {
	OpportunityID     string            `json:"opportunity_id"`
	BestKnownAction   RevealAction      `json:"best_known_action"`
	BadActionClasses  []RevealBadAction `json:"bad_action_classes,omitempty"`
	PublicExplanation string            `json:"public_explanation"`
	Stats             RevealStats       `json:"stats"`
}

type RevealAction struct {
	Command string `json:"command"`
	Option  string `json:"option,omitempty"`
	Target  string `json:"target,omitempty"`
	Phrase  string `json:"phrase,omitempty"`
}

type RevealBadAction struct {
	Command string `json:"command"`
	Option  string `json:"option,omitempty"`
	Outcome string `json:"outcome"`
}

type RevealStats struct {
	Submissions             int64   `json:"submissions"`
	CorrectPct              float64 `json:"correct_pct"`
	MeanConfidenceCorrect   float64 `json:"mean_confidence_correct"`
	MeanConfidenceIncorrect float64 `json:"mean_confidence_incorrect"`
	MostCommonWrongOption   string  `json:"most_common_wrong_option,omitempty"`
}

func (s Snapshot) SavedAtTime() time.Time {
	switch {
	case s.SavedAtUnixNano != 0:
		return time.Unix(0, s.SavedAtUnixNano).UTC()
	case s.SavedAt != 0:
		return time.Unix(s.SavedAt, 0).UTC()
	default:
		return time.Time{}
	}
}
