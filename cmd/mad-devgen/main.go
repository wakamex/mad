package main

import (
	"flag"
	"log"
	"path/filepath"

	"github.com/mihai/mad/internal/season"
	"github.com/mihai/mad/internal/storage"
)

func main() {
	tickCount := flag.Int("ticks", 1000, "Total tick count to generate; must be a multiple of 20 (full) or 10 (focused)")
	outPath := flag.String("out", filepath.Join("seasons", "dev1000", "season_ir.json"), "Path to output season IR JSON")
	focused := flag.Bool("focused", false, "Generate focused clue+payoff season only (no standing/ladder/hazard)")
	flag.Parse()

	var ir season.IRFile
	var err error
	if *focused {
		ir, err = season.BuildFocusedDevSeasonIR(*tickCount)
	} else {
		ir, err = season.BuildGeneratedDevSeasonIR(*tickCount)
	}
	if err != nil {
		log.Fatalf("generate dev season ir: %v", err)
	}

	if err := storage.SaveJSON(*outPath, ir); err != nil {
		log.Fatalf("write dev season ir: %v", err)
	}

	log.Printf("generated %d-beat dev season ir with %d story elements into %s", *tickCount, len(ir.Elements), *outPath)
}
