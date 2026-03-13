# MAD: Mutual Agent Destruction

A season-long benchmark for testing long-range memory and multi-step inference in LLM agents. 14K LoC Go, 250+ empirical runs.

## Key Result

Persistent agents reach **91% of the theoretical score ceiling**. Memoryless agents score at random.

| Condition | Score | payoff_gate | reputation_ladder |
|---|---:|---:|---:|
| Persistent + reveals | **+8,381** | **96%** | **81%** |
| Ephemeral (no memory) | +1,696 | 28% | 44% |
| Greedy-best ceiling | 9,256 | — | — |
| Random baseline | -2 | — | — |

Tested with Claude Haiku on a focused 90-tick season (clue + ladder + payoff families). Full results in [RESULTS.md](./RESULTS.md).

### What it measures

Each tick delivers prose narrative, structured state, and action choices. Correct decisions require remembering evidence from prior ticks — no local shortcut exists.

| Family | What the agent must do | Memory gap |
|---|---|---:|
| **payoff_gate** | Infer regime from 2 prior clue beats, pick correct market option | +68pp |
| **reputation_ladder** | Same clue recall, pick correct faction offer | +37pp |
| **hazard_interrupt** | Learn per-faction lane ROI from experience | in progress |
| **standing_work_loop** | Ambient resource building | low ceiling |

### How the memory gap was validated

1. **Eliminated local leakage** — collapsed prose to single answer-neutral templates so tick text contains zero signal about the correct answer. Probe confirms <5% leakage across all families.
2. **Conjunctive clue evidence** — regime identity is split across 2 clue beats via domain elimination. Neither beat alone identifies the correct action.
3. **Ephemeral baseline** — same season, same model, but no context history. Scores match random exactly (28% on a 3-way choice = 29% baseline).

## Architecture

MAD generates deterministic seasons from a story-element IR. Each season is a sequence of ticks containing interleaved narrative beats from multiple families. A compiler weaves elements respecting precursor dependencies, and a simulator computes offline baselines (greedy, oracle, visible-greedy, random).

The harness drives external LLM agents (Claude, Codex, OpenRouter) through a season, recording every prompt, response, and scored outcome.

```
mad-devgen → season_ir.json → mad-weave → season.json → mad-sim → baselines
                                                       → mad-harness → run reports
```

## Quick Start

Generate and compile a dev season:

```bash
go run ./cmd/mad-devgen -ticks 1000 -out ./seasons/dev1000/season_ir.json
go run ./cmd/mad-weave -ir ./seasons/dev1000/season_ir.json -out ./seasons/dev1000/season.json
```

Simulate baselines:

```bash
go run ./cmd/mad-sim -season ./seasons/dev1000/season.json -out ./build/simulation.json
```

Run Haiku against a focused season:

```bash
./scripts/mad-run --provider claude --model haiku --effort low --memory off \
  --context persistent --max-ticks 0 \
  --season ./seasons/focused-clue-ladder-payoff/season.json
```

Run the full harness with multiple models:

```bash
go run ./cmd/mad-harness \
  -season ./seasons/dev1000/season.json \
  -out ./build/harness.json \
  -runs 3 -max-ticks 100 \
  -runner claude:haiku@low \
  -runner claude:sonnet@medium
```

Run tests:

```bash
go test ./...
```

### Text ablation

The `--text-mode` flag controls how much prose the model sees:
- `full` — complete narrative (default)
- `source-types` — source type labels only, no prose
- `redacted` — all prose removed, only structured action surface

### Memory and context modes

- `--context persistent` — full context history + recent reveals
- `--context ephemeral` — each tick is one-shot, no history
- `--memory on/off` — enable/disable provider-native memory (Claude MEMORY.md, Codex memory)

## Tools

| Command | Purpose |
|---|---|
| `mad-devgen` | Generate season IR from family templates |
| `mad-weave` | Compile IR into a playable season |
| `mad-sim` | Offline simulation and baseline computation |
| `mad-harness` | Drive LLM agents through a season |
| `mad-run` | Human-friendly single-run launcher |
| `mad-probe` | Measure prose leakage per family |
| `mad-oracle-sweep` | Sweep oracle lookahead settings |
| `mad-core` | Live game server (not yet deployed) |
| `mad-loadgen` | Load test the write path |
| `mad-compile` | Compile public tick artifacts |

## Remaining Work

1. Multi-run confidence intervals (3-5 reps per condition)
2. Full 1000-tick runs (ephemeral vs persistent)
3. Multi-model comparison (Haiku, Sonnet, Opus, Codex, GPT)
4. Text ablation trio on current prose
5. Hazard family redesign (strip resource gates, test pure faction-lane learning)

## Docs

- [RESULTS.md](./RESULTS.md) — empirical findings, per-family breakdowns, design change log
- [PROPOSAL.md](./PROPOSAL.md) — original architectural blueprint
- [IMPLEMENTATION.md](./IMPLEMENTATION.md) — implementation plan and scaling analysis
- [SEASON_GENERATOR.md](./SEASON_GENERATOR.md) — story-element families and dependency rules
- [CONFIG.md](./CONFIG.md) — canonical mode definitions
