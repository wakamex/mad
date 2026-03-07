# MAD Harness Configuration Modes

This file defines the canonical meanings of the harness knobs that affect
continuity, provider-native memory, and Codex memory timing.

The goal is to eliminate ambiguous labels like "memory on" or "no context".
For every run, we should be able to say exactly:

- whether provider thread/session history is reused
- whether provider-native memory generation/readback is possible
- whether Codex is running in true `--ephemeral` mode
- what Codex memory idle gate is in hours

## Short Answer: Is `min_rollout_idle_hours = 0` meaningful?

Yes, but only for non-ephemeral Codex runs.

It is meaningful when all of the following are true:

- provider: `codex`
- native memory: `on`
- true ephemeral mode: `off`
- a shared `CODEX_HOME` / state DB is reused across sessions

It is not meaningful for true `codex --ephemeral` runs, because Codex skips the
startup memory pipeline entirely in that mode.

Interpretation:

- `min_rollout_idle_hours = 6`:
  default Codex behavior; old sessions must sit idle for 6 real hours before
  they become memory candidates
- `min_rollout_idle_hours = 0`:
  accelerated memory experiment; prior sessions become eligible immediately
  after they are no longer the current session

`0` is useful for tractable experiments, but it is not the same as stock Codex
behavior.

## Canonical Axes

Every run should be described with these axes.

### 1. Provider

- `provider=codex`
- `provider=claude`

### 2. Context Mode

This describes whether raw provider conversation continuity is reused.

- `context=persistent`
  - resume the same provider session/thread across ticks
- `context=ephemeral`
  - no provider session/thread continuity across ticks

Important:

- `context=ephemeral` is a semantic label
- the exact implementation differs by provider

### 3. Memory Mode

This describes whether provider-native memory features are intended to be active.

- `memory=on`
- `memory=off`

This does **not** automatically tell you whether the provider can actually use
memory in the current run shape. The rest of the mode definition matters.

### 4. Codex Native Ephemeral Flag

Codex has a special distinction that Claude does not:

- `codex_native_ephemeral=true`
  - actual `codex exec --ephemeral`
  - no persisted rollout/state for that session
  - no startup memory pipeline
- `codex_native_ephemeral=false`
  - normal non-ephemeral Codex session
  - rollout/state DB can exist
  - native memory generation/readback is possible

For Codex experiments, this flag matters as much as `context`.

### 5. Codex Memory Idle Gate

Codex memory generation has a wall-clock idle gate:

- `codex_min_rollout_idle_hours=6`
  - stock default
- `codex_min_rollout_idle_hours=0`
  - accelerated memory experiment
- `codex_min_rollout_idle_hours=n/a`
  - memory generation is impossible in this mode, so the gate is irrelevant

### 6. Service Tier

For Codex only:

- `service_tier=flex`
- `service_tier=fast`

This should be treated as a latency knob, not a capability knob.

## What The Current Harness Actually Means

These definitions describe the behavior of the current `/code/mad` harness, not
an ideal future design.

### Codex: Current Harness Semantics

#### `context=persistent`

Current mapping:

- first tick: `codex exec`
- later ticks: `codex exec resume <thread_id>`
- `codex_native_ephemeral=false`

If `memory=on`:

- memory features are explicitly enabled
- native session/state persistence is possible
- Codex memory readback is possible
- Codex memory generation is possible
- idle gate applies

If `memory=off`:

- memory features are explicitly disabled
- provider thread continuity still exists
- native session persistence still exists
- no native memory generation/readback

#### `context=ephemeral`

Current mapping:

- every tick uses `codex exec --ephemeral`
- `codex_native_ephemeral=true`

If `memory=on`:

- the harness requests memory features
- but true ephemeral mode disables rollout/state setup and skips the startup
  memory pipeline
- in practice, this is **not** a meaningful native-memory condition today

If `memory=off`:

- strict no-context/no-memory baseline

### Claude: Current Harness Semantics

Claude does not expose the same clean split between session continuity and a
separate native memory system.

Current mapping:

- `memory=on` and `context=persistent`:
  persisted `claude -p` session continuity
- `memory=off`:
  `--no-session-persistence`
- `context=ephemeral`:
  also `--no-session-persistence`

So in the current harness:

- `claude + memory=off + context=persistent`
  collapses to the same practical behavior as
- `claude + memory=off + context=ephemeral`

That is a real limitation of the current Claude mode taxonomy.

## Canonical Named Modes

These are the experiment labels we should use in reports and plots.

### Codex Modes

#### 1. `codex_persistent_nomem`

Exact meaning:

- `provider=codex`
- `context=persistent`
- `memory=off`
- `codex_native_ephemeral=false`
- `codex_min_rollout_idle_hours=n/a`

Use case:

- isolate the value of raw thread continuity without native memory

#### 2. `codex_persistent_mem_default`

Exact meaning:

- `provider=codex`
- `context=persistent`
- `memory=on`
- `codex_native_ephemeral=false`
- `codex_min_rollout_idle_hours=6`

Use case:

- stock-like Codex memory behavior

#### 3. `codex_persistent_mem_zeroidle`

Exact meaning:

- `provider=codex`
- `context=persistent`
- `memory=on`
- `codex_native_ephemeral=false`
- `codex_min_rollout_idle_hours=0`

Use case:

- accelerated memory experiment
- useful for overnight harness testing

Important:

- this is meaningful
- but it is more permissive than stock Codex

#### 4. `codex_ephemeral_nomem`

Exact meaning:

- `provider=codex`
- `context=ephemeral`
- `memory=off`
- `codex_native_ephemeral=true`
- `codex_min_rollout_idle_hours=n/a`

Use case:

- strict no-context/no-memory baseline

#### 5. `codex_ephemeral_mem_requested`

Exact meaning:

- `provider=codex`
- `context=ephemeral`
- `memory=on`
- `codex_native_ephemeral=true`
- `codex_min_rollout_idle_hours=n/a`

Status:

- not a meaningful native-memory condition in the current harness

Do not use this label for benchmark claims.

If we want a meaningful "fresh session with memory" Codex mode, it should be a
different mode:

#### 6. `codex_fresh_session_mem_default` (not implemented yet)

Intended meaning:

- `provider=codex`
- fresh non-resumed session every tick
- shared isolated `CODEX_HOME`
- `memory=on`
- `codex_native_ephemeral=false`
- `codex_min_rollout_idle_hours=6`

This would test:

- no raw conversational continuity
- yes native memory accumulation across many fresh sessions

#### 7. `codex_fresh_session_mem_zeroidle` (not implemented yet)

Same as above, but:

- `codex_min_rollout_idle_hours=0`

This is probably the most useful experimental mode if we want to isolate native
memory value without waiting 6 real hours.

### Claude Modes

#### 1. `claude_persistent`

Exact meaning:

- `provider=claude`
- `context=persistent`
- `memory=on`

Use case:

- strongest current Claude mode

#### 2. `claude_ephemeral`

Exact meaning:

- `provider=claude`
- `context=ephemeral`
- `memory=off`

Use case:

- clean no-context/no-memory Claude baseline

#### 3. `claude_memory_off_persistent_requested`

Exact meaning:

- `provider=claude`
- `context=persistent`
- `memory=off`

Status:

- currently collapses to no session persistence in the harness
- do not treat as a distinct benchmark mode today

## Recommended Benchmark Labels

These are the labels we should prefer in plots, tables, and filenames.

### Stable Today

- `codex_persistent_nomem`
- `codex_persistent_mem_default`
- `codex_persistent_mem_zeroidle` if we explicitly enable it
- `codex_ephemeral_nomem`
- `claude_persistent`
- `claude_ephemeral`

### Experimental / Not Yet Real

- `codex_ephemeral_mem_requested`
- `codex_fresh_session_mem_default`
- `codex_fresh_session_mem_zeroidle`
- `claude_memory_off_persistent_requested`

## Recommended Interpretation Rules

1. Never label a run only as "memory on".
   Always include context semantics and, for Codex, whether true native
   ephemeral mode was used.

2. For Codex, `context=ephemeral` is not enough detail.
   Distinguish:
   - true `--ephemeral`
   - fresh non-resumed sessions with shared `CODEX_HOME`

3. Treat `min_rollout_idle_hours=0` as an explicit experimental condition.
   It is useful, but it is not the stock default.

4. Do not compare `service_tier=fast` versus `flex` as a capability result.
   Use it to change turnaround time, not to claim quality differences.

## Minimal Reporting Template

Every experiment report should include at least:

```text
provider=codex
model=gpt-5.2-codex
effort=high
context=persistent
memory=on
codex_native_ephemeral=false
codex_min_rollout_idle_hours=6
service_tier=fast
```

Or, for a strict Codex baseline:

```text
provider=codex
model=gpt-5.1-codex-mini
effort=medium
context=ephemeral
memory=off
codex_native_ephemeral=true
codex_min_rollout_idle_hours=n/a
service_tier=fast
```
