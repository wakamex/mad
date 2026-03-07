package main

import (
	"flag"
	"log"
	"path/filepath"

	"github.com/mihai/mad/internal/season"
	"github.com/mihai/mad/internal/storage"
)

func main() {
	irPath := flag.String("ir", filepath.Join("seasons", "dev", "season_ir.json"), "Path to season IR JSON")
	outPath := flag.String("out", filepath.Join("build", "season.json"), "Path to compiled season JSON")
	flag.Parse()

	loadedIR, err := season.LoadIRFile(*irPath)
	if err != nil {
		log.Fatalf("load season ir: %v", err)
	}

	compiled, err := season.CompileIR(loadedIR)
	if err != nil {
		log.Fatalf("compile season ir: %v", err)
	}

	if err := storage.SaveJSON(*outPath, compiled); err != nil {
		log.Fatalf("write compiled season: %v", err)
	}

	log.Printf("compiled %d beats from %d story elements into %s", len(compiled.Ticks), len(loadedIR.Elements), *outPath)
}
