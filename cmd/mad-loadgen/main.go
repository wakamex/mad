package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"os/signal"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/mihai/mad/internal/game"
)

type loadCurrent struct {
	TickID     string `json:"tick_id"`
	NextTickAt int64  `json:"next_tick_at"`
}

func main() {
	baseURL := flag.String("base-url", "http://127.0.0.1:8080", "MAD base URL")
	players := flag.Int("players", 1000, "Number of synthetic players to submit as")
	concurrency := flag.Int("concurrency", 64, "Number of concurrent workers")
	rounds := flag.Int("rounds", 1, "Number of burst rounds")
	pause := flag.Duration("pause", 500*time.Millisecond, "Pause between rounds")
	timeout := flag.Duration("timeout", 10*time.Second, "Per-request timeout")
	deadlineLead := flag.Duration("deadline-lead", 0, "If set, wait until this long before the current deadline before firing the burst")
	command := flag.String("command", "hold", "Action command to submit")
	target := flag.String("target", "", "Action target to submit")
	option := flag.String("option", "", "Action option to submit")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	client := &http.Client{Timeout: *timeout}
	var accepted, rateLimited, conflicts, other int64
	var totalLatencyNS int64
	var maxLatencyNS int64
	latencies := make([]int64, 0, *players**rounds)
	var latMu sync.Mutex
	started := time.Now()

	for round := 0; round < *rounds; round++ {
		current, err := fetchCurrent(ctx, client, *baseURL)
		if err != nil {
			log.Fatalf("fetch current: %v", err)
		}
		if *deadlineLead > 0 {
			waitUntil := time.Unix(current.NextTickAt, 0).Add(-*deadlineLead)
			if delay := time.Until(waitUntil); delay > 0 {
				log.Printf("round %d/%d waiting %s for %s", round+1, *rounds, delay, current.TickID)
				timer := time.NewTimer(delay)
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
			}
		}

		log.Printf("starting round %d/%d on tick %s", round+1, *rounds, current.TickID)
		var wg sync.WaitGroup
		sem := make(chan struct{}, *concurrency)

		for player := 1; player <= *players; player++ {
			player := player
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				status, latency, err := submitAction(ctx, client, *baseURL, current.TickID, player, round, game.ActionSubmission{
					TickID:     current.TickID,
					Command:    *command,
					Target:     *target,
					Option:     *option,
				})
				if err != nil {
					atomic.AddInt64(&other, 1)
					log.Printf("submit error for player %d: %v", player, err)
					return
				}
				atomic.AddInt64(&totalLatencyNS, latency.Nanoseconds())
				updateMax(&maxLatencyNS, latency.Nanoseconds())
				latMu.Lock()
				latencies = append(latencies, latency.Nanoseconds())
				latMu.Unlock()
				switch status {
				case http.StatusAccepted:
					atomic.AddInt64(&accepted, 1)
				case http.StatusTooManyRequests:
					atomic.AddInt64(&rateLimited, 1)
				case http.StatusConflict, http.StatusGone:
					atomic.AddInt64(&conflicts, 1)
				default:
					atomic.AddInt64(&other, 1)
				}
			}()
		}
		wg.Wait()
		if round < *rounds-1 {
			time.Sleep(*pause)
		}
	}

	total := accepted + rateLimited + conflicts + other
	avgLatency := time.Duration(0)
	if total > 0 {
		avgLatency = time.Duration(totalLatencyNS / total)
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	fmt.Printf("players=%d rounds=%d concurrency=%d elapsed=%s\n", *players, *rounds, *concurrency, time.Since(started))
	fmt.Printf("accepted=%d rate_limited=%d late_or_wrong_tick=%d other=%d\n", accepted, rateLimited, conflicts, other)
	fmt.Printf("avg_latency=%s p50=%s p95=%s p99=%s max_latency=%s\n",
		avgLatency,
		percentile(latencies, 0.50),
		percentile(latencies, 0.95),
		percentile(latencies, 0.99),
		time.Duration(maxLatencyNS),
	)
}

func fetchCurrent(ctx context.Context, client *http.Client, baseURL string) (loadCurrent, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/current.json", nil)
	if err != nil {
		return loadCurrent{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return loadCurrent{}, err
	}
	defer resp.Body.Close()

	var current loadCurrent
	if err := json.NewDecoder(resp.Body).Decode(&current); err != nil {
		return loadCurrent{}, err
	}
	return current, nil
}

func submitAction(ctx context.Context, client *http.Client, baseURL, tickID string, player, round int, action game.ActionSubmission) (int, time.Duration, error) {
	action.TickID = tickID
	action.SubmissionID = fmt.Sprintf("loadgen-r%03d-p%07d", round+1, player)

	body, err := json.Marshal(action)
	if err != nil {
		return 0, 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/actions", bytes.NewReader(body))
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer dev-token-%06d", player))
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, time.Since(start), nil
}

func percentile(values []int64, q float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	if q <= 0 {
		return time.Duration(values[0])
	}
	if q >= 1 {
		return time.Duration(values[len(values)-1])
	}
	idx := int(math.Ceil(float64(len(values))*q)) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(values) {
		idx = len(values) - 1
	}
	return time.Duration(values[idx])
}

func updateMax(dst *int64, candidate int64) {
	for {
		current := atomic.LoadInt64(dst)
		if candidate <= current {
			return
		}
		if atomic.CompareAndSwapInt64(dst, current, candidate) {
			return
		}
	}
}
