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
	flag.Parse()

	loadedSeason, err := season.LoadFile(*seasonPath)
	if err != nil {
		log.Fatalf("load season: %v", err)
	}

	report, err := season.Simulate(loadedSeason)
	if err != nil {
		log.Fatalf("simulate season: %v", err)
	}

	if err := storage.SaveJSON(*outPath, report); err != nil {
		log.Fatalf("write simulation report: %v", err)
	}

	log.Printf("simulated %s into %s", report.Summary(), *outPath)
}
