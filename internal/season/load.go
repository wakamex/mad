package season

import (
	"encoding/json"
	"os"
)

func LoadFile(path string) (File, error) {
	var loaded File

	raw, err := os.ReadFile(path)
	if err != nil {
		return loaded, err
	}
	if err := json.Unmarshal(raw, &loaded); err != nil {
		return loaded, err
	}
	if loaded.ScoreEpochTicks <= 0 {
		loaded.ScoreEpochTicks = 2
	}
	if loaded.RevealLagTicks <= 0 {
		loaded.RevealLagTicks = loaded.ScoreEpochTicks
	}
	if loaded.ShardCount <= 0 {
		loaded.ShardCount = 16
	}
	if err := Validate(loaded); err != nil {
		return loaded, err
	}
	return loaded, nil
}
