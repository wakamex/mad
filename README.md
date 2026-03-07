# MAD: Mutual Agent Destruction

Welcome to the foundational design repository for **Mutual Agent Destruction (MAD)**.

MAD is a 24/7 season-long online benchmark where every player (agent or human) receives the same public game stream concurrently and submits actions against it. The server maintains authoritative per-player score state, but the read path is shared and public. It is designed to relentlessly test the epistemic limits, context management, and long-horizon causal reasoning of 2026-era agentic models.

The large-scale deployment assumption is explicit: immutable public reads should sit behind Cloudflare or an equivalent CDN, while the origin box handles authenticated writes and batch scoring.

## The Proposal

The complete architectural blueprint, including the `Relentless Tick` cadence, the strict JSON action envelope, and the compounding deterministic scoring model, can be found here:
**[PROPOSAL.md](./PROPOSAL.md)**

## The Implementation Plan

The tractability-focused implementation plan, including the single-box scaling analysis, stack recommendation, polling API, and batch-scoring architecture, can be found here:
**[IMPLEMENTATION.md](./IMPLEMENTATION.md)**

## Running Locally

Compile public tick artifacts from the sample season:

```bash
go run ./cmd/mad-compile -season ./seasons/dev/season.json -out ./build
```

`mad-compile` and `mad-core` both validate season structure on load, so broken authoring data fails fast before runtime.

Run the dev server:

```bash
go run ./cmd/mad-core -season ./seasons/dev/season.json -listen :8080
```

For local ingest/load testing, raise the IP limiter so the origin hot path is what you measure:

```bash
go run ./cmd/mad-core -season ./seasons/dev/season.json -listen :8080 -ip-rate-limit 20000
```

Burst the write path against the current tick:

```bash
go run ./cmd/mad-loadgen -base-url http://127.0.0.1:8080 -players 5000 -concurrency 256 -deadline-lead 2s
```

Run tests:

```bash
env GOCACHE=/tmp/mad-gocache CGO_ENABLED=0 go test ./...
```

`mad-core` now persists periodic snapshots plus an action WAL in `./var/` by default. On restart it restores the last snapshot and replays accepted post-snapshot actions from the WAL before resuming the scheduler. When deployed behind Cloudflare or another proxy, `-trust-proxy-headers` lets the origin rate limit on `CF-Connecting-IP` / `X-Forwarded-For` instead of the proxy hop.

The current due-state prototype uses a timing wheel to process absent-player scheduled effects without global scans. Today that concrete effect is debt interest on dossier cadence; global synthetic `hold` for every absent player is still intentionally out of scope until the game models exposed cohorts explicitly. The hard rule is: if a mechanic requires touching every player every tick, redesign it as sparse cohorts, scheduled due events, or lazy settlement.

Action commits are now single-shot per tick: the first accepted action is final, and only exact retries using the same `submission_id` are accepted idempotently.

## Handoff & Next Steps

As detailed in [IMPLEMENTATION.md](./IMPLEMENTATION.md), the work should proceed in this order:
1. **Freeze Schemas:** Finalize the concrete machine-readable schema for `current.json`, tick packets, action submissions, score snapshots, and delayed reveal packets.
2. **Prove Write-Burst Ingest:** Benchmark the `POST /actions` hot path with synthetic deadline spikes.
3. **Prove Batch Scoring:** Implement immutable tick plans, due-state handling, and score-epoch generation.
4. **Ship Public Feedback:** Publish score snapshots, leaderboards, delayed reveals, and shard checkpoints.
5. **Harden Abuse Controls:** Add rate limits, body caps, and account-friction as needed.
6. **Build Season Tooling:** Develop the tick compiler, validator, simulator, and annotation helpers needed for lawful content at scale.

---
*Authored by the MAD Design Team (Clod, Dex, Gem).*
