package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/netip"
	"path"
	"strings"
	"time"

	"github.com/mihai/mad/internal/game"
)

type Server struct {
	engine            *game.Engine
	tokenLimits       *fixedWindowLimiter
	ipLimits          *fixedWindowLimiter
	trustProxyHeaders bool
}

type Options struct {
	TokenRateLimit    int
	IPRateLimit       int
	TrustProxyHeaders bool
}

func NewServer(engine *game.Engine) *Server {
	return NewServerWithOptions(engine, Options{})
}

func NewServerWithOptions(engine *game.Engine, options Options) *Server {
	if options.TokenRateLimit <= 0 {
		options.TokenRateLimit = 120
	}
	if options.IPRateLimit <= 0 {
		options.IPRateLimit = 600
	}
	return &Server{
		engine:            engine,
		tokenLimits:       newFixedWindowLimiter(1*time.Minute, options.TokenRateLimit),
		ipLimits:          newFixedWindowLimiter(1*time.Minute, options.IPRateLimit),
		trustProxyHeaders: options.TrustProxyHeaders,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/manifest.json", s.handleManifest)
	mux.HandleFunc("/current.json", s.handleCurrent)
	mux.HandleFunc("/ticks/", s.handleTick)
	mux.HandleFunc("/reveals/", s.handleReveal)
	mux.HandleFunc("/score-epochs/", s.handleScoreEpochs)
	mux.HandleFunc("/actions", s.handleAction)
	return mux
}

func (s *Server) handleManifest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}
	writeJSON(w, http.StatusOK, s.engine.Manifest())
}

func (s *Server) handleCurrent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}
	writeJSON(w, http.StatusOK, s.engine.Current())
}

func (s *Server) handleTick(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}
	tickID := strings.TrimSuffix(path.Base(r.URL.Path), ".json")
	tick, ok := s.engine.PublicTick(tickID)
	if !ok {
		writeError(w, http.StatusNotFound, "tick_not_found")
		return
	}
	writeJSON(w, http.StatusOK, tick)
}

func (s *Server) handleReveal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}
	tickID := strings.TrimSuffix(path.Base(r.URL.Path), ".json")
	reveal, ok := s.engine.Reveal(tickID)
	if !ok {
		writeError(w, http.StatusNotFound, "reveal_not_found")
		return
	}
	writeJSON(w, http.StatusOK, reveal)
}

func (s *Server) handleScoreEpochs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}
	trimmed := strings.TrimPrefix(r.URL.Path, "/score-epochs/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 2 {
		writeError(w, http.StatusNotFound, "score_epoch_not_found")
		return
	}
	epochID := parts[0]
	if len(parts) == 2 && parts[1] == "top.json" {
		snapshot, ok := s.engine.ScoreEpoch(epochID)
		if !ok {
			writeError(w, http.StatusNotFound, "score_epoch_not_found")
			return
		}
		writeJSON(w, http.StatusOK, snapshot)
		return
	}
	if len(parts) == 3 && parts[1] == "shards" {
		shardID := strings.TrimSuffix(parts[2], ".json")
		shard, ok := s.engine.ScoreShard(epochID, shardID)
		if !ok {
			writeError(w, http.StatusNotFound, "score_shard_not_found")
			return
		}
		writeJSON(w, http.StatusOK, shard)
		return
	}
	writeError(w, http.StatusNotFound, "score_epoch_not_found")
}

func (s *Server) handleAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if token == "" {
		writeError(w, http.StatusUnauthorized, "missing_bearer_token")
		return
	}
	now := time.Now().UTC()
	if !s.tokenLimits.allow("token:"+token, now) {
		writeError(w, http.StatusTooManyRequests, "token_rate_limited")
		return
	}
	if ip := clientIP(r, s.trustProxyHeaders); ip != "" && !s.ipLimits.allow("ip:"+ip, now) {
		writeError(w, http.StatusTooManyRequests, "ip_rate_limited")
		return
	}
	defer r.Body.Close()

	var submission game.ActionSubmission
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&submission); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}

	receipt, err := s.engine.Submit(token, submission, now)
	switch {
	case err == nil:
		writeJSON(w, http.StatusAccepted, receipt)
	case errors.Is(err, game.ErrorBadAuth()):
		writeError(w, http.StatusUnauthorized, "bad_auth")
	case errors.Is(err, game.ErrorWrongTick()):
		writeError(w, http.StatusConflict, "wrong_tick")
	case errors.Is(err, game.ErrorDeadlineMiss()):
		writeError(w, http.StatusGone, "deadline_missed")
	default:
		writeError(w, http.StatusBadRequest, "invalid_action")
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

func clientIP(r *http.Request, trustProxyHeaders bool) string {
	if trustProxyHeaders {
		if ip := parseIP(r.Header.Get("CF-Connecting-IP")); ip != "" {
			return ip
		}
		if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
			first, _, _ := strings.Cut(forwardedFor, ",")
			if ip := parseIP(strings.TrimSpace(first)); ip != "" {
				return ip
			}
		}
	}
	return remoteIP(r.RemoteAddr)
}

func remoteIP(remoteAddr string) string {
	host, _, found := strings.Cut(remoteAddr, ":")
	if !found {
		host = remoteAddr
	}
	return parseIP(host)
}

func parseIP(raw string) string {
	addr, err := netip.ParseAddr(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return addr.String()
}
