package season

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRuleMarshalJSONOmitsEmptyRequirementsAndEffects(t *testing.T) {
	data, err := json.Marshal(Rule{
		Match:          ActionMatch{Command: "hold"},
		Delta:          ScoreDelta{MissPenalties: 1},
		Label:          "hold",
		Classification: "miss",
	})
	if err != nil {
		t.Fatalf("marshal rule: %v", err)
	}

	text := string(data)
	if strings.Contains(text, `"requirements"`) {
		t.Fatalf("expected empty requirements to be omitted: %s", text)
	}
	if strings.Contains(text, `"effects"`) {
		t.Fatalf("expected empty effects to be omitted: %s", text)
	}
}
