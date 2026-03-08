package season

import "fmt"

const (
	devClusterBeatCount   = 20
	devElementsPerCluster = 5
	devMinElementBeats    = 2
	devMaxElementBeats    = 5
)

type devFaction struct {
	ID                    string
	Name                  string
	Protocol              string
	StabilizeBonus        int64
	ExploitBonus          int64
	StabilizeDebtRelief   int64
	StabilizeRepThreshold int64
	StabilizeRepSpend     int64
	StabilizeDebtCap      int64
	ExploitAuraThreshold  int64
	ExploitAuraSpend      int64
	ExploitDebtCap        int64
}

type devRegime struct {
	ID               string
	Label            string
	Description      string
	AffectedSources  []string
	OfferBestOption  string
	MarketBestOption string
}

var devFactions = []devFaction{
	{ID: "glass_choir", Name: "Glass Choir", Protocol: "glass curtain", StabilizeBonus: 14, ExploitBonus: 0, StabilizeDebtRelief: 10, StabilizeRepThreshold: 16, StabilizeRepSpend: 2, StabilizeDebtCap: 46, ExploitAuraThreshold: 20, ExploitAuraSpend: 10, ExploitDebtCap: 30},
	{ID: "civic_ward", Name: "Civic Ward", Protocol: "civic cordon", StabilizeBonus: 10, ExploitBonus: 2, StabilizeDebtRelief: 12, StabilizeRepThreshold: 10, StabilizeRepSpend: 5, StabilizeDebtCap: 48, ExploitAuraThreshold: 18, ExploitAuraSpend: 8, ExploitDebtCap: 30},
	{ID: "harbor_union", Name: "Harbor Union", Protocol: "dock brace", StabilizeBonus: 6, ExploitBonus: 10, StabilizeDebtRelief: 6, StabilizeRepThreshold: 8, StabilizeRepSpend: 6, StabilizeDebtCap: 40, ExploitAuraThreshold: 12, ExploitAuraSpend: 6, ExploitDebtCap: 36},
	{ID: "archive_office", Name: "Archive Office", Protocol: "checksum lock", StabilizeBonus: 8, ExploitBonus: 6, StabilizeDebtRelief: 8, StabilizeRepThreshold: 18, StabilizeRepSpend: 3, StabilizeDebtCap: 42, ExploitAuraThreshold: 18, ExploitAuraSpend: 5, ExploitDebtCap: 34},
	{ID: "silt_exchange", Name: "Silt Exchange", Protocol: "market divert", StabilizeBonus: 4, ExploitBonus: 14, StabilizeDebtRelief: 4, StabilizeRepThreshold: 7, StabilizeRepSpend: 7, StabilizeDebtCap: 38, ExploitAuraThreshold: 10, ExploitAuraSpend: 7, ExploitDebtCap: 38},
	{ID: "relay_guild", Name: "Relay Guild", Protocol: "relay brace", StabilizeBonus: 12, ExploitBonus: 4, StabilizeDebtRelief: 9, StabilizeRepThreshold: 14, StabilizeRepSpend: 2, StabilizeDebtCap: 44, ExploitAuraThreshold: 22, ExploitAuraSpend: 9, ExploitDebtCap: 32},
}

var devRegimes = []devRegime{
	{
		ID:               "suppression_window",
		Label:            "Suppression Window",
		Description:      "Official bulletins omit sensitive cargo details while intermediaries quietly redirect value.",
		AffectedSources:  []string{"official_bulletin", "faction_notice"},
		OfferBestOption:  "broker",
		MarketBestOption: "broker",
	},
	{
		ID:               "atonement_drive",
		Label:            "Atonement Drive",
		Description:      "Public notices reward visible contrition and quarantine chains over quiet profit taking.",
		AffectedSources:  []string{"public_notice", "critical_broadcast"},
		OfferBestOption:  "penitent",
		MarketBestOption: "quarantine",
	},
	{
		ID:               "archive_audit",
		Label:            "Archive Audit Sweep",
		Description:      "Archive fragments and certified records gain leverage while sloppy chains are punished.",
		AffectedSources:  []string{"archive_fragment", "archive_console"},
		OfferBestOption:  "auditor",
		MarketBestOption: "auction",
	},
}

var devColors = []string{"green", "amber", "saffron", "ivory", "cobalt", "scarlet", "silver", "ashen"}
var devPhenomena = []string{"rain", "fog", "dust", "static", "hail", "glow", "drift", "mire"}
var devRoles = []string{"broker", "warden", "auditor", "carrier", "scribe", "factor", "porter", "binder"}
var devMaterials = []string{"glass", "salt", "wire", "resin", "silk", "amber", "basalt", "signal"}
var devAliases = []string{"anchor", "choirmark", "ledger-key", "veil-token", "resonance seal", "relay shard", "storm docket", "proof reed"}
var devDistricts = []string{"southern ward", "north quay", "mirror steps", "relay row", "silt exchange", "archive annex", "ember causeway", "river stairs"}
var devWorkTypes = []string{"cleanup", "escort", "ledger", "sorting", "inspection", "repair", "triage", "registry"}
var devHazards = []string{"containment wash", "archive firebreak", "quarantine bloom", "signal spill", "relay fracture", "silt collapse", "fog surge", "glass quake"}

func BuildGeneratedDevSeasonIR(tickCount int) (IRFile, error) {
	if tickCount <= 0 {
		return IRFile{}, fmt.Errorf("tick count must be positive")
	}
	if tickCount%devClusterBeatCount != 0 {
		return IRFile{}, fmt.Errorf("tick count must be a multiple of %d", devClusterBeatCount)
	}

	clusterCount := tickCount / devClusterBeatCount
	ir := IRFile{
		SchemaVersion:   "v1alpha1",
		SeasonID:        fmt.Sprintf("dev-season-%dtick", tickCount),
		Title:           fmt.Sprintf("The Latent Labyrinth (%d-Tick Dev)", tickCount),
		CompileSeed:     1007,
		ScoreEpochTicks: 12,
		RevealLagTicks:  6,
		ShardCount:      64,
		ClockDefaults: map[string]int64{
			"standard":  45_000,
			"dossier":   90_000,
			"interrupt": 20_000,
		},
		Elements: make([]StoryElement, 0, clusterCount*devElementsPerCluster),
	}

	for cluster := 0; cluster < clusterCount; cluster++ {
		theme := buildDevTheme(cluster)
		plan := buildDevClusterPlan(cluster)
		ir.Elements = append(ir.Elements,
			buildStandingWorkElement(cluster, theme, plan),
			buildClueChainElement(cluster, theme, plan),
			buildReputationLadderElement(cluster, theme, plan),
			buildPreparednessHazardElement(cluster, theme, plan),
			buildPayoffGateElement(cluster, theme, plan),
		)
	}

	if err := ValidateIR(ir); err != nil {
		return IRFile{}, err
	}
	return ir, nil
}

type devTheme struct {
	ClusterIndex int
	Faction      devFaction
	Regime       devRegime
	Color        string
	Phenomenon   string
	Role         string
	Material     string
	Alias        string
	District     string
	WorkA        string
	WorkB        string
	Hazard       string
	RepTier      int64
	AuraTier     int64
	DebtCap      int64
}

type devClusterPlan struct {
	Standing int
	Clue     int
	Ladder   int
	Hazard   int
	Payoff   int
}

func buildDevTheme(cluster int) devTheme {
	color := devColors[cluster%len(devColors)]
	phenomenon := devPhenomena[(cluster*3+1)%len(devPhenomena)]
	role := devRoles[(cluster*5+2)%len(devRoles)]
	return devTheme{
		ClusterIndex: cluster,
		Faction:      devFactions[cluster%len(devFactions)],
		Regime:       devRegimes[cluster%len(devRegimes)],
		Color:        color,
		Phenomenon:   phenomenon,
		Role:         role,
		Material:     devMaterials[(cluster*7+3)%len(devMaterials)],
		Alias:        devAliases[(cluster*11+4)%len(devAliases)],
		District:     devDistricts[(cluster*13+5)%len(devDistricts)],
		WorkA:        devWorkTypes[cluster%len(devWorkTypes)],
		WorkB:        devWorkTypes[(cluster+3)%len(devWorkTypes)],
		Hazard:       devHazards[(cluster*17+6)%len(devHazards)],
		RepTier:      4 + int64((cluster/len(devFactions))%4)*2,
		AuraTier:     6 + int64(cluster%3)*2,
		DebtCap:      36 + int64((cluster/len(devRegimes))%5)*4,
	}
}

func buildDevClusterPlan(cluster int) devClusterPlan {
	lengths := boundedBeatPartition(cluster, devClusterBeatCount, devElementsPerCluster, devMinElementBeats, devMaxElementBeats)
	return devClusterPlan{
		Standing: lengths[0],
		Clue:     lengths[1],
		Ladder:   lengths[2],
		Hazard:   lengths[3],
		Payoff:   lengths[4],
	}
}

func boundedBeatPartition(seed, total, slots, minValue, maxValue int) []int {
	values := make([]int, 0, slots)
	remaining := total
	for i := 0; i < slots; i++ {
		remainingSlots := slots - i - 1
		lower := minValue
		if candidate := remaining - remainingSlots*maxValue; candidate > lower {
			lower = candidate
		}
		upper := maxValue
		if candidate := remaining - remainingSlots*minValue; candidate < upper {
			upper = candidate
		}
		span := upper - lower + 1
		value := lower + ((seed*17 + i*11 + seed/3) % span)
		values = append(values, value)
		remaining -= value
	}
	return values
}

func buildStandingWorkElement(cluster int, theme devTheme, plan devClusterPlan) StoryElement {
	cooldownName := fmt.Sprintf("%s.%03d.standing", theme.Faction.ID, cluster+1)
	beats := make([]StoryBeat, 0, plan.Standing)

	for i := 1; i <= plan.Standing; i++ {
		workName := standingWorkName(theme, cluster, i)
		target := fmt.Sprintf("quest.cluster.%03d.work.%d", cluster+1, i)
		sourceText := standingWorkText(theme, i, workName)
		repDelta := int64(2)
		if i >= 3 {
			repDelta = 1
		}
		delta := ScoreDelta{
			Yield:         0,
			Insight:       int64((i + 1) % 2),
			Aura:          int64(i % 2),
			Debt:          2 + int64((i-1)/3),
			MissPenalties: 0,
		}
		beats = append(beats, StoryBeat{
			BeatID:          fmt.Sprintf("cluster_%03d.standing.%d", cluster+1, i),
			ClockClass:      "standard",
			ResourceTouches: []string{"reputation", "availability", "debt", "cooldowns"},
			Sources: []Source{
				{
					SourceID:   fmt.Sprintf("public_notice.cluster_%03d.work_%d", cluster+1, i),
					SourceType: "public_notice",
					Text:       sourceText,
				},
			},
			Opportunities: []Opportunity{
				{
					OpportunityID:   target,
					AllowedCommands: []string{"commit", "hold"},
					AllowedOptions:  []string{workName},
				},
			},
			Scoring: ScoringPlan{
				Rules: []Rule{
					{
						Match: ActionMatch{Command: "commit", Target: target, Option: workName},
						Requirements: RuleRequirements{
							RequiresAvailability: []string{defaultAvailability},
						},
						Effects: StateEffects{
							LockTicks:         1,
							AvailabilityDelta: "on_shift",
							SetCooldowns:      map[string]int{cooldownName: 2 + i},
							ReputationDelta:   map[string]int64{theme.Faction.ID: repDelta},
						},
						Delta:          delta,
						Label:          fmt.Sprintf("You took the %s shift. The immediate return was small, but %s logged your name.", workName, theme.Faction.Name),
						Classification: "best",
					},
					{
						Match:          ActionMatch{Command: "hold"},
						Delta:          ScoreDelta{},
						Label:          "You ignored the routine work.",
						Classification: "miss",
					},
				},
			},
		})
	}

	return StoryElement{
		ElementID:       fmt.Sprintf("cluster_%03d_standing", cluster+1),
		Family:          "standing_work_loop",
		LatentVars:      []string{fmt.Sprintf("cluster_%03d_optional_standing", cluster+1)},
		ResourceTouches: []string{"reputation", "availability", "debt", "cooldowns"},
		Beats:           beats,
	}
}

func buildClueChainElement(cluster int, theme devTheme, plan devClusterPlan) StoryElement {
	beats := make([]StoryBeat, 0, plan.Clue)
	for i := 1; i <= plan.Clue; i++ {
		target := fmt.Sprintf("clue.cluster.%03d.%d", cluster+1, i)
		beat := StoryBeat{
			BeatID:          fmt.Sprintf("cluster_%03d.clue.%d", cluster+1, i),
			ClockClass:      clueClockClass(i),
			ProducesTags:    []string{clueTag(cluster, i)},
			ResourceTouches: []string{"insight"},
			Sources: []Source{
				{
					SourceID:   fmt.Sprintf("%s.cluster_%03d.%d", clueSourceType(i), cluster+1, i),
					SourceType: clueSourceType(i),
					Text:       clueText(theme, i),
				},
			},
			Opportunities: []Opportunity{
				{
					OpportunityID:   target,
					AllowedCommands: []string{"inspect", "hold"},
				},
			},
			Scoring: ScoringPlan{
				Rules: []Rule{
					{
						Match:          ActionMatch{Command: "inspect", Target: target},
						Delta:          ScoreDelta{Yield: 0, Insight: 2 + int64((i%3)+1), Aura: 0, Debt: 0, MissPenalties: 0},
						Label:          clueInspectLabel(i),
						Classification: "best",
					},
					{
						Match:          ActionMatch{Command: "hold"},
						Delta:          ScoreDelta{Yield: 0, Insight: 0, Aura: 0, Debt: 0, MissPenalties: 1},
						Label:          "The clue passed without annotation.",
						Classification: "miss",
					},
				},
			},
		}
		if i == 1 {
			beat.ActiveSourceRegimes = []SourceRegime{buildSourceRegime(theme.Regime)}
		}
		beats = append(beats, beat)
	}

	return StoryElement{
		ElementID:       fmt.Sprintf("cluster_%03d_clues", cluster+1),
		Family:          "seed_clue_chain",
		LatentVars:      []string{fmt.Sprintf("cluster_%03d_suffix", cluster+1), fmt.Sprintf("cluster_%03d_bias", cluster+1)},
		ResourceTouches: []string{"insight"},
		Beats:           beats,
	}
}

func buildReputationLadderElement(cluster int, theme devTheme, plan devClusterPlan) StoryElement {
	options := []string{"broker", "penitent", "auditor"}
	wrongA, wrongB := otherOptions(options, theme.Regime.OfferBestOption)
	beats := make([]StoryBeat, 0, plan.Ladder)

	for i := 1; i <= plan.Ladder; i++ {
		target := fmt.Sprintf("quest.cluster.%03d.offer.%d", cluster+1, i)
		consumes := []string{clueTag(cluster, minInt(i, maxInt(plan.Clue, 2)))}
		if i == 1 {
			consumes = []string{clueTag(cluster, 1)}
		}
		publicReqs := []PublicRequirement(nil)
		thresholdRep := theme.RepTier + int64(maxInt(0, i-2))*2
		thresholdDebt := theme.DebtCap - int64(maxInt(0, i-2))*2
		if thresholdDebt < 18 {
			thresholdDebt = 18
		}
		if i >= 2 {
			publicReqs = append(publicReqs,
				PublicRequirement{
					Metric:   "reputation",
					Scope:    theme.Faction.ID,
					Operator: ">=",
					Value:    thresholdRep,
					Label:    fmt.Sprintf("%s standing %d+ unlocks the trusted tier.", theme.Faction.Name, thresholdRep),
				},
				PublicRequirement{
					Metric:   "debt",
					Operator: "<=",
					Value:    thresholdDebt,
					Label:    fmt.Sprintf("Debt %d or lower preserves trusted handling.", thresholdDebt),
				},
			)
		}

		precursors := []string{fmt.Sprintf("cluster_%03d.clue.%d", cluster+1, minInt(i, plan.Clue))}
		if i == 1 {
			precursors = append(precursors, fmt.Sprintf("cluster_%03d.standing.1", cluster+1))
		} else {
			precursors = append(precursors, fmt.Sprintf("cluster_%03d.offer.%d", cluster+1, i-1))
		}

		rules := []Rule{
			{
				Match:          ActionMatch{Command: "inspect", Target: target},
				Delta:          ScoreDelta{Yield: 0, Insight: 2, Aura: 0, Debt: 0, MissPenalties: 0},
				Label:          "You inspected the social framing before acting.",
				Classification: "best",
			},
		}

		if i >= 2 {
			rules = append(rules, Rule{
				Match: ActionMatch{Command: "commit", Target: target, Option: theme.Regime.OfferBestOption},
				Requirements: RuleRequirements{
					RequiresAvailability: []string{defaultAvailability},
					MinReputation:        map[string]int64{theme.Faction.ID: thresholdRep},
					MaxDebt:              thresholdDebt,
				},
				Delta:          ScoreDelta{Yield: 115 + int64(i*35), Insight: 24 + int64(i*6), Aura: 8 + int64(i), Debt: 0, MissPenalties: 0},
				Label:          "Your earlier standing unlocked the premium tier.",
				Classification: "best",
			})
		}
		rules = append(rules,
			Rule{
				Match: ActionMatch{Command: "commit", Target: target, Option: theme.Regime.OfferBestOption},
				Requirements: RuleRequirements{
					RequiresAvailability: []string{defaultAvailability},
				},
				Delta:          ScoreDelta{Yield: 70 + int64(i*18), Insight: 14 + int64(i*4), Aura: 5 + int64(i/2), Debt: 0, MissPenalties: 0},
				Label:          fmt.Sprintf("%s was the correct social read for the public regime.", theme.Regime.OfferBestOption),
				Classification: "best",
			},
			Rule{
				Match: ActionMatch{Command: "commit", Target: target, Option: wrongA},
				Requirements: RuleRequirements{
					RequiresAvailability: []string{defaultAvailability},
				},
				Delta:          ScoreDelta{Yield: 0, Insight: 0, Aura: 0, Debt: 30 + int64(i*4), MissPenalties: 10 + int64(i)},
				Label:          fmt.Sprintf("%s signaled the wrong reading and cost you standing.", wrongA),
				Classification: "bad",
			},
			Rule{
				Match: ActionMatch{Command: "commit", Target: target, Option: wrongB},
				Requirements: RuleRequirements{
					RequiresAvailability: []string{defaultAvailability},
				},
				Delta:          ScoreDelta{Yield: 0, Insight: 0, Aura: 0, Debt: 36 + int64(i*5), MissPenalties: 12 + int64(i)},
				Label:          fmt.Sprintf("%s poisoned the trust window.", wrongB),
				Classification: "bad",
			},
			Rule{
				Match:          ActionMatch{Command: "hold"},
				Delta:          ScoreDelta{Yield: 0, Insight: 0, Aura: 0, Debt: 0, MissPenalties: 7 + int64(i)},
				Label:          "The faction offer expired.",
				Classification: "miss",
			},
		)

		beat := StoryBeat{
			BeatID:           fmt.Sprintf("cluster_%03d.offer.%d", cluster+1, i),
			ClockClass:       "standard",
			ConsumesTags:     consumes,
			ResourceTouches:  []string{"reputation", "debt", "availability"},
			PrecursorBeatIDs: precursors,
			Sources: []Source{
				{
					SourceID:   fmt.Sprintf("faction_notice.cluster_%03d.offer_%d", cluster+1, i),
					SourceType: "faction_notice",
					Text:       ladderPrompt(theme, i >= 2),
				},
			},
			Opportunities: []Opportunity{
				{
					OpportunityID:      target,
					AllowedCommands:    []string{"inspect", "commit", "hold"},
					AllowedOptions:     options,
					PublicRequirements: publicReqs,
				},
			},
			Scoring: ScoringPlan{Rules: rules},
		}
		if i == 1 {
			beat.ActiveSourceRegimes = []SourceRegime{buildSourceRegime(theme.Regime)}
		}
		beats = append(beats, beat)
	}

	return StoryElement{
		ElementID:       fmt.Sprintf("cluster_%03d_reputation", cluster+1),
		Family:          "reputation_ladder",
		LatentVars:      []string{fmt.Sprintf("cluster_%03d_social_read", cluster+1)},
		ResourceTouches: []string{"reputation", "debt", "availability"},
		Beats:           beats,
	}
}

func buildPreparednessHazardElement(cluster int, theme devTheme, plan devClusterPlan) StoryElement {
	beats := make([]StoryBeat, 0, plan.Hazard)

	for i := 1; i <= plan.Hazard; i++ {
		target := fmt.Sprintf("hazard.cluster.%03d.%d", cluster+1, i)
		stabilizeTarget := target + ".stabilize"
		exploitTarget := target + ".exploit"
		stabilizeRep := theme.Faction.StabilizeRepThreshold
		stabilizeRepSpend := theme.Faction.StabilizeRepSpend
		stabilizeDebtCap := theme.Faction.StabilizeDebtCap
		exploitAura := theme.Faction.ExploitAuraThreshold
		exploitAuraSpend := theme.Faction.ExploitAuraSpend
		exploitDebtCap := theme.Faction.ExploitDebtCap
		sourceText := fmt.Sprintf(
			"%s struck the %s. %s is offering %s to trusted operators, while risk-takers can still try to exploit the surge if their aura and debt stay within public limits.",
			theme.Hazard,
			theme.District,
			theme.Faction.Name,
			theme.Faction.Protocol,
		)

		precursors := []string{fmt.Sprintf("cluster_%03d.clue.%d", cluster+1, minInt(maxInt(plan.Clue, 2), maxInt(2, i)))}
		if i > 1 {
			precursors = append(precursors, fmt.Sprintf("cluster_%03d.hazard.%d", cluster+1, i-1))
		}

		rules := []Rule{
			{
				Match: ActionMatch{Command: "commit", Target: stabilizeTarget, Option: "stabilize"},
				Requirements: RuleRequirements{
					RequiresAvailability: []string{defaultAvailability},
					MinReputation:        map[string]int64{theme.Faction.ID: stabilizeRep},
					MaxDebt:              stabilizeDebtCap,
				},
				Effects: StateEffects{
					ReputationDelta: map[string]int64{theme.Faction.ID: -stabilizeRepSpend},
				},
				Delta: ScoreDelta{
					Yield:         32 + int64(i*5) + theme.Faction.StabilizeBonus,
					Insight:       10 + int64(i*2),
					Aura:          0,
					Debt:          -(5 + theme.Faction.StabilizeDebtRelief/2),
					MissPenalties: 0,
				},
				Label:          fmt.Sprintf("%s trusted you to run the %s protocol and contain the spill cleanly.", theme.Faction.Name, theme.Faction.Protocol),
				Classification: "best",
			},
			{
				Match: ActionMatch{Command: "commit", Target: stabilizeTarget, Option: "stabilize"},
				Requirements: RuleRequirements{
					RequiresAvailability: []string{defaultAvailability},
				},
				Delta: ScoreDelta{
					Yield:         0,
					Insight:       0,
					Aura:          0,
					Debt:          2 + int64(i/2),
					MissPenalties: 18 + int64(i*2),
				},
				Label:          "You reached for faction cover without enough standing to clear the lane.",
				Classification: "bad",
			},
			{
				Match: ActionMatch{Command: "commit", Target: exploitTarget, Option: "exploit"},
				Requirements: RuleRequirements{
					RequiresAvailability: []string{defaultAvailability},
					MinAura:              exploitAura,
					MaxDebt:              exploitDebtCap,
				},
				Delta: ScoreDelta{
					Yield:         74 + int64(i*8) + theme.Faction.ExploitBonus,
					Insight:       8 + int64(i*2),
					Aura:          -exploitAuraSpend,
					Debt:          2 + int64(i/2),
					MissPenalties: 0,
				},
				Label:          "You exploited the disruption for value, burning readiness to do it.",
				Classification: "best",
			},
			{
				Match: ActionMatch{Command: "commit", Target: exploitTarget, Option: "exploit"},
				Requirements: RuleRequirements{
					RequiresAvailability: []string{defaultAvailability},
				},
				Delta: ScoreDelta{
					Yield:         0,
					Insight:       0,
					Aura:          0,
					Debt:          4 + int64(i),
					MissPenalties: 20 + int64(i*2),
				},
				Label:          "You lunged for upside without enough visible margin to survive the blast.",
				Classification: "bad",
			},
			{
				Match:          ActionMatch{Command: "hold"},
				Delta:          ScoreDelta{Yield: 0, Insight: 0, Aura: 0, Debt: 0, MissPenalties: 14 + int64(i*2)},
				Label:          "The interrupt rolled through while you hesitated.",
				Classification: "bad",
			},
		}

		beats = append(beats, StoryBeat{
			BeatID:           fmt.Sprintf("cluster_%03d.hazard.%d", cluster+1, i),
			ClockClass:       "interrupt",
			ConsumesTags:     []string{clueTag(cluster, minInt(maxInt(plan.Clue, 2), maxInt(2, i)))},
			ResourceTouches:  []string{"availability", "aura", "debt", "reputation"},
			PrecursorBeatIDs: precursors,
			Sources: []Source{
				{
					SourceID:   fmt.Sprintf("critical_broadcast.cluster_%03d.%d", cluster+1, i),
					SourceType: "critical_broadcast",
					Text:       sourceText,
				},
			},
			Opportunities: []Opportunity{
				{
					OpportunityID:   stabilizeTarget,
					AllowedCommands: []string{"commit", "hold"},
					AllowedOptions:  []string{"stabilize"},
					PublicRequirements: []PublicRequirement{
						{
							Metric:   "reputation",
							Scope:    theme.Faction.ID,
							Operator: ">=",
							Value:    stabilizeRep,
							Label:    fmt.Sprintf("%s standing %d+ unlocks %s; successful use spends %d standing.", theme.Faction.Name, stabilizeRep, theme.Faction.Protocol, stabilizeRepSpend),
						},
						{
							Metric:   "debt",
							Operator: "<=",
							Value:    stabilizeDebtCap,
							Label:    fmt.Sprintf("Debt %d or lower preserves trusted response access.", stabilizeDebtCap),
						},
					},
				},
				{
					OpportunityID:   exploitTarget,
					AllowedCommands: []string{"commit"},
					AllowedOptions:  []string{"exploit"},
					PublicRequirements: []PublicRequirement{
						{
							Metric:   "aura",
							Operator: ">=",
							Value:    exploitAura,
							Label:    fmt.Sprintf("Aura %d+ unlocks safe exploitation; successful use spends %d aura.", exploitAura, exploitAuraSpend),
						},
						{
							Metric:   "debt",
							Operator: "<=",
							Value:    exploitDebtCap,
							Label:    fmt.Sprintf("Debt %d or lower keeps the exploit lane viable.", exploitDebtCap),
						},
					},
				},
			},
			Scoring: ScoringPlan{Rules: rules},
		})
	}

	return StoryElement{
		ElementID:       fmt.Sprintf("cluster_%03d_hazard", cluster+1),
		Family:          "hazard_interrupt",
		LatentVars:      []string{fmt.Sprintf("cluster_%03d_hazard_bias", cluster+1)},
		ResourceTouches: []string{"availability", "aura", "debt", "reputation"},
		Beats:           beats,
	}
}

func buildPayoffGateElement(cluster int, theme devTheme, plan devClusterPlan) StoryElement {
	marketOptions := []string{"broker", "auction", "quarantine"}
	marketWrongA, marketWrongB := otherOptions(marketOptions, theme.Regime.MarketBestOption)
	beats := make([]StoryBeat, 0, plan.Payoff)

	for i := 1; i <= plan.Payoff; i++ {
		target := fmt.Sprintf("quest.cluster.%03d.payoff.%d", cluster+1, i)
		clockClass := "standard"
		opportunity := Opportunity{
			OpportunityID:   target,
			AllowedCommands: []string{"inspect", "commit", "hold"},
			AllowedOptions:  marketOptions,
		}
		sources := []Source{
			{
				SourceID:   fmt.Sprintf("market_gossip.cluster_%03d.payoff_%d", cluster+1, i),
				SourceType: "market_gossip",
				Text:       marketPrompt(theme),
			},
		}
		clueBeat := minInt(plan.Clue, maxInt(2, i))
		consumes := []string{clueTag(cluster, 1), clueTag(cluster, clueBeat)}
		precursors := []string{
			fmt.Sprintf("cluster_%03d.offer.%d", cluster+1, minInt(plan.Ladder, maxInt(1, i))),
			fmt.Sprintf("cluster_%03d.clue.%d", cluster+1, clueBeat),
		}

		rules := []Rule{
			{
				Match:          ActionMatch{Command: "inspect", Target: target},
				Delta:          ScoreDelta{Yield: 0, Insight: 3, Aura: 0, Debt: 0, MissPenalties: 0},
				Label:          "You checked how the public contradiction mapped onto the current payoff window.",
				Classification: "best",
			},
		}

		if i == 2 {
			clockClass = "dossier"
			suffixOptions := dossierSuffixOptions(theme, cluster)
			opportunity = Opportunity{
				OpportunityID:   target,
				AllowedCommands: []string{"commit", "hold"},
				AllowedOptions:  suffixOptions,
			}
			sources = []Source{
				{
					SourceID:   fmt.Sprintf("archive_console.cluster_%03d", cluster+1),
					SourceType: "archive_console",
					Text:       fmt.Sprintf("Archive console: authorization requires selecting the verified %s-line suffix.", theme.Material),
				},
			}
			rules = []Rule{
				{
					Match: ActionMatch{Command: "commit", Target: target, Option: theme.Role},
					Requirements: RuleRequirements{
						RequiresAvailability: []string{defaultAvailability},
					},
					Delta:          ScoreDelta{Yield: 40, Insight: 220, Aura: 12, Debt: 0, MissPenalties: 0},
					Label:          "The dossier choice bound the whole clue chain together.",
					Classification: "best",
				},
				{
					Match:          ActionMatch{Command: "hold"},
					Delta:          ScoreDelta{Yield: 0, Insight: 0, Aura: 0, Debt: 0, MissPenalties: 6},
					Label:          "You let the dossier window pass without attempting the authorization choice.",
					Classification: "miss",
				},
			}
			for _, option := range suffixOptions {
				if option == theme.Role {
					continue
				}
				rules = append(rules, Rule{
					Match: ActionMatch{Command: "commit", Target: target, Option: option},
					Requirements: RuleRequirements{
						RequiresAvailability: []string{defaultAvailability},
					},
					Delta:          ScoreDelta{Yield: 0, Insight: 0, Aura: 0, Debt: 70, MissPenalties: 30},
					Label:          "The archive rejected the wrong suffix and charged the attempt against your credibility.",
					Classification: "bad",
				})
			}
			precursors = []string{
				fmt.Sprintf("cluster_%03d.offer.%d", cluster+1, minInt(plan.Ladder, 2)),
				fmt.Sprintf("cluster_%03d.clue.%d", cluster+1, minInt(plan.Clue, 2)),
			}
		} else {
			if i > 2 {
				precursors = append(precursors, fmt.Sprintf("cluster_%03d.payoff.%d", cluster+1, i-1))
			}
			rules = append(rules,
				Rule{
					Match: ActionMatch{Command: "commit", Target: target, Option: theme.Regime.MarketBestOption},
					Requirements: RuleRequirements{
						RequiresAvailability: []string{defaultAvailability},
					},
					Delta:          ScoreDelta{Yield: 80 + int64(i*20), Insight: 20 + int64(i*4), Aura: 5 + int64(i/2), Debt: 0, MissPenalties: 0},
					Label:          fmt.Sprintf("%s was the correct conversion of the clue pair into immediate value.", theme.Regime.MarketBestOption),
					Classification: "best",
				},
				Rule{
					Match: ActionMatch{Command: "commit", Target: target, Option: marketWrongA},
					Requirements: RuleRequirements{
						RequiresAvailability: []string{defaultAvailability},
					},
					Delta:          ScoreDelta{Yield: 0, Insight: 0, Aura: 0, Debt: 30 + int64(i*4), MissPenalties: 10 + int64(i)},
					Label:          fmt.Sprintf("%s turned the public signal into the wrong trade.", marketWrongA),
					Classification: "bad",
				},
				Rule{
					Match: ActionMatch{Command: "commit", Target: target, Option: marketWrongB},
					Requirements: RuleRequirements{
						RequiresAvailability: []string{defaultAvailability},
					},
					Delta:          ScoreDelta{Yield: 0, Insight: 0, Aura: 0, Debt: 42 + int64(i*5), MissPenalties: 14 + int64(i)},
					Label:          fmt.Sprintf("%s was the expensive misread.", marketWrongB),
					Classification: "bad",
				},
				Rule{
					Match:          ActionMatch{Command: "hold"},
					Delta:          ScoreDelta{Yield: 0, Insight: 0, Aura: 0, Debt: 0, MissPenalties: 8 + int64(i)},
					Label:          "The payoff window closed while you watched.",
					Classification: "miss",
				},
			)
		}

		beats = append(beats, StoryBeat{
			BeatID:           fmt.Sprintf("cluster_%03d.payoff.%d", cluster+1, i),
			ClockClass:       clockClass,
			ConsumesTags:     consumes,
			ResourceTouches:  []string{"insight", "aura", "debt", "availability"},
			PrecursorBeatIDs: precursors,
			Sources:          sources,
			Opportunities:    []Opportunity{opportunity},
			Scoring:          ScoringPlan{Rules: rules},
		})
	}

	return StoryElement{
		ElementID:       fmt.Sprintf("cluster_%03d_payoff", cluster+1),
		Family:          "payoff_gate",
		LatentVars:      []string{fmt.Sprintf("cluster_%03d_payoff", cluster+1)},
		ResourceTouches: []string{"insight", "aura", "debt", "availability"},
		Beats:           beats,
	}
}

func standingWorkName(theme devTheme, cluster, beat int) string {
	switch beat {
	case 1:
		return theme.WorkA
	case 2:
		return theme.WorkB
	default:
		return devWorkTypes[(cluster+beat*3)%len(devWorkTypes)]
	}
}

func standingWorkText(theme devTheme, beat int, workName string) string {
	switch beat {
	case 1:
		return fmt.Sprintf("%s posted another low-pay %s shift in the %s. The work is ordinary, the memory of who showed up is not.", theme.Faction.Name, workName, theme.District)
	case 2:
		return fmt.Sprintf("A second %s rota opened near the %s. Nobody promises a prize, only that reliable names keep resurfacing later.", workName, theme.District)
	default:
		return fmt.Sprintf("Another %s rota opened under %s supervision. The wage is forgettable, the optionality is not.", workName, theme.Faction.Name)
	}
}

func dossierSuffixOptions(theme devTheme, cluster int) []string {
	options := []string{theme.Role}
	for offset := 0; len(options) < len(devRoles); offset++ {
		candidate := devRoles[(cluster*3+offset)%len(devRoles)]
		if candidate == theme.Role {
			continue
		}
		options = append(options, candidate)
	}
	return options
}

func clueTag(cluster, beat int) string {
	switch beat {
	case 1:
		return fmt.Sprintf("cluster_%03d.signal.primary", cluster+1)
	case 2:
		return fmt.Sprintf("cluster_%03d.signal.alias", cluster+1)
	default:
		return fmt.Sprintf("cluster_%03d.signal.fragment_%d", cluster+1, beat-2)
	}
}

func clueClockClass(beat int) string {
	if beat == 2 || beat == 4 {
		return "dossier"
	}
	return "standard"
}

func clueSourceType(beat int) string {
	switch beat {
	case 1:
		return "official_bulletin"
	case 2:
		return "archive_fragment"
	case 3:
		return "market_gossip"
	case 4:
		return "archive_console"
	default:
		return "faction_notice"
	}
}

func clueText(theme devTheme, beat int) string {
	switch beat {
	case 1:
		return fmt.Sprintf("Official bulletin: no %s %s crossed the %s under %s conditions.", theme.Color, theme.Material, theme.District, theme.Phenomenon)
	case 2:
		return fmt.Sprintf("Archive fragment: older ledgers call the %s consignment a %s, and the verified authorization retained the %s suffix.", theme.Material, theme.Alias, theme.Role)
	case 3:
		return fmt.Sprintf("Market gossip ties the %s %s line to the %s suffix, not to literal cargo shape.", theme.Color, theme.Material, theme.Role)
	case 4:
		return fmt.Sprintf("Console note: the %s docket only clears when the %s lineage is treated as the same asset family.", theme.Alias, theme.Material)
	default:
		return fmt.Sprintf("Faction note: the %s regime keeps rewarding operators who remember the %s/%s pairing.", theme.Regime.Label, theme.Color, theme.Phenomenon)
	}
}

func clueInspectLabel(beat int) string {
	switch beat {
	case 1:
		return "You logged the contradiction between the bulletin and the market."
	case 2:
		return "You recorded the alias lineage and the role suffix."
	case 3:
		return "You bound the role suffix back to the public market signal."
	case 4:
		return "You indexed the console lineage note against the earlier alias."
	default:
		return "You preserved another small but lawful clue fragment."
	}
}

func buildSourceRegime(regime devRegime) SourceRegime {
	return SourceRegime{
		RegimeID:            regime.ID,
		Label:               regime.Label,
		Description:         regime.Description,
		AffectedSourceTypes: append([]string(nil), regime.AffectedSources...),
	}
}

func ladderPrompt(theme devTheme, trustedTier bool) string {
	tier := "initial"
	if trustedTier {
		tier = "trusted"
	}
	switch theme.Regime.OfferBestOption {
	case "penitent":
		return fmt.Sprintf("%s %s offer: the %s circuit wants visible contrition and audited hands. Quiet profit is being punished this cycle.", theme.Faction.Name, tier, theme.Color)
	case "auditor":
		return fmt.Sprintf("%s %s offer: only verified custody and careful auditors are being rewarded while checksum disputes spread through the %s.", theme.Faction.Name, tier, theme.District)
	default:
		return fmt.Sprintf("%s %s offer: discreet brokers are being favored while official notices keep pretending the %s line does not exist.", theme.Faction.Name, tier, theme.Alias)
	}
}

func marketPrompt(theme devTheme) string {
	switch theme.Regime.MarketBestOption {
	case "quarantine":
		return fmt.Sprintf("Market brief: buyers will pay for controlled quarantine chains on %s %s touched by %s exposure.", theme.Color, theme.Material, theme.Phenomenon)
	case "auction":
		return fmt.Sprintf("Market brief: certified lots from the %s are moving best through public auction, not quiet side deals.", theme.District)
	default:
		return fmt.Sprintf("Market brief: the suppressed %s line is still richest when moved through discreet brokers.", theme.Alias)
	}
}

func otherOptions(options []string, best string) (string, string) {
	other := make([]string, 0, len(options)-1)
	for _, option := range options {
		if option != best {
			other = append(other, option)
		}
	}
	return other[0], other[1]
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
