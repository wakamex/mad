package main

import (
	"flag"
	"log"
	"path/filepath"

	"github.com/mihai/mad/internal/season"
	"github.com/mihai/mad/internal/storage"
)

func main() {
	seasonPath := flag.String("season", filepath.Join("seasons", "dev", "season.json"), "Path to compiled season JSON")
	outPath := flag.String("out", filepath.Join("build", "simulation.json"), "Path to simulation report JSON")
	randomRuns := flag.Int("random-runs", 10000, "Number of deterministic random legal-play simulations to run")
	randomSeed := flag.Int64("random-seed", 1, "Seed for deterministic random legal-play simulations")
	oracleHorizon := flag.Int("oracle-horizon", 16, "Lookahead horizon for the bounded oracle baseline")
	oracleBeamWidth := flag.Int("oracle-beam", 8, "Beam width for the bounded oracle baseline")
	failOnRandomWarnings := flag.Bool("fail-on-random-warnings", false, "Exit non-zero if the random-play audit produces warnings")
	flag.Parse()

	loadedSeason, err := season.LoadFile(*seasonPath)
	if err != nil {
		log.Fatalf("load season: %v", err)
	}

	report, err := season.SimulateWithOptions(loadedSeason, season.SimulationOptions{
		RandomRuns:      *randomRuns,
		RandomSeed:      *randomSeed,
		OracleHorizon:   *oracleHorizon,
		OracleBeamWidth: *oracleBeamWidth,
	})
	if err != nil {
		log.Fatalf("simulate season: %v", err)
	}

	if err := storage.SaveJSON(*outPath, report); err != nil {
		log.Fatalf("write simulation report: %v", err)
	}

	if *failOnRandomWarnings && report.RandomAudit != nil && len(report.RandomAudit.Warnings) > 0 {
		log.Fatalf("random audit warnings: %v", report.RandomAudit.Warnings)
	}

	log.Printf("simulated %s into %s", report.Summary(), *outPath)
}
