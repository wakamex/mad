package api

import (
	"testing"
	"time"
)

func TestFixedWindowLimiter(t *testing.T) {
	limiter := newFixedWindowLimiter(1*time.Minute, 2)
	now := time.Now().UTC()

	if !limiter.allow("k", now) {
		t.Fatalf("expected first request allowed")
	}
	if !limiter.allow("k", now.Add(5*time.Second)) {
		t.Fatalf("expected second request allowed")
	}
	if limiter.allow("k", now.Add(10*time.Second)) {
		t.Fatalf("expected third request denied")
	}
	if !limiter.allow("k", now.Add(61*time.Second)) {
		t.Fatalf("expected request allowed after window rollover")
	}
}
