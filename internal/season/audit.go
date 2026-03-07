package season

import "fmt"

type IRAuditReport struct {
	FamilyCounts                map[string]int `json:"family_counts"`
	TagConsumingBeats           int            `json:"tag_consuming_beats"`
	CrossElementDependencyBeats int            `json:"cross_element_dependency_beats"`
	StandingWorkElements        int            `json:"standing_work_elements"`
	WeakStandingWorkElements    []string       `json:"weak_standing_work_elements,omitempty"`
	FlatGreedyBeats             []string       `json:"flat_greedy_beats,omitempty"`
	Warnings                    []string       `json:"warnings,omitempty"`
}

func AuditIR(ir IRFile) IRAuditReport {
	report := IRAuditReport{
		FamilyCounts:             make(map[string]int),
		WeakStandingWorkElements: make([]string, 0),
		FlatGreedyBeats:          make([]string, 0),
	}

	beatLocations := make(map[string]beatLocation, totalBeatCount(ir.Elements))
	beatToElement := make(map[string]string, totalBeatCount(ir.Elements))
	tagConsumers := make(map[string]map[string]struct{})
	for elementIndex, element := range ir.Elements {
		report.FamilyCounts[element.Family]++
		for beatIndex, beat := range element.Beats {
			beatLocations[beat.BeatID] = beatLocation{elementIndex: elementIndex, beatIndex: beatIndex}
			beatToElement[beat.BeatID] = element.ElementID
			for _, tag := range requiredTagsForBeat(beat) {
				if tagConsumers[tag] == nil {
					tagConsumers[tag] = make(map[string]struct{})
				}
				tagConsumers[tag][beat.BeatID] = struct{}{}
			}
		}
	}
	guaranteedEarlier := make(map[string]map[string]struct{}, totalBeatCount(ir.Elements))
	producers := collectTagProducerSets(ir)
	for _, element := range ir.Elements {
		for _, beat := range element.Beats {
			guaranteedEarlier[beat.BeatID] = guaranteedEarlierBeats(ir, beatLocations, beat.BeatID)
		}
	}

	for _, element := range ir.Elements {
		if element.Family == "standing_work_loop" {
			report.StandingWorkElements++
			report.WeakStandingWorkElements = append(report.WeakStandingWorkElements, auditStandingWorkElement(element, beatToElement, tagConsumers)...)
		}
		for _, beat := range element.Beats {
			if len(beat.ConsumesTags) > 0 {
				report.TagConsumingBeats++
			}
			if hasCrossElementDependency(beat, element.ElementID, beatToElement, guaranteedEarlier[beat.BeatID], producers) {
				report.CrossElementDependencyBeats++
			}
			if isFlatGreedyBeat(beat) {
				report.FlatGreedyBeats = append(report.FlatGreedyBeats, beat.BeatID)
			}
		}
	}

	if report.CrossElementDependencyBeats == 0 {
		report.Warnings = append(report.Warnings, "season has no cross-element dependency beats")
	}
	if len(report.FlatGreedyBeats) > 0 {
		report.Warnings = append(report.Warnings, fmt.Sprintf("season has %d flat greedy beats", len(report.FlatGreedyBeats)))
	}
	if len(report.WeakStandingWorkElements) > 0 {
		report.Warnings = append(report.Warnings, fmt.Sprintf("season has %d weak standing work elements", len(report.WeakStandingWorkElements)))
	}
	return report
}

func auditStandingWorkElement(element StoryElement, beatToElement map[string]string, tagConsumers map[string]map[string]struct{}) []string {
	var warnings []string
	if !standingWorkHasMeaningfulCost(element) {
		warnings = append(warnings, fmt.Sprintf("%s: no meaningful immediate cost, lock, cooldown, or commitment pressure", element.ElementID))
	}
	if standingWorkFanoutCount(element, beatToElement, tagConsumers) < 2 {
		warnings = append(warnings, fmt.Sprintf("%s: produced state fans out to fewer than 2 downstream beats", element.ElementID))
	}
	return warnings
}

func standingWorkHasMeaningfulCost(element StoryElement) bool {
	for _, beat := range element.Beats {
		for _, rule := range beat.Scoring.Rules {
			if scalarScore(rule.Delta) < 0 ||
				rule.Effects.LockTicks > 0 ||
				rule.Effects.AvailabilityDelta != "" ||
				len(rule.Effects.SetCooldowns) > 0 ||
				rule.Effects.InventoryDelta != 0 {
				return true
			}
		}
	}
	return false
}

func standingWorkFanoutCount(element StoryElement, beatToElement map[string]string, tagConsumers map[string]map[string]struct{}) int {
	consumerBeats := make(map[string]struct{})
	for _, beat := range element.Beats {
		for _, tag := range producedTagsForBeat(beat) {
			for consumerBeatID := range tagConsumers[tag] {
				if beatToElement[consumerBeatID] == element.ElementID {
					continue
				}
				consumerBeats[consumerBeatID] = struct{}{}
			}
		}
	}
	return len(consumerBeats)
}

func producedTagsForBeat(beat StoryBeat) []string {
	tags := append([]string{}, beat.ProducesTags...)
	for _, rule := range beat.Scoring.Rules {
		tags = append(tags, rule.Effects.AddTags...)
	}
	return tags
}

func requiredTagsForBeat(beat StoryBeat) []string {
	tags := append([]string{}, beat.ConsumesTags...)
	for _, rule := range beat.Scoring.Rules {
		tags = append(tags, rule.Requirements.RequiresAllTags...)
		tags = append(tags, rule.Requirements.RequiresAnyTags...)
		tags = append(tags, rule.Requirements.ForbidsTags...)
	}
	return tags
}

func hasCrossElementDependency(beat StoryBeat, elementID string, beatToElement map[string]string, guaranteedEarlier map[string]struct{}, producers tagProducerSets) bool {
	for earlierBeatID := range guaranteedEarlier {
		if otherElementID := beatToElement[earlierBeatID]; otherElementID != "" && otherElementID != elementID {
			return true
		}
	}

	requiredTags := append([]string{}, beat.ConsumesTags...)
	for _, rule := range beat.Scoring.Rules {
		requiredTags = append(requiredTags, rule.Requirements.RequiresAllTags...)
		requiredTags = append(requiredTags, rule.Requirements.RequiresAnyTags...)
		requiredTags = append(requiredTags, rule.Requirements.ForbidsTags...)
	}
	for _, tag := range requiredTags {
		for _, producerBeatID := range producers.Possible[tag] {
			if otherElementID := beatToElement[producerBeatID]; otherElementID != "" && otherElementID != elementID {
				return true
			}
		}
	}
	return false
}

func isFlatGreedyBeat(beat StoryBeat) bool {
	if len(beat.ConsumesTags) > 0 || len(beat.ResourceTouches) > 0 {
		return false
	}

	bestScore, ok := int64(0), false
	bestCount := 0
	for _, rule := range beat.Scoring.Rules {
		if len(rule.Requirements.RequiresAllTags) > 0 ||
			len(rule.Requirements.RequiresAnyTags) > 0 ||
			len(rule.Requirements.ForbidsTags) > 0 ||
			len(rule.Requirements.RequiresAvailability) > 0 ||
			len(rule.Requirements.ForbidsAvailability) > 0 ||
			len(rule.Requirements.RequiresCooldownReady) > 0 ||
			len(rule.Effects.AddTags) > 0 ||
			len(rule.Effects.RemoveTags) > 0 ||
			rule.Effects.LockTicks > 0 ||
			rule.Effects.InventoryDelta != 0 ||
			len(rule.Effects.ReputationDelta) > 0 ||
			rule.Effects.AvailabilityDelta != "" ||
			len(rule.Effects.SetCooldowns) > 0 {
			return false
		}
		if rule.Classification == "best" {
			bestCount++
			score := scalarScore(rule.Delta)
			if !ok || score > bestScore {
				bestScore = score
				ok = true
			}
		}
	}
	if !ok || bestCount != 1 {
		return false
	}

	for _, rule := range beat.Scoring.Rules {
		if rule.Classification == "best" {
			continue
		}
		if scalarScore(rule.Delta) >= bestScore {
			return false
		}
	}
	return true
}

func scalarScore(delta ScoreDelta) int64 {
	return delta.Yield + delta.Insight + delta.Aura - delta.Debt - delta.MissPenalties
}
