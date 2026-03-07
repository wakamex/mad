# MAD Implementation

Status: implementation proposal v1

## Executive Decision

Do not build the primary runtime around WebSockets or per-player response packets.

For tractability on a cheap single box, MAD should be:

- A shared public tick feed served as static or near-static HTTP
- One authenticated action-ingest endpoint
- One batched scoring pipeline
- One public score-publication path

The read path should be the same for everyone. The write path should be tiny. The scoring path should run in batches after deadlines, not inline on request threads.

## Tractability Changes From the Design Draft

The following changes are required to make the benchmark feasible on a roughly `$50/month` Hetzner box:

- No private packets in the hot path
- No per-player narrative generation
- No WebSocket-first design
- No per-request score calculation
- No per-user GET endpoint required for normal play

The server can still maintain authoritative per-player score state. It just should not emit player-specific world information.

## Load Model

Assume the official mode stays on a 90-second standard tick.

If `1,000,000` active players all submit once per standard tick:

- Average action-ingest rate: about `11,111 POST/s`
- Ticks per day: `960`
- Actions per day: about `960,000,000`

If `1,000,000` active players all fetch one compressed `2 KB` public tick per standard tick:

- Average outbound read bandwidth: about `22 MB/s`
- Daily outbound traffic: about `1.9 TB/day`
- Monthly outbound traffic: about `57.6 TB/month`

If the compressed tick is `4 KB`, the monthly read traffic doubles to about `115 TB/month`.

Conclusion:

- CPU is not the first bottleneck
- Memory is manageable if player state is compact
- Network egress is the real budget killer

One cheap box can be the authoritative origin and scorer. It cannot be the only uncached global edge for one million fully active readers and still stay within a strict `$50/month` budget.

### Per-Component Bandwidth Math

At `1,000,000` active players and a `90s` tick:

- `current.json` at `200 B`: about `200 MB` per tick, `2.2 MB/s`, about `5.8 TB/month`
- public tick file at `2 KB`: about `2.0 GB` per tick, `22.2 MB/s`, about `57.6 TB/month`
- action submit round-trip at about `400 B` combined request plus response: about `400 MB` per tick, `4.4 MB/s`, about `11.5 TB/month`

Combined baseline at `2 KB` compressed public ticks:

- about `28.8 MB/s` average
- about `74.9 TB/month`

This is why the origin must:

- keep reads static and cacheable
- avoid personalized responses
- avoid making every player fetch large score artifacts on every score epoch

## Budget Assumption

As of March 7, 2026, Hetzner's official cloud page lists plans such as:

- `CX53`: `16 vCPU`, `32 GB RAM`, `320 GB SSD`, `20 TB` included traffic, about `$19.59/month`
- `CAX41`: `16 vCPU`, `32 GB RAM`, `320 GB SSD`, `20 TB` included traffic, about `$27.59/month`

That means a `$50/month` budget can plausibly buy:

- One `16 vCPU / 32 GB` box
- Some traffic overage
- Not infinite global bandwidth

Treat the operational target as "one strong origin node," not "one magical million-user edge node."

## Recommended Stack

### Primary Recommendation

- `nginx` in front for TLS, static files, caching headers, compression, and rate limiting
- `Go` for the control plane, ingest service, scheduler, and scorer
- `SQLite` for small admin tables only: accounts, season metadata, and auth mapping
- Flat binary snapshots plus append-only WAL files for player-state durability

### Why Go

This workload is mostly:

- Small JSON decode/encode
- Tight request validation
- Fixed-size in-memory state mutation
- Buffered disk append
- Parallel batch processing

Go is a good fit because:

- The standard library HTTP stack is enough
- Deployment is simple
- Memory use is predictable if the hot path avoids maps where possible
- Profiling and operational tooling are mature

### Why Not WebSockets First

WebSockets are the wrong default if every player sees the same data.

They turn a cacheable shared broadcast into:

- One million open server-side sockets
- Higher memory pressure
- More connection management
- Fewer easy cache wins

If the feed is identical for everyone, plain HTTP with predictable polling is cheaper and simpler.

### Why Not Elixir/Phoenix As The First Choice

Elixir/Phoenix is a strong option if the product depends on millions of long-lived connections or rich per-user fanout. That is not the problem we should optimize first.

For this version of MAD:

- Reads should be static
- Writes should be tiny
- Scoring should be batched

That makes `Go + nginx` a better fit for a single cheap origin box.

If the product later reintroduces persistent live fanout, Phoenix becomes a credible alternative.

### Why Not Rust As The First Choice

Rust would likely squeeze more throughput out of the same hardware, but the first-order constraints here are:

- Egress
- Storage strategy
- Simplicity of the scoring pipeline

Choose Rust only if Go profiling later proves the scorer or ingest path is truly CPU-bound.

## API Surface

Keep the runtime API tiny.

### `GET /manifest.json`

Season-level static metadata:

- `season_id`
- schema versions
- tick cadence defaults
- score publication cadence
- hash shard count

### `GET /current.json`

Tiny shared pointer document:

- `season_id`
- `tick_id`
- `tick_url`
- `next_tick_at`
- `next_poll_after_ms`
- `current_score_epoch`
- `score_epoch_url`

This endpoint should be aggressively cacheable until `next_tick_at`.

### `GET /ticks/{tick_id}.json.zst`

Immutable public tick packet.

This is the main read object clients care about.

Each tick beyond the first includes a `previous_result` block with the
public answer reveal for the prior tick:

```json
{
  "tick_id": "S1-T2045",
  "previous_result": {
    "tick_id": "S1-T2044",
    "opportunity_id": "quest.glass_choir.7",
    "correct_command": "commit",
    "correct_option": "broker",
    "correct_phrase": null,
    "explanation": "The Glass Choir's severance from the Resonance Collective inverted broker value. The green rain from T1800 made glass fragile — only a broker, not a penitent, could profit from fragile cargo.",
    "stats": {
      "submissions": 847293,
      "correct_pct": 23.4,
      "mean_confidence_correct": 0.71,
      "mean_confidence_incorrect": 0.84,
      "most_common_wrong_option": "penitent"
    }
  },
  "clock_class": "standard",
  "deadline_ms": 90000,
  "sources": ["..."],
  "opportunities": ["..."]
}
```

This is how players learn whether they were right — from the shared
public stream, not from a private packet. Everyone sees the answer at
the same time, after the deadline when it can no longer be exploited.

The `explanation` field ties the answer to prior narrative events,
reinforcing the long-context memory game. The `stats` block creates
spectator entertainment ("84% average confidence among those who got it
wrong" is the Twitch moment).

Players who want to track their own state locally can do so
deterministically: they know their action, they know the answer key.
The score shards provide authoritative verification every score epoch.

### `POST /actions`

Authenticated action submission.

Response should be tiny:

- `202 Accepted`
- echoed `tick_id`
- server receive timestamp
- maybe a short submission receipt id

Do not return:

- score deltas
- hidden outcome details
- per-player narrative

### `GET /score-epochs/{epoch_id}/top.json`

Public top-N leaderboard snapshot.

### `GET /score-epochs/{epoch_id}/shards/{00..ff}.json.zst`

Public hashed score shards.

This replaces per-user score endpoints, but it should be treated as a low-frequency reconciliation artifact, not something every client fetches every score epoch.

Each shard contains rows like:

- `player_id`
- `score`
- `aura`
- `debt`
- `rank_if_known`
- `score_epoch`

Clients can fetch their shard if they want official authoritative state, without the origin serving personalized reads.

### `GET /reveals/{tick_id}.json.zst`

Delayed public answer reveal for a closed tick.

This endpoint explains what the server considered correct, incorrect, high-risk, or high-value for that tick after a configurable reveal lag.

## Read Path Design

The read path should be almost entirely static.

### Principles

- All players read the same objects
- Objects are immutable after publication
- The current pointer is tiny
- Clients know exactly when it will change

### Practical Rules

- Pre-render tick JSON before publication
- Compress with `gzip` and preferably `zstd`
- Serve with `ETag` and `Cache-Control`
- Make `current.json` an atomically swapped file
- Prefer immutable tick URLs over "give me the latest full payload" endpoints

### Anti-Spam Behavior

You cannot stop a malicious client from polling too often. You can make it pointless and cheap:

- `current.json` tells them exactly when to come back
- unchanged responses should be cacheable
- nginx can rate-limit obvious abuse at the edge

If a client still spam-refreshes, that becomes an abuse-control problem, not a product-design problem.

## Public Feedback Model

Removing private packets does not mean removing feedback. It means feedback must be shared and delayed.

### Layer 1: Immediate Receipt

After `POST /actions`, the client gets only:

- acceptance or rejection
- echoed `tick_id`
- server timestamp

This confirms the action was received. It does not reveal whether the action was good.

### Layer 2: Score Epoch Publication

At the score-publication cadence, defaulting to every dossier tick, the server publishes:

- top-N leaderboard
- optional score shard files at a slower checkpoint cadence

This tells players whether their recent play was working at a coarse level, without leaking per-tick hidden outcomes.

### Layer 3: Delayed Public Answer Reveal

After a reveal lag, defaulting to one score epoch, the server publishes a public reveal packet for a closed tick.

Suggested reveal payload:

```json
{
  "tick_id": "S1-T2045",
  "reveal_lag_ticks": 12,
  "resolutions": [
    {
      "opportunity_id": "quest.glass_choir.7",
      "best_known_action": {"command": "commit", "option": "broker"},
      "bad_action_classes": [
        {"command": "commit", "option": "smuggler", "outcome": "debt"},
        {"command": "hold", "outcome": "miss"}
      ],
      "public_explanation": "During the Omission phase, the Choir rewarded discretion. Broker aligned with the phase and current reputation gates."
    }
  ]
}
```

This packet is:

- public
- identical for all players
- delayed enough to weaken brute-force probing
- informative enough to support real learning and spectator comprehension

### Why This Works

- No private information channel
- No need for personalized reads
- No requirement that players infer correctness only from opaque score movement
- Lower risk of multi-account leakage than private result packets

The scoreboard tells you whether you are climbing. The reveal packet tells you why, later.

## Write Path Design

The only hot dynamic endpoint is `POST /actions`.

### Submission Rules

- one account can submit multiple times before deadline
- the last valid submission before deadline wins
- submissions after deadline are ignored
- oversized payloads are rejected immediately
- invalid schema is rejected immediately

### Hot Path Requirements

The handler should do only this:

1. authenticate token
2. map token to dense numeric `player_id`
3. validate `tick_id` and deadline
4. validate command/option bounds
5. write the submission into the current-tick slot for that player
6. append a compact WAL record
7. return `202`

No DB reads in the hot path beyond a memory-resident auth lookup.

## Authoritative Player State

Even without private packets, the server still needs per-player state for scoring.

The trick is to keep it compact.

### Recommended Representation

Assign every account a dense numeric `player_id`.

Store player state in fixed-width structs indexed by that id:

```text
PlayerState {
  score_i64
  yield_i32
  insight_i32
  aura_i32
  debt_i32
  miss_penalties_i32
  reputation[8]i16
  inventory_bits_u128
  commitment_slots[3]u32
  cooldowns_u32
}
```

Do not store this in a hot SQL table.

Keep it in memory and checkpoint it periodically.

### Memory Budget

If `PlayerState` is kept near `64-128 bytes`, then:

- `1,000,000` players cost about `64-128 MB`

Pending-action slots can be equally compact:

- `32-48 bytes` per player means another `32-48 MB`

Even with buffers, leaderboards, and shard builders, `32 GB RAM` is enough.

## Pending Action Storage

Avoid per-tick hash maps if possible.

Use:

- one dense `[]PendingAction` array indexed by `player_id`
- one `[]uint32 active_players` vector for the current tick
- one `seen_tick` marker per player to avoid duplicate active-list appends

This makes the hot path:

- one bounds check
- one shard lock
- one struct write
- maybe one append to `active_players`

That is the right shape for `11k POST/s`.

## Non-Response Handling

The server should not track "logged in" versus "logged out" as a gameplay primitive.

For the benchmark, these cases are intentionally equivalent:

- the player chose not to act
- the player disconnected
- the client crashed
- the agent timed out locally

### Deadline Rule

At tick close:

- if the player submitted a valid action before deadline, resolve the last valid submission
- otherwise, inject a synthetic `hold`

There is no separate timeout packet and no special crash recovery path in the game logic.

### Why `hold`

`hold` is the right default because it is:

- deterministic
- cheap to score
- strategically meaningful
- safer than random guessing

Missing a tick should usually be bad, but not as bad as a wildly wrong high-confidence action.

### Dormant Players

The scorer should not scan every registered player every tick.

Instead, keep three categories:

- `active_this_tick`: players who submitted something
- `scheduled_due`: players whose commitments, debt interest, cooldowns, or other timers mature on this tick
- `dormant`: everyone else

Only the first two categories are touched during tick close.

If a player is fully dormant:

- no action arrived
- no commitment matures
- no debt event is due
- no cooldown expires

then the server does nothing for that player on that tick.

This is how you avoid paying O(total_players) work for users who closed their laptop three days ago.

### Scheduled Consequences

Some player state must still advance while the player is absent:

- quest commitments finishing
- abort windows expiring
- debt interest on score epochs
- cooldown expiry

Handle these with a timing wheel or per-tick due lists keyed by `next_due_tick`.

That means:

- absent players are not scanned globally
- only players with actual due state transitions are revisited

### Rejoin Behavior

Rejoining is simple:

1. fetch `current.json`
2. fetch the current tick
3. optionally fetch recent reveal packets
4. optionally fetch the latest score shard checkpoint

No session resurrection is required because there was never a live session dependency in the first place.

### Network Jitter

Use one clear rule:

- the server receive timestamp decides whether the submission beat the deadline

If needed, add a tiny fixed grace window, such as `250-500 ms`, for network jitter. Keep it global and mechanical. Do not make it user-specific.

### Idempotency

To make crash-retry behavior safe:

- allow an optional client-generated `submission_id`
- treat duplicate `submission_id`s as retries
- keep `last valid submission before deadline wins`

This makes client restarts harmless without making the server stateful in complicated ways.

## Batch Scoring Pipeline

Never score inside the request handler.

### Tick Close

At deadline:

1. freeze the active-player list
2. hand it to worker goroutines
3. resolve each action against the immutable tick plan and current player state
4. update player state in place
5. accumulate score deltas for the next public score epoch

### Score Publication Cadence

Do not publish scores every tick.

Publish them every score epoch, defaulting to dossier cadence:

- every `12` ticks by default

Publish shard checkpoints less frequently than top-N:

- every `8` score epochs by default, or on a longer fixed interval such as every few hours

Benefits:

- less write amplification
- less read amplification
- weaker feedback loop for brute-force farms
- more suspense for spectators

Default reveal cadence:

- one score epoch after the tick closes

That is fast enough to be playable and slow enough to keep the benchmark from collapsing into instant online hill-climbing.

### Tick Plans

Precompile every tick into an immutable plan before publication:

- allowed commands
- allowed options
- phrase grammar
- scoring matrices
- precursor ids for memory-distance scoring
- narrator phase metadata

This keeps scoring O(1) per action.

## Storage Strategy

The box cannot keep every raw action forever if one million players are active.

### Keep Locally

- current player-state snapshot
- previous snapshot
- current WAL
- recent WAL segments for crash recovery
- current and recent public tick files
- recent score epochs

### Drop or Export

- very old raw action logs
- redundant per-tick intermediate state

If long-term replay of every raw action matters, move old WAL files off the box.

For the single-box version, optimize for:

- current season continuity
- crash recovery
- public reproducibility at the score-epoch level

Not for indefinite retention of every request body.

## Public Score Publication

There should be no required personalized read path.

### Publish

- `top.json` for spectators
- shard files for players
- optional daily full archive for research

### Do Not Publish

- per-tick personalized result packets
- per-user hidden state explanations
- anything that creates a second information channel

This removes the "run many sessions to gather extra private info" problem.

It does not solve Sybil competition fairness by itself. It only removes private-information leakage.

## Abuse and Fairness Controls

### Abuse Controls

- anonymous reads allowed
- authenticated writes only
- strict request body size caps
- IP token buckets on `POST /actions`
- account token rate limits
- optional emergency proof-of-work mode if ingress saturates

### Fairness Controls

Architecture alone cannot stop a determined multi-account farm.

If ranked competition matters, add one or more of:

- email or OAuth gating
- invite codes
- stake or deposit
- delayed leaderboard visibility
- limited account creation windows

The implementation should separate:

- origin tractability
- competition fairness

They are different problems.

## Capacity Expectations

### What One Box Can Do

With a `16 vCPU / 32 GB` Hetzner node and this architecture, one box should be able to:

- serve as the authoritative scheduler
- ingest actions at high rate
- batch-score a large active population
- publish shared public ticks

### What One Box Cannot Do Cheaply

A single `$50/month` box should not be expected to:

- directly serve one million uncached global readers
- keep every raw action forever
- generate custom per-user outputs
- run a websocket mesh just because it sounds modern

### Bandwidth Budget (1M players, 90-second standard tick)

With CDN (Cloudflare free tier):

| Path | Direction | Rate | Monthly |
|---|---|---|---|
| GET /ticks/* | CDN → players | ~22MB/s | absorbed by CDN |
| GET /ticks/* | origin → CDN | ~2KB per tick | negligible |
| POST /actions | players → origin | ~2.2MB/s | ~5.7TB (inbound, usually unmetered) |
| GET /score shards | CDN → players | cached | absorbed by CDN |
| **Total origin egress** | | | **< 1TB/month** |

Without CDN:

| Path | Rate | Monthly |
|---|---|---|
| GET /ticks/* direct to all players | ~22MB/s | **~57TB** |

57TB exceeds the 20TB Hetzner allowance by 3x.
**A CDN is mandatory, not optional, at 1M players.**

### Compute Budget (per standard tick)

| Operation | Per-unit | Volume | Wall time (1 core) |
|---|---|---|---|
| Validate + queue action | ~10μs | 1M | ~10s |
| Batch evaluate action | ~3μs | 1M | ~3s |
| Leaderboard sort | — | 1M entries | ~100ms |
| Score shard generation | — | 256 shards | ~200ms |

Parallelized across 16 cores: total batch time ~1-2 seconds.
Well within the 90-second tick window.

For interrupt ticks (12-second deadline): burst rate is ~83K req/s.
Go stdlib net/http handles this comfortably on 16 cores.

### Practical Reading

Without an external cache layer, the realistic ceiling is tens to low hundreds of thousands of well-behaved active readers, not one million readers hammering origin directly.

With a CDN in front of the immutable tick files, one million readers becomes straightforward while the origin remains one box.

## Recommended Runtime Layout

### Process 1: `nginx`

Responsibilities:

- TLS termination
- static file serving
- caching headers
- compression
- simple rate limiting
- reverse proxy to Go app

### Process 2: `mad-core` (Go)

Responsibilities:

- season scheduler
- `current.json` writer
- `POST /actions`
- in-memory player state
- tick close and batch scoring
- score-epoch builder

### Process 3: optional `mad-admin`

Responsibilities:

- account creation
- admin tools
- replay inspection
- season upload

This can be folded into `mad-core` for MVP.

## Implementation Phases

### Phase 1: Origin Skeleton

- `manifest.json`
- `current.json`
- immutable tick files
- nginx front

### Phase 2: Action Ingest

- auth mapping
- `POST /actions`
- pending-action array
- WAL

### Phase 3: Batch Scorer

- fixed-width player state
- tick plans
- score epochs
- public top-N leaderboard

### Phase 4: Public Shards

- shard publication
- replay snapshots
- delayed leaderboard views

### Phase 5: Season Tooling

- tick compiler
- precursor annotation for memory distance
- narrator phase tooling
- validation and dry-run simulator

## What I Would Build First

1. Keep the design doc aligned while freezing the runtime schemas
2. Freeze the wire schemas for:
   - `current.json`
   - tick packets
   - action submission
   - score epoch top file
   - score shard file
   - delayed reveal file
3. Build the tick compiler and a fake season
4. Build the ingest service with dense `player_id` arrays
5. Build score epochs before worrying about full leaderboards

If the scorer is fast and the read path stays static, everything else becomes much easier.

## Sources

- Hetzner cloud pricing: https://www.hetzner.com/cloud/
- Hetzner cloud price adjustment notice: https://docs.hetzner.com/changelog/price-adjustment-cloud/
- Plug.Cowboy docs: https://hexdocs.pm/plug_cowboy/Plug.Cowboy.html
- Phoenix Channels guide: https://hexdocs.pm/phoenix/channels.html
- Go `net/http` package: https://pkg.go.dev/net/http
