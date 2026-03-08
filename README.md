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

## The Season Generator Map

The content-architecture map for story-element families, dependency rules, and skill-ceiling levers can be found here:
**[SEASON_GENERATOR.md](./SEASON_GENERATOR.md)**

## Running Locally

Weave the sample story-element IR into a compiled season:

```bash
env GOCACHE=/tmp/mad-gocache CGO_ENABLED=0 go run ./cmd/mad-weave -ir ./seasons/dev/season_ir.json -out ./build/season.json
```

Generate the larger reusable 1000-tick dev season IR:

```bash
env GOCACHE=/tmp/mad-gocache CGO_ENABLED=0 go run ./cmd/mad-devgen -ticks 1000 -out ./seasons/dev1000/season_ir.json
```

Compile that larger dev season:

```bash
env GOCACHE=/tmp/mad-gocache CGO_ENABLED=0 go run ./cmd/mad-weave -ir ./seasons/dev1000/season_ir.json -out ./seasons/dev1000/season.json
```

Dry-run the compiled season to inspect final tick order, reveal timing, derived memory-distance annotations, three baseline policies (`greedy_best`, `visible_greedy`, and `always_hold`), an explicit-vs-hidden decomposition (`visible_greedy` versus `greedy_best - visible_greedy`), and a deterministic random-play audit (`mean`, `p90`, `p99`, positive-rate, plus a representative `p99` random-run breakdown):

```bash
env GOCACHE=/tmp/mad-gocache CGO_ENABLED=0 go run ./cmd/mad-sim -season ./build/season.json -out ./build/simulation.json -random-runs 10000 -random-seed 1
```

The generated `seasons/dev1000` fixture is the current long-form dev season. At the moment it compiles to:

- `1000` ticks
- about `14.3` hours total runtime
- `250` story elements with deterministic variable lengths in the `2..5` beat range across standing work, clue chains, reputation ladders, hazard interrupts, and payoff gates
- baseline snapshot around `greedy_best=77628`, `visible_greedy=2047`, `always_hold=-8772`
- random-play audit around `mean=-1760`, `p90=299`, `p99=1925`, `positive_rate≈14.2%` using `1000` runs and seed `11`

For CI or release gating, fail the run if the random-play audit says the season is too easy to luck through:

```bash
env GOCACHE=/tmp/mad-gocache CGO_ENABLED=0 go run ./cmd/mad-sim -season ./build/season.json -out ./build/simulation.json -random-runs 10000 -random-seed 1 -fail-on-random-warnings
```

Compile public tick artifacts from the compiled season:

```bash
go run ./cmd/mad-compile -season ./build/season.json -out ./build/public
```

`mad-weave`, `mad-compile`, and `mad-core` all validate their input on load, so broken authoring data fails fast before runtime. The intended authoring flow is:

1. Define ordered multi-beat story elements in `season_ir.json`.
2. Deterministically interleave those elements into a compiled `season.json`.
3. Dry-run the compiled season and inspect the generated schedule/reveal report.
4. Compile immutable public tick artifacts from that compiled season.

For fast tests and smoke runs, keep using `seasons/dev/`. For a more realistic authoring and simulation loop, use `seasons/dev1000/`.

The compiler derives precursor tick links and memory-distance annotations after weaving, so story scoring stays independent of final tick spacing.
The simulator's `greedy_best` baseline is intentionally local to each tick. It is useful for sanity checks, but it is not a season-optimal oracle once opportunity costs or commitments become stateful. `visible_greedy` is the cheap constrained baseline: it only uses the current public action surface, clock class, public requirements, and explicit player state, with no source-text parsing and no hidden scoring labels. The report's `decomposition` section is an approximation: `explicit_visible` comes from `visible_greedy`, while `hidden_or_nonlocal_premium` is the remaining score in `greedy_best - visible_greedy`, so it mixes real cross-beat value with any hidden-label advantage still present in `greedy_best`.

Run the external-agent harness against a compiled season. The harness keeps a single conversation thread per runner, records every action/response, and saves a per-tick `score_trace` so the result can be plotted like VendingBench:

```bash
env GOCACHE=/tmp/mad-gocache CGO_ENABLED=0 go run ./cmd/mad-harness \
  -season ./seasons/dev1000/season.json \
  -out ./build/harness.json \
  -runs 3 \
  -max-ticks 25 \
  -runner codex:gpt-5.2-codex@high \
  -runner codex:gpt-5.1-codex-mini@medium \
  -runner claude:haiku@low \
  -runner claude:sonnet@medium
```

`mad-harness` checkpoints the JSON report after every tick, so long runs leave a live-updating `score_trace` on disk instead of only writing at the very end. That makes overnight runs inspectable and plot-friendly even before they finish.
During each run, `mad-harness` prints one static `run_start` header and then keeps a single live progress line updated in place with terse stats like tick progress, score, last score delta, average step time, ETA, errors, and last completed tick. At the end of each run it prints a concise summary with final score, step count, wall time, average step latency, `p50`/`p95` decision latency, ticks per minute, and the last completed tick. If `-runs N` is greater than `1`, it also prints multi-run aggregates at the very end.

For humans, the easiest entrypoint is [scripts/mad-run](/code/mad/scripts/mad-run), a thin shim over `cmd/mad-run`:

```bash
  ./scripts/mad-run --provider codex --model gpt-5.2-codex --effort high --memory on --service-tier fast --max-ticks 100
  ./scripts/mad-run --provider codex --model gpt-5.1-codex-mini --effort medium --memory off --context ephemeral --service-tier fast --runs 3 --season ./seasons/dev/season.json
  ./scripts/mad-run --provider claude --model haiku --effort low --memory off --context ephemeral --probe
  ./scripts/mad-run --provider openrouter --model openai/gpt-oss-20b --memory off --context ephemeral --service-tier fast --max-ticks 100
```

`scripts/mad-run` creates a timestamped run directory under `build/runs/`, stores the exact command it launched, and writes a live-updating `harness.json` plus `launcher.log`.

For unambiguous experiment labels, use the canonical mode definitions in [CONFIG.md](/code/mad/CONFIG.md). Forecast ranges for common model/mode permutations live in [FORECAST.md](/code/mad/FORECAST.md).

## OpenRouter Baselines

The harness should accept any `openrouter:<model-slug>` runner so the baseline
matrix stays future-proof.

Measured practical results, including:
- which models actually return non-null `top_logprobs` on the `1-token`
  logprob-choice path
- advertised throughput versus actual short-output latency
- current default / fastest / cheapest working candidates

live in [OPENROUTER_BASELINES.md](/code/mad/OPENROUTER_BASELINES.md).

Memory and context semantics are explicit:

- `codex --memory on`: create an isolated writable `CODEX_HOME` inside the run directory, preserve provider-native session continuity there, explicitly enable Codex memory features, and set the run-local Codex memory idle gate to `0` hours by default so memory is usable during benchmark runs.
- `codex --memory off`: use the same isolated writable `CODEX_HOME`, but explicitly disable Codex memory features while keeping normal session continuity.
- `codex --service-tier fast`: request Codex fast mode (`service_tier=fast`). This is the quickest way to get a Codex no-context baseline, especially when combined with `--context ephemeral`.
- `codex --service-tier flex`: request the normal flex tier explicitly.
- `claude --memory on`: use an isolated Claude home inside the run directory. With `--context persistent`, the harness keeps a persisted `claude -p` session. With `--context ephemeral`, it uses `--no-session-persistence` but still leaves Claude auto-memory enabled.
- `claude --memory off`: use the same isolated Claude home, but explicitly set `CLAUDE_CODE_DISABLE_AUTO_MEMORY=1`. Session persistence still follows `--context`; memory-off no longer implies ephemeral.
- `openrouter`: requires `OPENROUTER_API_KEY` in the environment. The current harness path is intended for fast baseline runs, especially `--memory off --context ephemeral`. It uses direct OpenRouter chat completions and asks the model to return a numbered action choice, which the harness validates locally. Unlike Codex and Claude, it does not currently have a provider-native session or memory layer inside the harness.
- `--context persistent`: keep thread/session continuity and let the harness carry forward the model's own `notes` field across ticks.
- `--context ephemeral`: run each tick as a one-shot baseline. Provider-native session continuity is disabled, and the harness does not carry prior `notes` into later prompts.

Continuity is provider-native:

- Codex starts a persisted `codex exec` session in the real project cwd, captures the provider `thread_id`, and resumes that exact thread on later ticks.
- Claude starts each run with an isolated Claude home rooted under `build/runs/.../runner-state/.../claude-home`. Persistent mode uses an explicit UUID `--session-id`; ephemeral mode uses `--no-session-persistence`. The harness disables `CLAUDE.md` loading for benchmark cleanliness.
- Each harness run records `session.workdir`, `session.provider_session_id`, `session.native_home_dir`, `session.native_project_dir`, `session.native_session_path`, and `session.native_memory_path`.
- A Claude `native_memory_path` may still be absent on disk after a short run; memory-on means Claude can read/write `MEMORY.md`, not that it is guaranteed to write one every turn.

Probe runner/model availability without playing a season:

```bash
env GOCACHE=/tmp/mad-gocache CGO_ENABLED=0 go run ./cmd/mad-harness -probe -out ./build/harness-probe.json
```

If no `-runner` flags are provided, `mad-harness` uses the current default matrix:

- `codex:gpt-5.1-codex-mini@medium`
- `codex:gpt-5.2-codex@high`
- `codex:gpt-5.4@medium`
- `claude:haiku@low`
- `claude:sonnet@medium`
- `claude:opus@high`

Run the dev server:

```bash
go run ./cmd/mad-core -season ./build/season.json -listen :8080
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
6. **Build Season Tooling:** Extend the existing story-element IR, weave compiler, validator, simulation report, and annotation helpers with richer authoring ergonomics and deeper season simulation for lawful content at scale.

---
*Authored by the MAD Design Team (Clod, Dex, Gem).*
