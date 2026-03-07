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

	audit := season.AuditIR(loadedIR)

	compiled, err := season.CompileIR(loadedIR)
	if err != nil {
		log.Fatalf("compile season ir: %v", err)
	}

	if err := storage.SaveJSON(*outPath, compiled); err != nil {
		log.Fatalf("write compiled season: %v", err)
	}

	log.Printf(
		"compiled %d beats from %d story elements into %s (cross_element_dependencies=%d flat_greedy_beats=%d weak_standing_work=%d warnings=%d)",
		len(compiled.Ticks),
		len(loadedIR.Elements),
		*outPath,
		audit.CrossElementDependencyBeats,
		len(audit.FlatGreedyBeats),
		len(audit.WeakStandingWorkElements),
		len(audit.Warnings),
	)
}
