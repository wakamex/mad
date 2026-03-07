package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"path/filepath"
	"sort"
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
	recentReveals := flag.Int("recent-reveals", 6, "Number of recent public reveals to include in prompts")
	maxNotesChars := flag.Int("max-notes-chars", 1600, "Maximum persisted notes length")
	decisionTimeout := flag.Duration("decision-timeout", 90*time.Second, "Timeout per model decision")
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

	runners := make([]harness.Runner, 0, len(specs))
	for _, spec := range specs {
		runner, err := harness.NewCLIRunner(spec, *workdir, "")
		if err != nil {
			log.Fatalf("create runner %s: %v", spec.Label(), err)
		}
		runners = append(runners, runner)
		defer runner.Close()
	}

	report := harness.SuiteReport{
		GeneratedAt: time.Now().UTC(),
	}
	ctx := context.Background()

	if *probeOnly {
		for _, runner := range runners {
			probe := harness.RunProbe(ctx, runner)
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
		RecentRevealCount: *recentReveals,
		MaxNotesChars:     *maxNotesChars,
		DecisionTimeout:   *decisionTimeout,
	}
	for _, runner := range runners {
		runIndex := len(report.Runs)
		report.Runs = append(report.Runs, harness.RunResult{
			Runner:      runner.Spec(),
			Session:     runner.SessionInfo(),
			SeasonID:    file.SeasonID,
			SeasonTitle: file.Title,
		})
		runOptions.StepCallback = func(partial harness.RunResult) error {
			report.GeneratedAt = time.Now().UTC()
			report.Runs[runIndex] = partial
			return storage.SaveJSON(*outPath, report)
		}
		result, err := harness.RunSeason(ctx, file, simReport, runner, runOptions)
		if err != nil {
			log.Fatalf("run %s: %v", runner.Spec().Label(), err)
		}
		report.GeneratedAt = time.Now().UTC()
		report.Runs[runIndex] = result
		if err := storage.SaveJSON(*outPath, report); err != nil {
			log.Fatalf("write harness report: %v", err)
		}
		log.Printf("run_summary %s", summarizeRun(result))
	}
	for _, run := range report.Runs {
		log.Printf("final_summary %s", summarizeRun(run))
	}
	log.Printf("run_report runners=%d out=%s", len(report.Runs), *outPath)
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
		"runner=%s score=%d steps=%d wall=%s avg_step=%s p50=%s p95=%s ticks_per_min=%.2f last_tick=%s errors=%d",
		result.Runner.Label(),
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
