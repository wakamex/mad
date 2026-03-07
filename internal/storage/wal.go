package storage

import (
	"bufio"
	"encoding/json"
	"os"
	"sync"
	"time"
)

type WAL struct {
	mu   sync.Mutex
	path string
	file *os.File
}

type ActionRecord struct {
	SeasonID     string    `json:"season_id,omitempty"`
	PlayerID     uint32    `json:"player_id"`
	PublicID     string    `json:"public_id"`
	SubmissionID string    `json:"submission_id,omitempty"`
	TickID       string    `json:"tick_id"`
	Command      string    `json:"command"`
	Target       string    `json:"target,omitempty"`
	Option       string    `json:"option,omitempty"`
	Confidence   float64   `json:"confidence"`
	ReceivedAt   time.Time `json:"received_at"`
}

func NewWAL(path string) (*WAL, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &WAL{path: path, file: file}, nil
}

func (w *WAL) Append(record ActionRecord) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if _, err := w.file.Write(append(encoded, '\n')); err != nil {
		return err
	}
	return nil
}

func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Close()
}

func (w *WAL) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Sync()
}

func (w *WAL) RecordsAfter(after time.Time, seasonID string) ([]ActionRecord, error) {
	file, err := os.Open(w.path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64<<10), 1<<20)

	records := make([]ActionRecord, 0)
	for scanner.Scan() {
		var record ActionRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return nil, err
		}
		if seasonID != "" && record.SeasonID != "" && record.SeasonID != seasonID {
			continue
		}
		if !after.IsZero() && !record.ReceivedAt.After(after) {
			continue
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return records, nil
}
