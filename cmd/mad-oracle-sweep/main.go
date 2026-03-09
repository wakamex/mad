package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mihai/mad/internal/season"
	"github.com/mihai/mad/internal/storage"
)

type oracleConfig struct {
	Horizon int `json:"horizon"`
	Beam    int `json:"beam"`
}

type oracleRun struct {
	Horizon         int     `json:"horizon"`
	Beam            int     `json:"beam"`
	OracleName      string  `json:"oracle_name"`
	OracleScore     int64   `json:"oracle_score"`
	GreedyScore     int64   `json:"greedy_score"`
	VisibleScore    int64   `json:"visible_greedy_score"`
	AlwaysHoldScore int64   `json:"always_hold_score"`
	GainOverGreedy  int64   `json:"gain_over_greedy"`
	GainOverVisible int64   `json:"gain_over_visible"`
	WallMS          int64   `json:"wall_ms"`
	TicksPerSecond  float64 `json:"ticks_per_second"`
	ParetoOptimal   bool    `json:"pareto_optimal,omitempty"`
}

type oracleSweepReport struct {
	GeneratedAt string         `json:"generated_at"`
	SeasonPath  string         `json:"season_path"`
	SeasonID    string         `json:"season_id"`
	Title       string         `json:"title"`
	TickCount   int            `json:"tick_count"`
	Configs     []oracleConfig `json:"configs"`
	Runs        []oracleRun    `json:"runs"`
}

func main() {
	seasonPath := flag.String("season", filepath.Join("seasons", "dev1000", "season.json"), "Path to compiled season JSON")
	outPath := flag.String("out", filepath.Join("benchmarks", "oracle-sweep", "dev1000.json"), "Path to JSON sweep report")
	configsRaw := flag.String("configs", "16x8,32x8,32x16,64x8,64x16,64x32,96x16,96x32,128x16,128x32", "Comma-separated horizon x beam configs, e.g. 16x8,32x16")
	flag.Parse()

	configs, err := parseOracleConfigs(*configsRaw)
	if err != nil {
		log.Fatalf("parse configs: %v", err)
	}

	file, err := season.LoadFile(*seasonPath)
	if err != nil {
		log.Fatalf("load season: %v", err)
	}

	report := oracleSweepReport{
		GeneratedAt: time.Now().Format(time.RFC3339),
		SeasonPath:  *seasonPath,
		SeasonID:    file.SeasonID,
		Title:       file.Title,
		TickCount:   len(file.Ticks),
		Configs:     configs,
		Runs:        make([]oracleRun, 0, len(configs)),
	}

	for _, cfg := range configs {
		start := time.Now()
		sim, err := season.SimulateWithOptions(file, season.SimulationOptions{
			OracleHorizon:   cfg.Horizon,
			OracleBeamWidth: cfg.Beam,
		})
		if err != nil {
			log.Fatalf("simulate h=%d b=%d: %v", cfg.Horizon, cfg.Beam, err)
		}
		elapsed := time.Since(start)

		oracleName := fmt.Sprintf("oracle_h%d_b%d", cfg.Horizon, cfg.Beam)
		oracleBaseline, ok := sim.Baselines[oracleName]
		if !ok {
			log.Fatalf("missing oracle baseline %s", oracleName)
		}
		greedy := sim.Baselines["greedy_best"].Ledger.Score
		visible := sim.Baselines["visible_greedy"].Ledger.Score
		hold := sim.Baselines["always_hold"].Ledger.Score

		report.Runs = append(report.Runs, oracleRun{
			Horizon:         cfg.Horizon,
			Beam:            cfg.Beam,
			OracleName:      oracleName,
			OracleScore:     oracleBaseline.Ledger.Score,
			GreedyScore:     greedy,
			VisibleScore:    visible,
			AlwaysHoldScore: hold,
			GainOverGreedy:  oracleBaseline.Ledger.Score - greedy,
			GainOverVisible: oracleBaseline.Ledger.Score - visible,
			WallMS:          elapsed.Milliseconds(),
			TicksPerSecond:  float64(len(file.Ticks)) / elapsed.Seconds(),
		})
	}

	markParetoOptimal(report.Runs)

	if err := storage.SaveJSON(*outPath, report); err != nil {
		log.Fatalf("write json report: %v", err)
	}
	mdPath := strings.TrimSuffix(*outPath, filepath.Ext(*outPath)) + ".md"
	if err := saveMarkdown(mdPath, report); err != nil {
		log.Fatalf("write markdown report: %v", err)
	}

	log.Printf("oracle sweep saved to %s and %s", *outPath, mdPath)
}

func parseOracleConfigs(raw string) ([]oracleConfig, error) {
	parts := strings.Split(raw, ",")
	configs := make([]oracleConfig, 0, len(parts))
	seen := make(map[oracleConfig]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		fields := strings.Split(strings.ToLower(part), "x")
		if len(fields) != 2 {
			return nil, fmt.Errorf("invalid config %q", part)
		}
		h, err := strconv.Atoi(fields[0])
		if err != nil || h <= 0 {
			return nil, fmt.Errorf("invalid horizon in %q", part)
		}
		b, err := strconv.Atoi(fields[1])
		if err != nil || b <= 0 {
			return nil, fmt.Errorf("invalid beam in %q", part)
		}
		cfg := oracleConfig{Horizon: h, Beam: b}
		if _, ok := seen[cfg]; ok {
			continue
		}
		seen[cfg] = struct{}{}
		configs = append(configs, cfg)
	}
	if len(configs) == 0 {
		return nil, errors.New("no valid configs")
	}
	return configs, nil
}

func markParetoOptimal(runs []oracleRun) {
	type idxRun struct {
		Index int
		Run   oracleRun
	}
	sorted := make([]idxRun, len(runs))
	for i, run := range runs {
		sorted[i] = idxRun{Index: i, Run: run}
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Run.WallMS != sorted[j].Run.WallMS {
			return sorted[i].Run.WallMS < sorted[j].Run.WallMS
		}
		return sorted[i].Run.OracleScore > sorted[j].Run.OracleScore
	})
	bestScore := int64(-1 << 62)
	for _, item := range sorted {
		if item.Run.OracleScore > bestScore {
			runs[item.Index].ParetoOptimal = true
			bestScore = item.Run.OracleScore
		}
	}
}

func saveMarkdown(path string, report oracleSweepReport) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# Oracle Sweep\n\n")
	fmt.Fprintf(&b, "- Season: `%s` (%s)\n", report.SeasonID, report.Title)
	fmt.Fprintf(&b, "- Ticks: `%d`\n", report.TickCount)
	fmt.Fprintf(&b, "- Generated: `%s`\n\n", report.GeneratedAt)
	b.WriteString("| Horizon | Beam | Score | Gain vs Greedy | Gain vs Visible | Wall | Ticks/s | Pareto |\n")
	b.WriteString("|---:|---:|---:|---:|---:|---:|---:|:---:|\n")
	for _, run := range report.Runs {
		fmt.Fprintf(&b, "| %d | %d | %d | %d | %d | %.2fs | %.2f | %s |\n",
			run.Horizon,
			run.Beam,
			run.OracleScore,
			run.GainOverGreedy,
			run.GainOverVisible,
			float64(run.WallMS)/1000.0,
			run.TicksPerSecond,
			boolMark(run.ParetoOptimal),
		)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func boolMark(v bool) string {
	if v {
		return "yes"
	}
	return ""
}
