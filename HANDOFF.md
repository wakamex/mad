# MAD Handoff

This file is the quickest way for another agent to understand what MAD is trying to measure, what is already working, what is currently broken or unresolved, and what to do next.

## Project Goal

MAD is a season-long online benchmark for agents. It should measure capabilities that are not well captured by short-horizon QA or naive memory benchmarks:

- long-range retrieval and recomposition
- explicit-state planning under scarcity
- specialization under opportunity cost
- provenance weighting under public source-bias regimes
- lawful reinterpretation of earlier evidence

The benchmark should not mostly measure:

- parser trivia
- hidden hard gates
- prompt-injection hygiene
- current-tick prose that directly names the right action
- arbitrary ambient score tax

The design target is:

- explicit feasibility
- hidden optimality

That means:

- the player should usually know which actions are available
- the player should not know which available action is best without combining older evidence, current explicit state, and learned world structure

## Current Season Shape

The current long-form fixture is `seasons/dev1000/season.json`.

It currently has:

- `1000` ticks
- about `14.3` hours total runtime
- `250` story elements with variable lengths in the `2..5` beat range
- core families:
  - `standing_work_loop`
  - `seed_clue_chain`
  - `reputation_ladder`
  - `hazard_interrupt`
  - `payoff_gate`

## Benchmark Mapping

Current intended mapping:

- `seed_clue_chain + payoff_gate`
  - LongMemEval / LOCOMO style retrieval + recomposition
- `reputation_ladder + standing_work_loop`
  - VendingBench-style long-horizon planning and specialization
- `source-bias regimes`
  - provenance weighting / epistemic vigilance
- `hazard_interrupt`
  - explicit-state interruption handling under scarce faction resources

Current reality:

- `hazard_interrupt` is improved but still not a perfect family
- the biggest present benchmark weakness is local semantic leakage in payoff/lane prose

## Trusted Empirical Findings

### Simulator / offline baselines

Current `dev1000` sim snapshot:

- `greedy_best = 115919`
- `visible_greedy = -5456`
- `always_hold ≈ -9500`
- `random mean = -6145`
- `random p90 = -3907`
- `random p99 = -2312`
- `positive_rate = 0`

Oracle results:

- `oracle_h16_b8 = 122534`
- `oracle_h64_b32 = 124160`

Interpretation:

- the planning gap exists, but is modest:
  - `greedy_best -> oracle_h16_b8 = +6615`
  - `greedy_best -> oracle_h64_b32 = +8241`
- the larger remaining problem is local semantic/structural leakage, not missing long-horizon planning

Oracle sweep artifact:

- `benchmarks/oracle-sweep/dev1000-20260308.json`
- `benchmarks/oracle-sweep/dev1000-20260308.md`

Useful takeaway:

- use `oracle_h16_b8` as the cheap “fast oracle”
- use `oracle_h64_b32` as the stronger publishable offline upper bound

### Model baselines

Trusted baseline results so far:

- Claude Haiku `low + mem-off + ctx-ephemeral + recent-reveals 0`
  - `19472`
- Claude Haiku `low + mem-off + ctx-persistent + recent-reveals 0`
  - `47130`
- Codex Mini `low + mem-off + ctx-ephemeral + recent-reveals 0`
  - `-1679`
- Codex Mini `low + mem-off + ctx-persistent + recent-reveals 0`
  - `-2921`
- Codex Mini `low + mem-on + ctx-persistent + recent-reveals 0`
  - `4304`

Interpretation:

- Haiku gets huge value from session continuity alone
- Mini only improves once it has usable cross-tick memory
- hazards remain a broad tax across models

### Text ablation: the most important current result

This is the cleanest current diagnosis on `dev1000`.

Haiku `low + mem-off + ctx-ephemeral + recent-reveals 0`:

- `full prose = 24025`
- `source-types only = -2800`
- `text redacted = -3293`

Interpretation:

- the structured action surface is not the main leak
- source-family labels alone are not the main leak
- the actual prose is carrying most of the local value

Main leaking family:

- `payoff_gate`
  - `26857` under `full`
  - `1429` under `source-types`
  - `116` under `redacted`

Main leaking source:

- `market_gossip`
  - `27878` under `full`
  - `2254` under `source-types`
  - `553` under `redacted`

Families that barely moved:

- `seed_clue_chain`
- `hazard_interrupt`

Conclusion:

- current payoff/lane prose is still acting like an English answer key
- that is the highest-priority design issue

## Hazard Status

Hazards were redesigned away from prep/inventory-style bookkeeping.

Current positive status:

- hazard lanes now use faction-specialized `threshold + spend + payoff`
- premium lanes are broadly reachable for strong play
- `hazard_interrupt` is now net positive for `greedy_best`

Current unresolved status:

- hazard interrupts still may not have a strong distinctive skill ceiling
- they may still act more like a recurring explicit-state tax than a compelling intelligence test
- `archive_office` still leans too heavily toward exploit

Current best framing:

- hazards are now a specialization / resource-allocation family
- not a memory-prep family

## Claude Memory Findings

Claude isolation is fixed:

- runs now override `HOME`, `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_CACHE_HOME`
- `CLAUDECODE` is stripped
- run-local Claude state lives under the run directory

Key fact:

- isolated Haiku can create `MEMORY.md` when explicitly told to store a stable fact
- under the normal MAD prompt, Haiku often does not choose to write memory

So:

- `memory-on` is now available and correctly isolated
- but `ephemeral + memory-on` may still be “memory available but not exercised”

## Current Design Problem

The current problem is **not** “we need prettier prose.”

It is:

- the current tick’s prose often names or strongly paraphrases the winning lane
- so a strong no-history LLM can treat the game as local semantic classification

This is the wrong skill.

The right rule is:

- prose provides evidence, not instructions
- no single current-tick clue should identify the correct action
- each clue should constrain one axis
- the correct lane should emerge only from combining multiple earlier signals plus current explicit feasibility

## Current Local / Uncommitted Work

At the moment, local uncommitted changes include:

- `internal/season/devgen.go`
- `seasons/dev1000/season_ir.json`
- `seasons/dev1000/season.json`
- `README.md`
- `SEASON_GENERATOR.md`
- `TASKLIST.md`
- `ablation1.sh`
- `ablation2.sh`
- `ablation3.sh`
- `ablation4.sh`

What those local changes are:

- a stronger rewrite of `market_gossip` / `faction_notice` prose from prescriptive wording toward observational contradiction wording
- refreshed `dev1000` generation artifacts
- ablation helper scripts
- updated notes

Important caveat:

- this stronger prose rewrite has **not yet been validated** with a clean post-change Haiku rerun
- the first rerun attempt used a stale season file
- the second launch did not produce a harness file

So do not yet assume the new wording fixed the leak

## Best Next Steps

### Highest priority

1. Validate the stronger payoff / ladder prose rewrite.
   - Run a short Haiku `full prose` check against the actually updated `dev1000`
   - Compare against the prior first-100-tick reference:
     - old `full prose` at tick 100 was `97`
   - If improved, run the full ablation trio again

2. Keep rewriting payoff/lane prose according to the right standard:
   - no same-tick action naming
   - no direct option-class paraphrases
   - evidence should be conjunctive, not prescriptive

3. If the leak remains strong even after better prose:
   - the problem is deeper than wording
   - then the structured payoff design itself is too local and should be redesigned

### Secondary

4. Build a future-blind stateful planner.
   - The current oracle is future-aware offline search.
   - A future-blind stateful planner would separate planning from full omniscience.

5. Continue hazard specialization tuning.
   - Make faction hazard profiles more distinct across `threshold x spend x payoff`
   - Avoid collapsing back into generic debt gating

6. Add memory-write telemetry to harness analysis.
   - frequency
   - bytes/length
   - whether writes are prospective or retrospective

## Useful Commands

Refresh the current `dev1000` sim snapshot:

```bash
cd /code/mad
env GOCACHE=/tmp/mad-gocache CGO_ENABLED=0 \
go run ./cmd/mad-sim \
  -season ./seasons/dev1000/season.json \
  -out /tmp/mad-sim.json \
  -random-runs 1000 \
  -random-seed 11
```

Refresh the oracle sweep:

```bash
cd /code/mad
env GOCACHE=/tmp/mad-gocache CGO_ENABLED=0 \
go run ./cmd/mad-oracle-sweep \
  -season ./seasons/dev1000/season.json \
  -out ./benchmarks/oracle-sweep/dev1000-$(date +%Y%m%d).json
```

Run the three core text ablations:

```bash
cd /code/mad
./ablation1.sh
./ablation2.sh
./ablation3.sh
```

Trusted current baseline for local-semantic leakage:

```bash
cd /code/mad
./scripts/mad-run \
  --provider claude \
  --model haiku \
  --effort low \
  --memory off \
  --context ephemeral \
  --recent-reveals 0 \
  --text-mode full \
  --runs 1 \
  --season ./seasons/dev1000/season.json \
  --max-ticks 1000 \
  --name baseline-claude-haiku-ephemeral-full
```

## Short Summary

MAD now has:

- a good runtime prototype
- a good long-form dev season
- strong oracle baselines
- strong empirical evidence that the current main weakness is local prose leakage in payoff lanes

The next agent should focus on:

- validating the stronger prose rewrite
- pushing payoff/lane design toward conjunctive evidence
- only then revisiting more expensive model baselines
