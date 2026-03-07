package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/mihai/mad/internal/game"
	"github.com/mihai/mad/internal/season"
)

func main() {
	seasonPath := flag.String("season", filepath.Join("seasons", "dev", "season.json"), "Path to season JSON")
	outDir := flag.String("out", "build", "Output directory")
	flag.Parse()

	loadedSeason, err := season.LoadFile(*seasonPath)
	if err != nil {
		log.Fatalf("load season: %v", err)
	}

	ticksDir := filepath.Join(*outDir, "ticks")
	if err := os.MkdirAll(ticksDir, 0o755); err != nil {
		log.Fatalf("mkdir ticks dir: %v", err)
	}

	manifest := game.Manifest{
		SeasonID:        loadedSeason.SeasonID,
		SchemaVersion:   loadedSeason.SchemaVersion,
		ScoreEpochTicks: loadedSeason.ScoreEpochTicks,
		RevealLagTicks:  loadedSeason.RevealLagTicks,
		HashShardCount:  loadedSeason.ShardCount,
	}
	if err := writeJSON(filepath.Join(*outDir, "manifest.json"), manifest); err != nil {
		log.Fatalf("write manifest: %v", err)
	}

	for _, tick := range loadedSeason.Ticks {
		if err := writeJSON(filepath.Join(ticksDir, tick.TickID+".json"), tick.Public()); err != nil {
			log.Fatalf("write tick %s: %v", tick.TickID, err)
		}
	}

	log.Printf("compiled %d public tick artifacts into %s", len(loadedSeason.Ticks), *outDir)
}

func writeJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(path, raw, 0o644)
}
