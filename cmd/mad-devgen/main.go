package main

import (
	"flag"
	"log"
	"path/filepath"

	"github.com/mihai/mad/internal/season"
	"github.com/mihai/mad/internal/storage"
)

func main() {
	tickCount := flag.Int("ticks", 1000, "Total tick count to generate; must be a multiple of 20")
	outPath := flag.String("out", filepath.Join("seasons", "dev1000", "season_ir.json"), "Path to output season IR JSON")
	flag.Parse()

	ir, err := season.BuildGeneratedDevSeasonIR(*tickCount)
	if err != nil {
		log.Fatalf("generate dev season ir: %v", err)
	}

	if err := storage.SaveJSON(*outPath, ir); err != nil {
		log.Fatalf("write dev season ir: %v", err)
	}

	log.Printf("generated %d-beat dev season ir with %d story elements into %s", *tickCount, len(ir.Elements), *outPath)
}
