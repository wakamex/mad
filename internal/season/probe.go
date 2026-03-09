package season

import (
	"fmt"
	"sort"
	"strings"
)

// ProbeReport is the output of the leakage probe. It measures how much
// information the current tick alone leaks about the correct action,
// without any history or memory.
type ProbeReport struct {
	SchemaVersion string              `json:"schema_version"`
	SeasonID      string              `json:"season_id"`
	TickCount     int                 `json:"tick_count"`
	ProbeableTicks int                `json:"probeable_ticks"`
	Families      map[string]ProbeFamily `json:"families"`
	Sources       map[string]ProbeSource `json:"sources"`
	Templates     []ProbeTemplate     `json:"templates,omitempty"`
	Summary       ProbeSummary        `json:"summary"`
}

// ProbeFamily aggregates leakage metrics for one story-element family.
type ProbeFamily struct {
	Ticks          int     `json:"ticks"`
	OptionCount    float64 `json:"avg_option_count"`
	RandomAccuracy float64 `json:"random_accuracy"`

	// Keyword overlap: does the prose share distinctive tokens with the correct option?
	KeywordHits      int     `json:"keyword_hits"`
	KeywordAccuracy  float64 `json:"keyword_accuracy"`

	// Template fingerprint: does the exact prose (with fill words) uniquely identify the answer?
	// High values here are expected in a deterministic game — this is the learning target.
	TemplateDistinct int     `json:"template_distinct_patterns"`
	TemplateAccuracy float64 `json:"template_accuracy"`

	// Skeleton: does the prose structure (with fill words stripped) predict the answer?
	// High values here mean the template skeleton itself leaks, even without learning fill-word mappings.
	SkeletonDistinct int     `json:"skeleton_distinct_patterns"`
	SkeletonAccuracy float64 `json:"skeleton_accuracy"`

	// Source-type: can the source_type alone predict the answer?
	SourceTypeAccuracy float64 `json:"source_type_accuracy"`

	// Majority class accuracy: how often you'd be correct guessing the most
	// common answer for this family. Separates answer imbalance from skeleton leakage.
	MajorityAccuracy float64 `json:"majority_accuracy"`

	// Combined leakage score: max of keyword and skeleton accuracy minus majority baseline.
	// Uses majority (not random) so answer-class imbalance doesn't inflate leakage.
	LeakageScore float64 `json:"leakage_score"`
}

// ProbeSource aggregates leakage by source_type.
type ProbeSource struct {
	Ticks           int     `json:"ticks"`
	KeywordAccuracy float64 `json:"keyword_accuracy"`
	TemplateAccuracy float64 `json:"template_accuracy"`
}

// ProbeTemplate records a distinct prose-pattern -> answer mapping.
type ProbeTemplate struct {
	Family       string `json:"family"`
	ProsePattern string `json:"prose_pattern"`
	BestOption   string `json:"best_option"`
	Occurrences  int    `json:"occurrences"`
}

// ProbeSummary is the top-level leakage verdict.
type ProbeSummary struct {
	TotalProbeable          int     `json:"total_probeable"`
	OverallRandomAccuracy   float64 `json:"overall_random_accuracy"`
	OverallKeywordAccuracy  float64 `json:"overall_keyword_accuracy"`
	OverallTemplateAccuracy float64 `json:"overall_template_accuracy"`
	OverallSkeletonAccuracy float64 `json:"overall_skeleton_accuracy"`
	WorstFamily             string  `json:"worst_family"`
	WorstFamilyLeakage      float64 `json:"worst_family_leakage"`
	Verdict                 string  `json:"verdict"`
}

// probeableTick is a tick with a non-trivial choice and a clear best action.
type probeableTick struct {
	tickIndex   int
	tick        TickDefinition
	bestAction  SimulatedAction
	bestOption  string
	allOptions  []string
	family      string
	sourceTypes []string
	proseTexts  []string
}

// RunProbe analyzes a compiled season for local semantic leakage.
func RunProbe(f File) ProbeReport {
	probeable := extractProbeableTicks(f)

	familyBuckets := make(map[string][]probeableTick)
	sourceBuckets := make(map[string][]probeableTick)
	for _, pt := range probeable {
		familyBuckets[pt.family] = append(familyBuckets[pt.family], pt)
		for _, st := range pt.sourceTypes {
			sourceBuckets[st] = append(sourceBuckets[st], pt)
		}
	}

	families := make(map[string]ProbeFamily)
	var allTemplates []ProbeTemplate
	for family, ticks := range familyBuckets {
		pf, templates := probeFamily(family, ticks)
		families[family] = pf
		allTemplates = append(allTemplates, templates...)
	}

	sources := make(map[string]ProbeSource)
	for sourceType, ticks := range sourceBuckets {
		sources[sourceType] = probeSourceType(ticks)
	}

	sort.Slice(allTemplates, func(i, j int) bool {
		return allTemplates[i].Occurrences > allTemplates[j].Occurrences
	})

	summary := buildProbeSummary(probeable, families)

	return ProbeReport{
		SchemaVersion:  "v1",
		SeasonID:       f.SeasonID,
		TickCount:      len(f.Ticks),
		ProbeableTicks: len(probeable),
		Families:       families,
		Sources:        sources,
		Templates:      allTemplates,
		Summary:        summary,
	}
}

func extractProbeableTicks(f File) []probeableTick {
	// Use a fresh player state to evaluate which rules are accessible.
	// We want "best from tick 0 state" to measure what a zero-history
	// agent could do by reading only the current tick.
	state := newSimulatedPlayerState()
	var result []probeableTick

	for i, tick := range f.Ticks {
		advanceSimulatedStateToTick(&state, i)

		actions := EnumerateActions(tick)
		if len(actions) <= 1 {
			// hold-only tick, nothing to probe
			continue
		}

		// Find best rule under greedy (full-knowledge) evaluation.
		bestRule, ok := bestRuleForTick(tick, state)
		if !ok {
			continue
		}

		bestAction := SimulatedAction{
			Command: bestRule.Match.Command,
			Target:  bestRule.Match.Target,
			Option:  bestRule.Match.Option,
		}

		// Collect unique non-hold option labels from all opportunities.
		optionSet := make(map[string]struct{})
		for _, a := range actions {
			if a.Command == "hold" {
				continue
			}
			key := a.Option
			if key == "" {
				key = a.Command
			}
			optionSet[key] = struct{}{}
		}
		options := make([]string, 0, len(optionSet))
		for o := range optionSet {
			options = append(options, o)
		}
		sort.Strings(options)

		if len(options) <= 1 {
			continue // only one non-hold choice, trivially solvable
		}

		bestOpt := bestAction.Option
		if bestOpt == "" {
			bestOpt = bestAction.Command
		}

		var sourceTypes, proseTexts []string
		for _, s := range tick.Sources {
			sourceTypes = append(sourceTypes, s.SourceType)
			proseTexts = append(proseTexts, s.Text)
		}

		family := tick.Annotations.Family
		if family == "" {
			family = "unknown"
		}

		result = append(result, probeableTick{
			tickIndex:   i,
			tick:        tick,
			bestAction:  bestAction,
			bestOption:  bestOpt,
			allOptions:  options,
			family:      family,
			sourceTypes: sourceTypes,
			proseTexts:  proseTexts,
		})

		// Apply greedy action to advance state for subsequent ticks.
		applyRuleToSimulatedState(&state, bestRule)
	}
	return result
}

func probeFamily(family string, ticks []probeableTick) (ProbeFamily, []ProbeTemplate) {
	n := len(ticks)
	if n == 0 {
		return ProbeFamily{}, nil
	}

	totalOptions := 0
	randomCorrect := 0.0
	keywordCorrect := 0
	templateCorrect := 0
	skeletonCorrect := 0
	sourceTypeCorrect := 0

	// Template fingerprinting: full prose pattern (preserves fill words).
	type templateKey struct {
		pattern string
		option  string
	}
	templateCounts := make(map[templateKey]int)
	patternToOptions := make(map[string]map[string]int)

	// Skeleton fingerprinting: stripped prose (fill words removed).
	skeletonToOptions := make(map[string]map[string]int)

	// Source-type fingerprinting.
	sourceTypeToOptions := make(map[string]map[string]int)

	for _, pt := range ticks {
		totalOptions += len(pt.allOptions)
		randomCorrect += 1.0 / float64(len(pt.allOptions))

		if keywordProbeCorrect(pt) {
			keywordCorrect++
		}

		// Template fingerprint (instance-level, preserves fill words).
		pattern := normalizeProsePattern(pt.proseTexts)
		tk := templateKey{pattern: pattern, option: pt.bestOption}
		templateCounts[tk]++
		if patternToOptions[pattern] == nil {
			patternToOptions[pattern] = make(map[string]int)
		}
		patternToOptions[pattern][pt.bestOption]++

		// Skeleton fingerprint (structure-level, strips fill words).
		skeleton := normalizeSkeletonPattern(pt.proseTexts)
		if skeletonToOptions[skeleton] == nil {
			skeletonToOptions[skeleton] = make(map[string]int)
		}
		skeletonToOptions[skeleton][pt.bestOption]++

		// Source-type fingerprint.
		stKey := strings.Join(pt.sourceTypes, "+")
		if sourceTypeToOptions[stKey] == nil {
			sourceTypeToOptions[stKey] = make(map[string]int)
		}
		sourceTypeToOptions[stKey][pt.bestOption]++
	}

	// Template accuracy.
	for _, pt := range ticks {
		pattern := normalizeProsePattern(pt.proseTexts)
		if majorityOption(patternToOptions[pattern]) == pt.bestOption {
			templateCorrect++
		}
	}

	// Skeleton accuracy.
	for _, pt := range ticks {
		skeleton := normalizeSkeletonPattern(pt.proseTexts)
		if majorityOption(skeletonToOptions[skeleton]) == pt.bestOption {
			skeletonCorrect++
		}
	}

	// Source-type accuracy.
	for _, pt := range ticks {
		stKey := strings.Join(pt.sourceTypes, "+")
		if majorityOption(sourceTypeToOptions[stKey]) == pt.bestOption {
			sourceTypeCorrect++
		}
	}

	// Build template list.
	var templates []ProbeTemplate
	for tk, count := range templateCounts {
		templates = append(templates, ProbeTemplate{
			Family:       family,
			ProsePattern: tk.pattern,
			BestOption:   tk.option,
			Occurrences:  count,
		})
	}

	avgOptions := float64(totalOptions) / float64(n)
	randomAcc := randomCorrect / float64(n)
	keywordAcc := float64(keywordCorrect) / float64(n)
	templateAcc := float64(templateCorrect) / float64(n)
	skeletonAcc := float64(skeletonCorrect) / float64(n)
	sourceTypeAcc := float64(sourceTypeCorrect) / float64(n)

	// Majority class accuracy: the best you could do by always guessing
	// the most common answer in this family (no prose information needed).
	answerCounts := make(map[string]int)
	for _, pt := range ticks {
		answerCounts[pt.bestOption]++
	}
	majorityCount := 0
	for _, c := range answerCounts {
		if c > majorityCount {
			majorityCount = c
		}
	}
	majorityAcc := float64(majorityCount) / float64(n)

	// Leakage is based on keyword and skeleton above the majority baseline.
	// Using majority (not random) ensures answer-class imbalance doesn't
	// inflate leakage — e.g., if "exploit" is always correct, that's a
	// balance issue, not a prose leak.
	leakage := keywordAcc - majorityAcc
	if skeletonAcc-majorityAcc > leakage {
		leakage = skeletonAcc - majorityAcc
	}
	if leakage < 0 {
		leakage = 0
	}

	return ProbeFamily{
		Ticks:              n,
		OptionCount:        avgOptions,
		RandomAccuracy:     randomAcc,
		KeywordHits:        keywordCorrect,
		KeywordAccuracy:    keywordAcc,
		TemplateDistinct:   len(patternToOptions),
		TemplateAccuracy:   templateAcc,
		SkeletonDistinct:   len(skeletonToOptions),
		SkeletonAccuracy:   skeletonAcc,
		SourceTypeAccuracy: sourceTypeAcc,
		MajorityAccuracy:   majorityAcc,
		LeakageScore:       leakage,
	}, templates
}

func probeSourceType(ticks []probeableTick) ProbeSource {
	n := len(ticks)
	if n == 0 {
		return ProbeSource{}
	}

	keywordCorrect := 0
	patternToOptions := make(map[string]map[string]int)
	for _, pt := range ticks {
		if keywordProbeCorrect(pt) {
			keywordCorrect++
		}
		pattern := normalizeProsePattern(pt.proseTexts)
		if patternToOptions[pattern] == nil {
			patternToOptions[pattern] = make(map[string]int)
		}
		patternToOptions[pattern][pt.bestOption]++
	}

	templateCorrect := 0
	for _, pt := range ticks {
		pattern := normalizeProsePattern(pt.proseTexts)
		if majorityOption(patternToOptions[pattern]) == pt.bestOption {
			templateCorrect++
		}
	}

	return ProbeSource{
		Ticks:            n,
		KeywordAccuracy:  float64(keywordCorrect) / float64(n),
		TemplateAccuracy: float64(templateCorrect) / float64(n),
	}
}

// keywordProbeCorrect checks if naive keyword matching between prose and
// option labels would pick the correct answer.
func keywordProbeCorrect(pt probeableTick) bool {
	prose := strings.ToLower(strings.Join(pt.proseTexts, " "))

	// Score each option by how many of its distinctive tokens appear in prose.
	bestScore := 0 // require at least 1 keyword hit to count as a match
	bestOption := ""
	tied := false
	for _, option := range pt.allOptions {
		score := keywordScore(prose, option, pt.allOptions)
		if score > bestScore {
			bestScore = score
			bestOption = option
			tied = false
		} else if score == bestScore && score > 0 {
			tied = true // multiple options with same score — ambiguous
		}
	}
	if tied || bestScore == 0 {
		return false // no clear keyword winner
	}
	return bestOption == pt.bestOption
}

// keywordScore counts how many tokens in the option label appear in the prose
// but NOT in competing option labels.
func keywordScore(prose, option string, allOptions []string) int {
	optionTokens := tokenize(option)
	competingTokens := make(map[string]struct{})
	for _, other := range allOptions {
		if other == option {
			continue
		}
		for _, t := range tokenize(other) {
			competingTokens[t] = struct{}{}
		}
	}

	score := 0
	for _, token := range optionTokens {
		if _, competing := competingTokens[token]; competing {
			continue
		}
		if strings.Contains(prose, token) {
			score++
		}
	}

	// Also check for semantic neighbors: common related words.
	for _, related := range relatedTerms(option) {
		if strings.Contains(prose, related) {
			score++
		}
	}
	return score
}

// normalizeProsePattern reduces prose to a structural fingerprint by:
// 1. Lowercasing
// 2. Replacing numbers with #
// 3. Collapsing whitespace
// This preserves fill words (faction names, districts, etc.) so each
// cluster's prose produces a distinct fingerprint. Template accuracy
// measures instance-level memorization (the learning target).
func normalizeProsePattern(texts []string) string {
	combined := strings.ToLower(strings.Join(texts, " | "))
	var b strings.Builder
	for _, r := range combined {
		if r >= '0' && r <= '9' {
			b.WriteRune('#')
		} else {
			b.WriteRune(r)
		}
	}
	result := b.String()
	for strings.Contains(result, "##") {
		result = strings.ReplaceAll(result, "##", "#")
	}
	return strings.TrimSpace(result)
}

// Known fill words from the devgen vocabulary. These are the proper nouns
// and game-specific terms that vary per cluster but don't carry answer info.
var skeletonStripWords = func() map[string]struct{} {
	words := make(map[string]struct{})
	for _, w := range []string{
		// colors (11)
		"green", "amber", "saffron", "ivory", "cobalt", "scarlet", "silver", "ashen", "vermilion", "cerulean", "ochre",
		// phenomena (13)
		"rain", "fog", "dust", "static", "hail", "glow", "drift", "mire", "tremor", "vapor", "ash", "frost", "flux",
		// roles (8)
		"broker", "warden", "auditor", "carrier", "scribe", "factor", "porter", "binder",
		// materials (11)
		"glass", "salt", "wire", "resin", "silk", "amber", "basalt", "signal", "copper", "bone", "lacquer",
		// aliases (individual words from compound aliases)
		"anchor", "choirmark", "resonance", "seal", "relay", "shard", "storm", "docket", "proof", "reed",
		"ghost", "writ", "marker", "tide", "cipher", "route", "blind", "receipt", "vault", "echo", "line", "trace", "bond",
		// districts (individual words from compound districts)
		"southern", "ward", "north", "quay", "mirror", "steps", "row", "silt", "exchange", "archive", "annex", "ember", "causeway", "river", "stairs",
		"gate", "basin", "flat", "chimney", "court", "alley",
		// factions (individual words from compound names)
		"choir", "civic", "harbor", "union", "office", "guild", "terrace",
		// hazards (individual words from compound names)
		"containment", "wash", "firebreak", "spore", "bloom", "spill", "fracture", "collapse", "cascade", "quake", "burn", "breach", "lock",
		// work types (11)
		"cleanup", "escort", "ledger", "sorting", "inspection", "repair", "triage", "registry", "survey", "dispatch", "stocktake",
		// protocols
		"curtain", "cordon", "brace", "checksum", "divert",
	} {
		words[w] = struct{}{}
	}
	return words
}()

// normalizeSkeletonPattern aggressively strips fill words to reveal only
// the template structure. This measures whether the prose skeleton alone
// (without learning cluster-specific fill words) predicts the answer.
func normalizeSkeletonPattern(texts []string) string {
	combined := strings.ToLower(strings.Join(texts, " | "))
	tokens := strings.Fields(combined)
	var kept []string
	for _, tok := range tokens {
		// Strip punctuation for matching.
		clean := strings.Trim(tok, ".,;:!?'\"()-")
		// Split hyphenated compounds and replace each stripped part with _.
		if strings.Contains(clean, "-") {
			parts := strings.Split(clean, "-")
			var rebuilt []string
			for _, part := range parts {
				if _, strip := skeletonStripWords[part]; strip {
					rebuilt = append(rebuilt, "_")
				} else {
					rebuilt = append(rebuilt, part)
				}
			}
			kept = append(kept, strings.Join(rebuilt, "-"))
		} else if _, strip := skeletonStripWords[clean]; strip {
			kept = append(kept, "_")
		} else {
			kept = append(kept, tok)
		}
	}
	result := strings.Join(kept, " ")
	// Collapse consecutive placeholders.
	for strings.Contains(result, "_ _") {
		result = strings.ReplaceAll(result, "_ _", "_")
	}
	// Replace numbers.
	var b strings.Builder
	for _, r := range result {
		if r >= '0' && r <= '9' {
			b.WriteRune('#')
		} else {
			b.WriteRune(r)
		}
	}
	result = b.String()
	for strings.Contains(result, "##") {
		result = strings.ReplaceAll(result, "##", "#")
	}
	return strings.TrimSpace(result)
}

func tokenize(s string) []string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "_", " ")
	return strings.Fields(s)
}

// relatedTerms maps option labels to semantically related words that
// might appear in prose as indirect hints.
func relatedTerms(option string) []string {
	related := map[string][]string{
		"broker":     {"intermediar", "middl", "brokerage", "suppressed", "suppression", "redirect"},
		"penitent":   {"contrit", "atonem", "penitenc", "quarantine", "contrition", "remors"},
		"auditor":    {"audit", "certified", "verificat", "provenance", "checksum", "ledger"},
		"quarantine": {"quarantine", "containm", "isolat", "exposure", "contaminat"},
		"auction":    {"auction", "bid", "certif", "settling", "clear"},
		"stabilize":  {"stabiliz", "contain", "protocol", "brace", "shield"},
		"exploit":    {"exploit", "surge", "risk", "opportun", "upside"},
	}
	return related[option]
}

func majorityOption(counts map[string]int) string {
	best := ""
	bestN := 0
	for opt, n := range counts {
		if n > bestN {
			best = opt
			bestN = n
		}
	}
	return best
}

func buildProbeSummary(probeable []probeableTick, families map[string]ProbeFamily) ProbeSummary {
	n := len(probeable)
	if n == 0 {
		return ProbeSummary{Verdict: "no probeable ticks"}
	}

	totalRandom := 0.0
	totalKeyword := 0
	totalTemplate := 0
	totalSkeleton := 0
	for _, pt := range probeable {
		totalRandom += 1.0 / float64(len(pt.allOptions))
		if keywordProbeCorrect(pt) {
			totalKeyword++
		}
	}

	// For overall template accuracy, rebuild a global pattern map.
	globalPatterns := make(map[string]map[string]int)
	for _, pt := range probeable {
		pattern := normalizeProsePattern(pt.proseTexts)
		if globalPatterns[pattern] == nil {
			globalPatterns[pattern] = make(map[string]int)
		}
		globalPatterns[pattern][pt.bestOption]++
	}
	for _, pt := range probeable {
		pattern := normalizeProsePattern(pt.proseTexts)
		if majorityOption(globalPatterns[pattern]) == pt.bestOption {
			totalTemplate++
		}
	}

	// For overall skeleton accuracy, rebuild a global skeleton map.
	globalSkeletons := make(map[string]map[string]int)
	for _, pt := range probeable {
		skeleton := normalizeSkeletonPattern(pt.proseTexts)
		if globalSkeletons[skeleton] == nil {
			globalSkeletons[skeleton] = make(map[string]int)
		}
		globalSkeletons[skeleton][pt.bestOption]++
	}
	for _, pt := range probeable {
		skeleton := normalizeSkeletonPattern(pt.proseTexts)
		if majorityOption(globalSkeletons[skeleton]) == pt.bestOption {
			totalSkeleton++
		}
	}

	worstFamily := ""
	worstLeakage := 0.0
	for family, pf := range families {
		if pf.LeakageScore > worstLeakage {
			worstLeakage = pf.LeakageScore
			worstFamily = family
		}
	}

	overallRandom := totalRandom / float64(n)
	overallKeyword := float64(totalKeyword) / float64(n)
	overallTemplate := float64(totalTemplate) / float64(n)
	overallSkeleton := float64(totalSkeleton) / float64(n)

	verdict := "OK"
	if worstLeakage > 0.3 {
		verdict = fmt.Sprintf("SEVERE LEAKAGE in %s (%.0f%% above random)", worstFamily, worstLeakage*100)
	} else if worstLeakage > 0.15 {
		verdict = fmt.Sprintf("MODERATE LEAKAGE in %s (%.0f%% above random)", worstFamily, worstLeakage*100)
	} else if worstLeakage > 0.05 {
		verdict = fmt.Sprintf("MILD LEAKAGE in %s (%.0f%% above random)", worstFamily, worstLeakage*100)
	}

	return ProbeSummary{
		TotalProbeable:          n,
		OverallRandomAccuracy:   overallRandom,
		OverallKeywordAccuracy:  overallKeyword,
		OverallTemplateAccuracy: overallTemplate,
		OverallSkeletonAccuracy: overallSkeleton,
		WorstFamily:             worstFamily,
		WorstFamilyLeakage:     worstLeakage,
		Verdict:                 verdict,
	}
}
