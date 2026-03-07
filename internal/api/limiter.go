package api

import (
	"sync"
	"time"
)

type fixedWindowLimiter struct {
	mu         sync.Mutex
	windowSize time.Duration
	limit      int
	counts     map[string]int
	windowEnds time.Time
}

func newFixedWindowLimiter(windowSize time.Duration, limit int) *fixedWindowLimiter {
	return &fixedWindowLimiter{
		windowSize: windowSize,
		limit:      limit,
		counts:     make(map[string]int),
		windowEnds: time.Now().UTC().Add(windowSize),
	}
}

func (l *fixedWindowLimiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if now.After(l.windowEnds) {
		l.counts = make(map[string]int)
		l.windowEnds = now.Add(l.windowSize)
	}

	l.counts[key]++
	return l.counts[key] <= l.limit
}
