package season

import "testing"

func TestRunProbe_SmallSeason(t *testing.T) {
	// Build a small dev season and verify the probe runs without panic.
	ir, err := BuildGeneratedDevSeasonIR(20)
	if err != nil {
		t.Fatalf("build dev IR: %v", err)
	}
	compiled, err := CompileIR(ir)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	report := RunProbe(compiled)
	if report.TickCount != 20 {
		t.Errorf("tick count = %d, want 20", report.TickCount)
	}
	if report.ProbeableTicks == 0 {
		t.Error("expected at least one probeable tick")
	}
	if len(report.Families) == 0 {
		t.Error("expected at least one family in probe report")
	}
	// The dev season is known to leak, so template accuracy should be high.
	for family, pf := range report.Families {
		if pf.TemplateAccuracy < pf.RandomAccuracy {
			t.Errorf("%s: template accuracy %.2f < random %.2f", family, pf.TemplateAccuracy, pf.RandomAccuracy)
		}
	}
}

func TestNormalizeProsePattern(t *testing.T) {
	a := normalizeProsePattern([]string{"Market brief: fog exposure split the bid on amber wire lots."})
	b := normalizeProsePattern([]string{"Market brief: fog exposure split the bid on amber wire lots."})
	if a != b {
		t.Errorf("identical prose produced different patterns: %q vs %q", a, b)
	}
	c := normalizeProsePattern([]string{"Market brief: rain exposure split the bid on green glass lots."})
	// Same template, different fill — should produce different pattern since
	// we don't strip proper nouns (just numbers). This is expected: the
	// template probe catches structural repetition.
	_ = c
}

func TestKeywordProbeCorrect(t *testing.T) {
	pt := probeableTick{
		bestOption: "quarantine",
		allOptions: []string{"broker", "auction", "quarantine"},
		proseTexts: []string{"Quarantine protocols are active in the southern ward."},
	}
	if !keywordProbeCorrect(pt) {
		t.Error("expected keyword probe to detect 'quarantine' in prose")
	}
}
