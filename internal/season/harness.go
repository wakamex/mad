package season

import "slices"

type HarnessState struct {
	sim simulatedPlayerState
}

type HarnessStateSnapshot struct {
	CurrentTick  int              `json:"current_tick"`
	Ledger       SimulatedLedger  `json:"ledger"`
	Reputation   map[string]int64 `json:"reputation"`
	Availability string           `json:"availability"`
	Cooldowns    map[string]int   `json:"cooldowns"`
	Inventory    int              `json:"inventory"`
	ActiveTags   []string         `json:"active_tags,omitempty"`
}

type HarnessOutcome struct {
	AppliedAction SimulatedAction      `json:"applied_action"`
	AppliedRule   Rule                 `json:"applied_rule"`
	Correct       bool                 `json:"correct"`
	Label         string               `json:"label"`
	ScoreBefore   int64                `json:"score_before"`
	ScoreAfter    int64                `json:"score_after"`
	ScoreDelta    int64                `json:"score_delta"`
	State         HarnessStateSnapshot `json:"state"`
}

func NewHarnessState() HarnessState {
	return HarnessState{sim: newSimulatedPlayerState()}
}

func (h *HarnessState) AdvanceToTick(tickIndex int) {
	advanceSimulatedStateToTick(&h.sim, tickIndex)
}

func (h *HarnessState) Snapshot() HarnessStateSnapshot {
	reputation := make(map[string]int64, len(h.sim.Reputation))
	for faction, value := range h.sim.Reputation {
		reputation[faction] = value
	}
	cooldowns := make(map[string]int, len(h.sim.CooldownReadyTickByName))
	for name, tick := range h.sim.CooldownReadyTickByName {
		cooldowns[name] = tick
	}
	tags := make([]string, 0, len(h.sim.Tags))
	for tag := range h.sim.Tags {
		tags = append(tags, tag)
	}
	slices.Sort(tags)
	return HarnessStateSnapshot{
		CurrentTick:  h.sim.CurrentTick,
		Ledger:       h.sim.Ledger,
		Reputation:   reputation,
		Availability: h.sim.Availability,
		Cooldowns:    cooldowns,
		Inventory:    h.sim.Inventory,
		ActiveTags:   tags,
	}
}

func (h *HarnessState) ApplyAction(tick TickDefinition, action SimulatedAction) HarnessOutcome {
	scoreBefore := h.sim.Ledger.Score
	rule, correct := evaluateSimulatedAction(tick.Scoring, action, h.sim)
	applyRuleToSimulatedState(&h.sim, rule)
	return HarnessOutcome{
		AppliedAction: action,
		AppliedRule:   rule,
		Correct:       correct,
		Label:         rule.Label,
		ScoreBefore:   scoreBefore,
		ScoreAfter:    h.sim.Ledger.Score,
		ScoreDelta:    h.sim.Ledger.Score - scoreBefore,
		State:         h.Snapshot(),
	}
}
