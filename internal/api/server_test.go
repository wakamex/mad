package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/mihai/mad/internal/game"
	"github.com/mihai/mad/internal/season"
	"github.com/mihai/mad/internal/storage"
)

func testServer(t *testing.T) (*Server, *game.Engine) {
	t.Helper()
	loadedSeason, err := season.LoadFile(filepath.Join("..", "..", "seasons", "dev", "season.json"))
	if err != nil {
		t.Fatalf("load season: %v", err)
	}
	wal, err := storage.NewWAL(filepath.Join(t.TempDir(), "actions.log"))
	if err != nil {
		t.Fatalf("wal: %v", err)
	}
	engine := game.NewEngine(loadedSeason, wal, 8)
	return NewServer(engine), engine
}

func TestActionEndpoint(t *testing.T) {
	server, engine := testServer(t)
	body, _ := json.Marshal(game.ActionSubmission{
		TickID:     engine.Current().TickID,
		Command:    "hold",
		Confidence: 0,
	})

	req := httptest.NewRequest(http.MethodPost, "/actions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+engine.DevToken(1))
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRevealEndpointAfterClose(t *testing.T) {
	server, engine := testServer(t)
	now := time.Now().UTC()
	_, err := engine.Submit(engine.DevToken(1), game.ActionSubmission{
		TickID:     engine.Current().TickID,
		Command:    "commit",
		Target:     "quest.glass_choir.7",
		Option:     "broker",
		Confidence: 0.8,
	}, now)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	engine.DebugForceClose(now)
	engine.DebugForceClose(now)

	req := httptest.NewRequest(http.MethodGet, "/reveals/S1-T0001.json", nil)
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRateLimit(t *testing.T) {
	server, engine := testServer(t)
	body, _ := json.Marshal(game.ActionSubmission{
		TickID:     engine.Current().TickID,
		Command:    "hold",
		Confidence: 0,
	})
	handler := server.Routes()
	for i := 0; i < 120; i++ {
		req := httptest.NewRequest(http.MethodPost, "/actions", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+engine.DevToken(1))
		req.RemoteAddr = "127.0.0.1:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusAccepted {
			t.Fatalf("unexpected early status at %d: %d", i, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/actions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+engine.DevToken(1))
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected rate limit, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestTrustProxyHeadersForIPLimit(t *testing.T) {
	loadedSeason, err := season.LoadFile(filepath.Join("..", "..", "seasons", "dev", "season.json"))
	if err != nil {
		t.Fatalf("load season: %v", err)
	}
	wal, err := storage.NewWAL(filepath.Join(t.TempDir(), "actions.log"))
	if err != nil {
		t.Fatalf("wal: %v", err)
	}
	engine := game.NewEngine(loadedSeason, wal, 8)
	server := NewServerWithOptions(engine, Options{
		TokenRateLimit:    120,
		IPRateLimit:       1,
		TrustProxyHeaders: true,
	})

	body, _ := json.Marshal(game.ActionSubmission{
		TickID:     engine.Current().TickID,
		Command:    "hold",
		Confidence: 0,
	})
	handler := server.Routes()

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/actions", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+engine.DevToken(i+1))
		req.Header.Set("CF-Connecting-IP", "203.0.113.9")
		req.RemoteAddr = "127.0.0.1:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if i == 0 && rec.Code != http.StatusAccepted {
			t.Fatalf("unexpected first status: %d body=%s", rec.Code, rec.Body.String())
		}
		if i == 1 && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("expected proxy-header rate limit, got %d body=%s", rec.Code, rec.Body.String())
		}
	}
}
