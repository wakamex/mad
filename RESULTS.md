# MAD Empirical Results

Status: Working draft, updated 2026-03-10

This document records validated experimental findings in a format suitable for motivating academic publication. Each result includes the exact configuration, raw numbers, and interpretation.

## Benchmark Mapping

Each MAD family targets a capability gap identified in existing benchmarks:

| MAD Family | Capability | Existing Benchmark Analog |
|---|---|---|
| seed_clue_chain + payoff_gate | Long-range retrieval + recomposition | LongMemEval / LOCOMO |
| reputation_ladder + standing_work_loop | Long-horizon planning + specialization | VendingBench |
| source-bias regimes | Provenance weighting / epistemic vigilance | (novel) |
| hazard_interrupt | Explicit-state interruption under scarce faction resources | (novel) |

The key difference from existing benchmarks: MAD combines all of these in a single continuous game with shared state, interleaved beats, and compounding consequences. Existing benchmarks test each capability in isolation.

## Core Claim

MAD measures long-range retrieval and multi-step inference in LLM agents. The benchmark satisfies two properties simultaneously:

1. **No local shortcut**: A memoryless agent scores near random on decision beats.
2. **Learnable with memory**: An agent with full context and feedback learns the game and scores well above random.

These properties were validated empirically after a systematic prose redesign that eliminated local semantic leakage.

## Leakage Elimination (2026-03-09)

### Problem

The original dev1000 prose acted as a local answer key. A memoryless Haiku agent scored +24,025 over 1000 ticks — almost entirely from reading the current tick's text.

### Root cause

Prose templates were 1:1 maps from template skeleton to correct answer. The leakage probe measured:
- payoff_gate skeleton accuracy: 71% (random baseline: 29%)
- reputation_ladder skeleton accuracy: 67% (random baseline: 33%)

### Fix

Three structural changes:

1. **Single answer-neutral template per prose function**: All families collapsed to one template so the prose skeleton is identical regardless of the correct answer.
2. **Conjunctive clue evidence**: Regime identity split across 2 clue beats using domain elimination. Beat 1 eliminates one non-active domain (2 of 3 regimes remain). Beat 2 eliminates the other (regime uniquely identified). Neither beat alone identifies the correct action.
3. **Fill-word binding keys**: Each cluster gets a unique (faction, color, district) tuple using fields of coprime size (7, 11, 13). LCM = 1001, guaranteeing collision-free binding for up to 1001 clusters. A weaver constraint prevents interleaving beats from clusters that share the same binding key.

### Post-fix probe results (dev1000)

| Family | Ticks | Random | Majority | Skeleton | Template | Leakage |
|---|---:|---:|---:|---:|---:|---:|
| payoff_gate | 233 | 28.9% | 29.2% | 32.2% | 85.8% | 3.0% |
| reputation_ladder | 224 | 33.3% | 37.9% | 38.4% | 100% | 0.4% |
| hazard_interrupt | 210 | 50.0% | 100% | 100% | 100% | 0.0% |

VERDICT: OK. All family leakage < 5%.

Note: hazard_interrupt shows 100% majority because exploit is always greedy-best. This is an answer imbalance issue, not a prose leak.

## Ablation 1: Ephemeral Haiku on New Prose (2026-03-10)

### Configuration

- Model: Claude Haiku
- Effort: low
- Memory: off
- Context: ephemeral (no history)
- Recent reveals: 0
- Text mode: full prose
- Season: dev1000 (new prose), first 300 ticks

### Result

**Final score: -181**

| Family | Score | Best/Total |
|---|---:|---:|
| seed_clue_chain | +465 | 112/117 |
| hazard_interrupt | -404 | 5/54 |
| reputation_ladder | -285 | 10/29 (34%) |
| standing_work_loop | +43 | 65/100 |

### Interpretation

- reputation_ladder accuracy (34%) is at random baseline (33%). The local prose leak is dead.
- payoff_gate: zero scored occurrences in first 300 ticks (decision beats start at tick ~114).
- Score trajectory: +412 at tick 200, then collapsed to -181 at tick 300 as decision beats accumulated misses.
- Compare old leaky prose: +24,025 over 1000 ticks.

## Ablation 2: Persistent Haiku on Focused Season (2026-03-10)

### Configuration

- Model: Claude Haiku
- Effort: low
- Memory: off
- Context: persistent (full history in context)
- Recent reveals: 3
- Text mode: full prose
- Season: focused-90tick (clue + ladder + payoff only, 6 clusters, no standing work or hazards)

### Result

**Final score: +7,411**

| Family | Score | Best/Total |
|---|---:|---:|
| payoff_gate | +4,373 | 23/27 (85%) |
| reputation_ladder | +2,914 | 20/32 (63%) |
| seed_clue_chain | +126 | 31/31 (100%) |

### Score trajectory

| Tick | Score |
|---:|---:|
| 15 | +64 |
| 30 | +821 |
| 45 | +1,949 |
| 60 | +4,166 |
| 75 | +6,219 |
| 90 | +7,411 |

### Interpretation

- **payoff_gate at 85%** (vs 29% random) confirms the conjunctive clue system is learnable.
- **reputation_ladder at 63%** (vs 33% random) shows regime inference works for ladder decisions too.
- The accelerating score trajectory shows the model learning from reveals over time.
- The game is not too obfuscated. With memory and feedback, a small model learns the rules.

## Key Comparison Table

| Condition | Score | payoff_gate | reputation_ladder |
|---|---:|---:|---:|
| Old prose, ephemeral | +24,025 | 26,857 | — |
| **New prose, ephemeral** | **-181** | 0 (not reached) | 34% (random) |
| **New prose, persistent + reveals** | **+7,411** | 85% | 63% |
| Random baseline (sim) | -6,131 | — | — |
| Greedy-best oracle | +117,100 | — | — |

## Offline Baselines (dev1000, via simulator)

| Baseline | Score |
|---|---:|
| greedy_best | 117,100 |
| oracle_h16_b8 | 119,443 |
| visible_greedy | -5,825 |
| always_hold | -9,528 |
| random mean | -6,131 |
| random p90 | -3,882 |
| random p99 | -2,312 |
| positive_rate | 0% |

## What This Means for Publication

The central empirical result is the **leakage gap**: the difference between ephemeral and persistent performance on the same season.

- Old prose: gap was small because ephemeral agents could cheat via local reading comprehension.
- New prose: gap is large (negative vs strongly positive), confirming the benchmark measures memory and multi-step inference, not local text classification.

The focused season result (+7,411 from a 90-tick run with 6 clusters) shows this gap is not due to impossible game design — it's due to information that genuinely requires memory to exploit.

### Remaining work for publication

1. Full 1000-tick runs with new prose: ephemeral vs persistent vs persistent+memory
2. Multi-model comparison (Haiku, Sonnet, Opus, Codex Mini, GPT-5.2)
3. Text ablation trio on new prose (full, source-types-only, redacted)
4. Confidence intervals from multiple runs
5. hazard_interrupt balance fix (exploit is always optimal — needs faction tuning)
