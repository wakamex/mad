package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mihai/mad/internal/season"
	"github.com/mihai/mad/internal/storage"
)

func main() {
	seasonPath := flag.String("season", filepath.Join("seasons", "dev1000", "season.json"), "Path to compiled season JSON")
	outPath := flag.String("out", "", "Optional path to save full JSON report")
	familyFilter := flag.String("family", "", "Only probe ticks from this family (e.g. payoff_gate)")
	flag.Parse()

	loaded, err := season.LoadFile(*seasonPath)
	if err != nil {
		log.Fatalf("load season: %v", err)
	}

	report := season.RunProbe(loaded)

	if *outPath != "" {
		if err := storage.SaveJSON(*outPath, report); err != nil {
			log.Fatalf("save report: %v", err)
		}
		log.Printf("full report saved to %s", *outPath)
	}

	// Print summary to stdout.
	fmt.Printf("MAD Leakage Probe: %s\n", report.SeasonID)
	fmt.Printf("  ticks: %d total, %d probeable\n\n", report.TickCount, report.ProbeableTicks)

	// Sort families for stable output.
	familyNames := make([]string, 0, len(report.Families))
	for f := range report.Families {
		familyNames = append(familyNames, f)
	}
	sort.Strings(familyNames)

	fmt.Printf("%-22s %5s %7s %7s %7s %7s %7s %7s %8s\n",
		"FAMILY", "TICKS", "RANDOM", "MAJOR", "KEYWRD", "SKELT", "TMPL", "SRCTYP", "LEAKAGE")
	fmt.Printf("%s\n", strings.Repeat("-", 91))

	for _, name := range familyNames {
		if *familyFilter != "" && name != *familyFilter {
			continue
		}
		pf := report.Families[name]
		marker := ""
		if pf.LeakageScore > 0.3 {
			marker = " !!!"
		} else if pf.LeakageScore > 0.15 {
			marker = " !!"
		} else if pf.LeakageScore > 0.05 {
			marker = " !"
		}
		fmt.Printf("%-22s %5d %6.1f%% %6.1f%% %6.1f%% %6.1f%% %6.1f%% %6.1f%% %6.1f%%%s\n",
			name, pf.Ticks,
			pf.RandomAccuracy*100,
			pf.MajorityAccuracy*100,
			pf.KeywordAccuracy*100,
			pf.SkeletonAccuracy*100,
			pf.TemplateAccuracy*100,
			pf.SourceTypeAccuracy*100,
			pf.LeakageScore*100,
			marker)
	}

	fmt.Printf("%s\n", strings.Repeat("-", 91))
	fmt.Printf("%-22s %5d %6.1f%% %6.1f%% %6.1f%% %6.1f%%\n",
		"OVERALL", report.Summary.TotalProbeable,
		report.Summary.OverallRandomAccuracy*100,
		report.Summary.OverallKeywordAccuracy*100,
		report.Summary.OverallSkeletonAccuracy*100,
		report.Summary.OverallTemplateAccuracy*100)
	fmt.Println()

	// Print top template patterns if leaking.
	if len(report.Templates) > 0 {
		fmt.Printf("Top template patterns (prose skeleton -> answer):\n")
		shown := 0
		for _, t := range report.Templates {
			if *familyFilter != "" && t.Family != *familyFilter {
				continue
			}
			if shown >= 15 {
				break
			}
			truncated := t.ProsePattern
			if len(truncated) > 80 {
				truncated = truncated[:77] + "..."
			}
			fmt.Printf("  [%s] %dx -> %s\n    %s\n",
				t.Family, t.Occurrences, t.BestOption, truncated)
			shown++
		}
		fmt.Println()
	}

	// Source breakdown.
	sourceNames := make([]string, 0, len(report.Sources))
	for s := range report.Sources {
		sourceNames = append(sourceNames, s)
	}
	sort.Strings(sourceNames)

	fmt.Printf("%-22s %5s %7s %7s\n", "SOURCE TYPE", "TICKS", "KEYWRD", "TMPL")
	fmt.Printf("%s\n", strings.Repeat("-", 50))
	for _, name := range sourceNames {
		ps := report.Sources[name]
		fmt.Printf("%-22s %5d %6.1f%% %6.1f%%\n",
			name, ps.Ticks,
			ps.KeywordAccuracy*100,
			ps.TemplateAccuracy*100)
	}

	fmt.Printf("\nVERDICT: %s\n", report.Summary.Verdict)
}
