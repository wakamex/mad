package season

import (
	"encoding/json"
	"os"
)

func LoadFile(path string) (File, error) {
	var loaded File
	if err := loadJSONFile(path, &loaded); err != nil {
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

func LoadIRFile(path string) (IRFile, error) {
	var loaded IRFile
	if err := loadJSONFile(path, &loaded); err != nil {
		return loaded, err
	}
	if loaded.SchemaVersion == "" {
		loaded.SchemaVersion = "v1alpha1"
	}
	if err := ValidateIR(loaded); err != nil {
		return loaded, err
	}
	return loaded, nil
}

func loadJSONFile(path string, value any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, value)
}
