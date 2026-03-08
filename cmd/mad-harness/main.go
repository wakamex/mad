package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mihai/mad/internal/harness"
	"github.com/mihai/mad/internal/season"
	"github.com/mihai/mad/internal/storage"
)

type runnerListFlag []string

func (f *runnerListFlag) String() string {
	return fmt.Sprintf("%v", []string(*f))
}

func (f *runnerListFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func main() {
	seasonPath := flag.String("season", filepath.Join("seasons", "dev1000", "season.json"), "Path to compiled season JSON")
	outPath := flag.String("out", filepath.Join("build", "harness.json"), "Path to harness report JSON")
	workdir := flag.String("workdir", ".", "Working directory for external runner CLIs")
	startTick := flag.Int("start-tick", 0, "Tick index to start from")
	maxTicks := flag.Int("max-ticks", 25, "Maximum ticks to play; 0 means entire season")
	recentReveals := flag.Int("recent-reveals", -1, "Number of recent public reveals to include in prompts; -1 means auto")
	maxNotesChars := flag.Int("max-notes-chars", 1600, "Maximum persisted notes length")
	decisionTimeout := flag.Duration("decision-timeout", 90*time.Second, "Timeout per model decision")
	runCount := flag.Int("runs", 1, "Number of independent runs per runner")
	memoryModeRaw := flag.String("memory", string(harness.MemoryModeInherit), "Native memory mode for runners: inherit, on, or off")
	contextModeRaw := flag.String("context", string(harness.ContextModePersistent), "Context continuity mode for runners: persistent or ephemeral")
	serviceTierRaw := flag.String("service-tier", string(harness.ServiceTierInherit), "Codex service tier for runners: inherit, fast, or flex")
	probeOnly := flag.Bool("probe", false, "Probe runner/model availability without playing a season")
	var runnerSpecs runnerListFlag
	flag.Var(&runnerSpecs, "runner", "Runner spec provider:model[@effort]; may be repeated")
	flag.Parse()

	memoryMode, err := harness.ParseMemoryMode(*memoryModeRaw)
	if err != nil {
		log.Fatalf("parse memory mode: %v", err)
	}
	contextMode, err := harness.ParseContextMode(*contextModeRaw)
	if err != nil {
		log.Fatalf("parse context mode: %v", err)
	}
	serviceTier, err := harness.ParseServiceTier(*serviceTierRaw)
	if err != nil {
		log.Fatalf("parse service tier: %v", err)
	}
	recentRevealCount := *recentReveals
	if recentRevealCount < 0 {
		recentRevealCount = defaultRecentReveals(memoryMode, contextMode)
	}
	if *runCount <= 0 {
		log.Fatalf("parse runs: must be >= 1")
	}

	specs := make([]harness.RunnerSpec, 0, len(runnerSpecs))
	if len(runnerSpecs) == 0 {
		specs = append(specs, harness.DefaultRunnerSpecs()...)
	} else {
		for _, raw := range runnerSpecs {
			spec, err := harness.ParseRunnerSpec(raw)
			if err != nil {
				log.Fatalf("parse runner %q: %v", raw, err)
			}
			specs = append(specs, spec)
		}
	}
	for i := range specs {
		specs[i].MemoryMode = memoryMode
		specs[i].ContextMode = contextMode
		specs[i].ServiceTier = serviceTier
	}
	for _, spec := range specs {
		for _, warning := range harness.RunnerWarnings(spec) {
			log.Printf("runner_warning %s", warning)
		}
	}

	report := harness.SuiteReport{
		GeneratedAt: time.Now().UTC(),
	}
	ctx := context.Background()

	if *probeOnly {
		for _, spec := range specs {
			runner, err := harness.NewCLIRunner(spec, *workdir, runnerRoot(*outPath, spec, 0))
			if err != nil {
				log.Fatalf("create runner %s: %v", spec.Label(), err)
			}
			probe := harness.RunProbe(ctx, runner)
			_ = runner.Close()
			report.Probes = append(report.Probes, probe)
			log.Printf("probe_summary runner=%s ok=%t duration=%s error=%q", probe.Runner.Label(), probe.OK, formatDurationMS(probe.DurationMS), probe.Error)
		}
		if err := storage.SaveJSON(*outPath, report); err != nil {
			log.Fatalf("write probe report: %v", err)
		}
		log.Printf("probe_report runners=%d out=%s", len(report.Probes), *outPath)
		return
	}

	file, err := season.LoadFile(*seasonPath)
	if err != nil {
		log.Fatalf("load season: %v", err)
	}
	simReport, err := season.Simulate(file)
	if err != nil {
		log.Fatalf("simulate season: %v", err)
	}

	report.SeasonPath = *seasonPath
	report.SeasonID = file.SeasonID
	report.SeasonTitle = file.Title
	if err := storage.SaveJSON(*outPath, report); err != nil {
		log.Fatalf("write initial harness report: %v", err)
	}
	runOptions := harness.RunOptions{
		StartTick:         *startTick,
		MaxTicks:          *maxTicks,
		RecentRevealCount: recentRevealCount,
		MaxNotesChars:     *maxNotesChars,
		DecisionTimeout:   *decisionTimeout,
	}
	for _, spec := range specs {
		for runNumber := 1; runNumber <= *runCount; runNumber++ {
			runner, err := harness.NewCLIRunner(spec, *workdir, runnerRoot(*outPath, spec, runNumber))
			if err != nil {
				log.Fatalf("create runner %s: %v", spec.Label(), err)
			}

			progress := &progressPrinter{}
			totalTicks := plannedTickCount(runOptions, len(file.Ticks))
			runIndex := len(report.Runs)
			report.Runs = append(report.Runs, harness.RunResult{
				Runner:      spec,
				RunNumber:   runNumber,
				RunCount:    *runCount,
				Session:     runner.SessionInfo(),
				SeasonID:    file.SeasonID,
				SeasonTitle: file.Title,
			})
			progress.Start(report.Runs[runIndex], totalTicks)
			runOptions.StepCallback = func(partial harness.RunResult) error {
				partial.RunNumber = runNumber
				partial.RunCount = *runCount
				report.GeneratedAt = time.Now().UTC()
				report.Runs[runIndex] = partial
				if err := storage.SaveJSON(*outPath, report); err != nil {
					return err
				}
				progress.Update(partial)
				return nil
			}

			result, err := harness.RunSeason(ctx, file, simReport, runner, runOptions)
			progress.Finish()
			_ = runner.Close()
			if err != nil {
				log.Fatalf("run %s: %v", spec.Label(), err)
			}

			result.RunNumber = runNumber
			result.RunCount = *runCount
			report.GeneratedAt = time.Now().UTC()
			report.Runs[runIndex] = result
			report.RunGroups = summarizeRunGroups(report.Runs)
			if err := storage.SaveJSON(*outPath, report); err != nil {
				log.Fatalf("write harness report: %v", err)
			}
			log.Printf("run_summary %s", summarizeRun(result))
		}
	}
	for _, run := range report.Runs {
		log.Printf("final_summary %s", summarizeRun(run))
	}
	for _, group := range report.RunGroups {
		log.Printf("multi_run_summary %s", summarizeRunGroup(group))
	}
	log.Printf("run_report runners=%d out=%s", len(report.Runs), *outPath)
}

func defaultRecentReveals(memoryMode harness.MemoryMode, contextMode harness.ContextMode) int {
	if memoryMode == harness.MemoryModeOff && contextMode == harness.ContextModeEphemeral {
		return 0
	}
	return 6
}

type progressPrinter struct {
	lastWidth int
}

func (p *progressPrinter) Start(result harness.RunResult, total int) {
	fmt.Fprintf(
		os.Stderr,
		"run_start runner=%s run=%d/%d season=%s ticks=%d\n",
		result.Runner.Label(),
		max(result.RunNumber, 1),
		max(result.RunCount, 1),
		result.SeasonID,
		total,
	)
}

func (p *progressPrinter) Update(result harness.RunResult) {
	line := formatProgressLine(result)
	if len(line) < p.lastWidth {
		line += strings.Repeat(" ", p.lastWidth-len(line))
	}
	if len(line) > p.lastWidth {
		p.lastWidth = len(line)
	}
	fmt.Fprintf(os.Stderr, "\r%s", line)
}

func (p *progressPrinter) Finish() {
	if p.lastWidth == 0 {
		return
	}
	fmt.Fprintln(os.Stderr)
	p.lastWidth = 0
}

func summarizeRun(result harness.RunResult) string {
	avg, p50, p95 := stepDurationStats(result.Steps)
	ticksPerMinute := 0.0
	wallMS := result.CompletedAt.Sub(result.StartedAt).Milliseconds()
	if wallMS > 0 && result.StepCount > 0 {
		ticksPerMinute = float64(result.StepCount) / (float64(wallMS) / float64(time.Minute/time.Millisecond))
	}
	lastTick := ""
	if len(result.ScoreTrace) > 0 {
		lastTick = result.ScoreTrace[len(result.ScoreTrace)-1].TickID
	}
	return fmt.Sprintf(
		"runner=%s run=%d/%d score=%d steps=%d wall=%s avg_step=%s p50=%s p95=%s ticks_per_min=%.2f last_tick=%s errors=%d",
		result.Runner.Label(),
		max(result.RunNumber, 1),
		max(result.RunCount, 1),
		result.FinalScore.Score,
		result.StepCount,
		result.CompletedAt.Sub(result.StartedAt).Round(time.Millisecond),
		avg,
		p50,
		p95,
		ticksPerMinute,
		lastTick,
		len(result.Errors),
	)
}

func runnerRoot(outPath string, spec harness.RunnerSpec, runNumber int) string {
	root := filepath.Join(filepath.Dir(outPath), "runner-state", slugify(spec.Label()))
	if runNumber <= 0 {
		return filepath.Join(root, "probe")
	}
	return filepath.Join(root, fmt.Sprintf("run-%03d", runNumber))
}

func slugify(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	replacer := strings.NewReplacer(
		"/", "-",
		":", "-",
		"@", "-",
		"+", "-",
		".", "-",
		" ", "-",
	)
	slug := replacer.Replace(raw)
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "runner"
	}
	return slug
}

func summarizeRunGroups(runs []harness.RunResult) []harness.RunGroup {
	if len(runs) == 0 {
		return nil
	}
	grouped := make(map[string][]harness.RunResult)
	order := make([]string, 0, len(runs))
	for _, run := range runs {
		key := run.Runner.Label()
		if _, ok := grouped[key]; !ok {
			order = append(order, key)
		}
		grouped[key] = append(grouped[key], run)
	}

	groups := make([]harness.RunGroup, 0, len(grouped))
	for _, key := range order {
		groupRuns := grouped[key]
		scores := make([]int64, 0, len(groupRuns))
		var wallTotal int64
		var tpmTotal float64
		minScore := int64(0)
		maxScore := int64(0)
		for i, run := range groupRuns {
			score := run.FinalScore.Score
			if i == 0 || score < minScore {
				minScore = score
			}
			if i == 0 || score > maxScore {
				maxScore = score
			}
			scores = append(scores, score)
			wallTotal += run.CompletedAt.Sub(run.StartedAt).Milliseconds()
			tpmTotal += ticksPerMinute(run)
		}
		groups = append(groups, harness.RunGroup{
			Runner:          groupRuns[0].Runner,
			RunCount:        len(groupRuns),
			MeanScore:       meanInt64(scores),
			MedianScore:     percentileInt64(scores, 0.50),
			P90Score:        percentileInt64(scores, 0.90),
			MinScore:        minScore,
			MaxScore:        maxScore,
			MeanWallMS:      wallTotal / int64(len(groupRuns)),
			MeanTicksPerMin: tpmTotal / float64(len(groupRuns)),
		})
	}
	return groups
}

func summarizeRunGroup(group harness.RunGroup) string {
	return fmt.Sprintf(
		"runner=%s runs=%d mean_score=%.2f median_score=%.2f p90_score=%.2f min_score=%d max_score=%d mean_wall=%s mean_ticks_per_min=%.2f",
		group.Runner.Label(),
		group.RunCount,
		group.MeanScore,
		group.MedianScore,
		group.P90Score,
		group.MinScore,
		group.MaxScore,
		time.Duration(group.MeanWallMS)*time.Millisecond,
		group.MeanTicksPerMin,
	)
}

func formatProgressLine(result harness.RunResult) string {
	total := result.EndTick - result.StartTick
	done := result.StepCount
	if total < 0 {
		total = 0
	}
	progressPct := 100.0
	if total > 0 {
		progressPct = 100 * float64(done) / float64(total)
	}
	elapsed := result.CompletedAt.Sub(result.StartedAt)
	if elapsed < 0 {
		elapsed = 0
	}
	avgStep := time.Duration(0)
	if done > 0 {
		avgStep = elapsed / time.Duration(done)
	}
	remaining := total - done
	if remaining < 0 {
		remaining = 0
	}
	eta := time.Duration(0)
	if done > 0 && remaining > 0 {
		eta = time.Duration(float64(elapsed) * (float64(remaining) / float64(done)))
	}
	lastDelta := int64(0)
	if len(result.Steps) > 0 {
		lastDelta = result.Steps[len(result.Steps)-1].Outcome.ScoreDelta
	}
	lastTick := ""
	if len(result.ScoreTrace) > 0 {
		lastTick = result.ScoreTrace[len(result.ScoreTrace)-1].TickID
	}
	return fmt.Sprintf(
		"%d/%d %5.1f%% score=%d d=%+d avg=%s eta=%s err=%d last=%s",
		done,
		total,
		progressPct,
		result.FinalScore.Score,
		lastDelta,
		avgStep.Round(time.Millisecond),
		eta.Round(time.Second),
		len(result.Errors),
		lastTick,
	)
}

func plannedTickCount(options harness.RunOptions, tickCount int) int {
	startTick := options.StartTick
	if startTick < 0 {
		startTick = 0
	}
	if startTick > tickCount {
		startTick = tickCount
	}
	endTick := tickCount
	if options.MaxTicks > 0 && startTick+options.MaxTicks < endTick {
		endTick = startTick + options.MaxTicks
	}
	if endTick < startTick {
		endTick = startTick
	}
	return endTick - startTick
}

func ticksPerMinute(result harness.RunResult) float64 {
	wallMS := result.CompletedAt.Sub(result.StartedAt).Milliseconds()
	if wallMS <= 0 || result.StepCount <= 0 {
		return 0
	}
	return float64(result.StepCount) / (float64(wallMS) / float64(time.Minute/time.Millisecond))
}

func percentileInt64(values []int64, q float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int64(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	if q <= 0 {
		return float64(sorted[0])
	}
	if q >= 1 {
		return float64(sorted[len(sorted)-1])
	}
	index := int(math.Ceil(q*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return float64(sorted[index])
}

func meanInt64(values []int64) float64 {
	if len(values) == 0 {
		return 0
	}
	var total int64
	for _, value := range values {
		total += value
	}
	return float64(total) / float64(len(values))
}

func stepDurationStats(steps []harness.StepTrace) (time.Duration, time.Duration, time.Duration) {
	if len(steps) == 0 {
		return 0, 0, 0
	}
	durations := make([]int64, 0, len(steps))
	var total int64
	for _, step := range steps {
		if step.DurationMS < 0 {
			continue
		}
		durations = append(durations, step.DurationMS)
		total += step.DurationMS
	}
	if len(durations) == 0 {
		return 0, 0, 0
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	avg := time.Duration(total/int64(len(durations))) * time.Millisecond
	return avg, percentileMS(durations, 0.50), percentileMS(durations, 0.95)
}

func percentileMS(durations []int64, q float64) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	if q <= 0 {
		return time.Duration(durations[0]) * time.Millisecond
	}
	if q >= 1 {
		return time.Duration(durations[len(durations)-1]) * time.Millisecond
	}
	index := int(math.Ceil(q*float64(len(durations)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(durations) {
		index = len(durations) - 1
	}
	return time.Duration(durations[index]) * time.Millisecond
}

func formatDurationMS(ms int64) time.Duration {
	if ms <= 0 {
		return 0
	}
	return time.Duration(ms) * time.Millisecond
}
