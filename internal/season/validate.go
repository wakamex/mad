package season

import (
	"errors"
	"fmt"
)

func Validate(file File) error {
	var errs []error

	if file.SeasonID == "" {
		errs = append(errs, fmt.Errorf("season_id is required"))
	}
	if len(file.Ticks) == 0 {
		errs = append(errs, fmt.Errorf("at least one tick is required"))
	}
	if file.ScoreEpochTicks <= 0 {
		errs = append(errs, fmt.Errorf("score_epoch_ticks must be > 0"))
	}
	if file.RevealLagTicks <= 0 {
		errs = append(errs, fmt.Errorf("reveal_lag_ticks must be > 0"))
	}
	if file.ShardCount <= 0 {
		errs = append(errs, fmt.Errorf("shard_count must be > 0"))
	}

	seenTickIDs := make(map[string]struct{}, len(file.Ticks))
	seenOpportunityIDs := make(map[string]string)

	for tickIndex, tick := range file.Ticks {
		prefix := fmt.Sprintf("tick[%d]", tickIndex)
		if tick.TickID == "" {
			errs = append(errs, fmt.Errorf("%s: tick_id is required", prefix))
		} else if _, exists := seenTickIDs[tick.TickID]; exists {
			errs = append(errs, fmt.Errorf("%s: duplicate tick_id %q", prefix, tick.TickID))
		} else {
			seenTickIDs[tick.TickID] = struct{}{}
		}
		if tick.DurationMS <= 0 {
			errs = append(errs, fmt.Errorf("%s: duration_ms must be > 0", prefix))
		}
		// Observe-only ticks (e.g. seed_clue_chain) have no Opportunities
		// or Scoring — the agent sees prose but takes no action.
		observeOnly := len(tick.Opportunities) == 0 && len(tick.Scoring.Rules) == 0
		if !observeOnly && len(tick.Opportunities) == 0 {
			errs = append(errs, fmt.Errorf("%s: at least one opportunity is required", prefix))
		}
		if !observeOnly && len(tick.Scoring.Rules) == 0 {
			errs = append(errs, fmt.Errorf("%s: at least one scoring rule is required", prefix))
		}
		for regimeIndex, regime := range tick.ActiveSourceRegimes {
			regimePrefix := fmt.Sprintf("%s active_source_regimes[%d]", prefix, regimeIndex)
			if regime.RegimeID == "" {
				errs = append(errs, fmt.Errorf("%s: regime_id is required", regimePrefix))
			}
			if regime.Label == "" {
				errs = append(errs, fmt.Errorf("%s: label is required", regimePrefix))
			}
		}

		if observeOnly {
			continue
		}
		opportunities := make(map[string]Opportunity, len(tick.Opportunities))
		for opportunityIndex, opportunity := range tick.Opportunities {
			opPrefix := fmt.Sprintf("%s opportunity[%d]", prefix, opportunityIndex)
			if opportunity.OpportunityID == "" {
				errs = append(errs, fmt.Errorf("%s: opportunity_id is required", opPrefix))
				continue
			}
			if _, exists := opportunities[opportunity.OpportunityID]; exists {
				errs = append(errs, fmt.Errorf("%s: duplicate opportunity_id %q within tick", opPrefix, opportunity.OpportunityID))
				continue
			}
			if previousTick, exists := seenOpportunityIDs[opportunity.OpportunityID]; exists {
				errs = append(errs, fmt.Errorf("%s: opportunity_id %q already used by %s", opPrefix, opportunity.OpportunityID, previousTick))
				continue
			}
			if len(opportunity.AllowedCommands) == 0 {
				errs = append(errs, fmt.Errorf("%s: allowed_commands must not be empty", opPrefix))
			}
			for requirementIndex, requirement := range opportunity.PublicRequirements {
				reqPrefix := fmt.Sprintf("%s public_requirements[%d]", opPrefix, requirementIndex)
				if requirement.Metric == "" {
					errs = append(errs, fmt.Errorf("%s: metric is required", reqPrefix))
				}
				if requirement.Operator == "" {
					errs = append(errs, fmt.Errorf("%s: operator is required", reqPrefix))
				}
			}
			opportunities[opportunity.OpportunityID] = opportunity
			seenOpportunityIDs[opportunity.OpportunityID] = tick.TickID
		}

		hasHoldRule := false
		for ruleIndex, rule := range tick.Scoring.Rules {
			rulePrefix := fmt.Sprintf("%s rule[%d]", prefix, ruleIndex)
			switch rule.Classification {
			case "best", "bad", "miss":
			default:
				errs = append(errs, fmt.Errorf("%s: classification %q is invalid", rulePrefix, rule.Classification))
			}
			if rule.Match.Command == "" {
				errs = append(errs, fmt.Errorf("%s: match.command is required", rulePrefix))
				continue
			}
			if rule.Match.Command == "hold" {
				hasHoldRule = true
				continue
			}
			if rule.Match.Target == "" {
				errs = append(errs, fmt.Errorf("%s: non-hold rules must set match.target", rulePrefix))
				continue
			}
			opportunity, ok := opportunities[rule.Match.Target]
			if !ok {
				errs = append(errs, fmt.Errorf("%s: match.target %q does not match any opportunity_id in tick", rulePrefix, rule.Match.Target))
				continue
			}
			if !contains(opportunity.AllowedCommands, rule.Match.Command) {
				errs = append(errs, fmt.Errorf("%s: command %q is not allowed by opportunity %q", rulePrefix, rule.Match.Command, opportunity.OpportunityID))
			}
			if rule.Match.Option != "" && len(opportunity.AllowedOptions) > 0 && !contains(opportunity.AllowedOptions, rule.Match.Option) {
				errs = append(errs, fmt.Errorf("%s: option %q is not allowed by opportunity %q", rulePrefix, rule.Match.Option, opportunity.OpportunityID))
			}
		}
		if !hasHoldRule {
			errs = append(errs, fmt.Errorf("%s: scoring must include a hold fallback rule", prefix))
		}
	}

	return errors.Join(errs...)
}

func contains(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}
