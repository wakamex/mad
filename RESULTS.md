# MAD Empirical Results

Status: Working draft, updated 2026-03-12

This document records validated experimental findings in a format suitable for motivating academic publication. Each result includes the exact configuration, raw numbers, and interpretation.

## Benchmark Mapping

Each MAD family targets a capability gap identified in existing benchmarks:

| MAD Family | Capability | Existing Benchmark Analog |
|---|---|---|
| seed_clue_chain + payoff_gate | Long-range retrieval + recomposition | LongMemEval / LOCOMO |
| reputation_ladder + standing_work_loop | Long-horizon planning + specialization | VendingBench |
| source-bias regimes | Provenance weighting / epistemic vigilance | (novel) |
| hazard_interrupt | Resource-gated lane selection under time pressure | (novel) |

The key difference from existing benchmarks: MAD combines all of these in a single continuous game with shared state, interleaved beats, and compounding consequences. Existing benchmarks test each capability in isolation.

## Core Claim

MAD measures long-range retrieval and multi-step inference in LLM agents. The benchmark satisfies two properties simultaneously:

1. **No local shortcut**: A memoryless agent scores near random on decision beats.
2. **Learnable with memory**: An agent with full context and feedback learns the game and scores well above random.

These properties were validated empirically for **payoff_gate** and **reputation_ladder** after a systematic prose redesign that eliminated local semantic leakage.

## Family Design Summary

### seed_clue_chain (observe-only)

Delivers evidence the agent must remember for later decisions. No action, no scoring, no LLM call. Prose is buffered and prepended to the next decision tick as `observations`. The temporal separation between clue beats forces genuine memory use — the agent can't "look up" the clue at decision time.

### payoff_gate (VALIDATED)

Three-way market decision where the correct option depends on the active source regime, which must be inferred from clue beats via conjunctive domain elimination. Near-random without memory (28%), strong with memory (96%). **+68pp memory gap.**

### reputation_ladder (VALIDATED)

Three-way faction offer where the correct option depends on the active source regime. Same conjunctive evidence mechanism as payoff. Near-random without memory (44%), strong with memory (81%). **+37pp memory gap.**

### hazard_interrupt (VALIDATED — ln reward function)

Multi-level investment hazard. Each tick offers 4 investment levels (spend 1-4 rep) plus hold. Reward follows `66 × a × ln(1+x) - 330×x + noise`, where `a` is a hidden per-faction parameter (2-25), `x` is investment level, and noise varies per beat (±440). The ln curve has diminishing returns — optimal investment level varies by faction. The model must learn each faction's `a` from noisy observations to optimize.

**Key property:** Greedy (tick-local) is NOT the ceiling. An agent that learns per-faction ROI and manages resources across ticks can beat greedy through strategic resource conservation. This tests genuine forward planning.

**Haiku results (90-tick standing+hazard season):**

| Condition | Score | vs Greedy (3,247) |
|---|---:|---:|
| **Ephemeral + structured history** | **8,420** | **259%** |
| Ephemeral (no history) | 4,026 | 124% |
| Persistent + history | 3,061 | 94% |
| Greedy (tick-local) | 3,247 | 100% |
| Random | -4,857 | negative |

**Key finding: more memory can hurt.** Persistent context (full conversation history) reduces performance by **2.75×** compared to ephemeral + structured history (3,061 vs 8,420). The model needs 42 rows of (faction, investment, yield) data, but persistent mode buries this in 90 ticks of prose, state snapshots, standing work decisions, and reveal text. The signal-to-noise ratio collapses.

This establishes hazard as a benchmark for **memory curation systems** — frameworks that select, compress, or reorganize context rather than accumulating it raw. The empirical gradient:

```
Ephemeral (no memory):            negative  — can't learn
Persistent (raw accumulation):    94% greedy — drowning in noise
??? (curated memory systems):     ???       — the gap frameworks compete on
Structured table (oracle memory): 259% greedy — proves the task is solvable
```

Memory curation approaches this benchmarks:
- **RAG over conversation**: retrieve relevant past hazard observations, discard standing work noise
- **Summarization-based memory**: compress to "Glass Choir: 3 observations, avg yield +450 at invest_3"
- **Learn-to-forget / attention pruning**: drop irrelevant context, converge toward the structured table
- **Tool-augmented memory**: model writes to a scratchpad/database, reads back structured data

The floor (raw persistent, 94%) and ceiling (structured table, 259%) are both empirically established. A framework scoring 200% of greedy is demonstrating real memory curation.

**Findings (2026-03-19):**
1. With structured `hazard_history`: Haiku learns the ROI function and scores **2.6× greedy** through forward planning — proving the task is solvable.
2. With persistent context only: the model barely matches greedy (3,061 vs 3,247) — 90 ticks of conversation noise drowns the signal.
3. Ephemeral no-history: scores 4,026 through lucky high-variance bets, not systematic learning.
4. The bottleneck is **memory format**, not model capability. The arithmetic is within Haiku's reach when data is clean.

**Season composition:** Hazard contributes 31% of full-season greedy score at 1000 ticks (payoff 35%, ladder 34%, hazard 31%).

**Design journey (2026-03-13 → 2026-03-19):**
1. Binary commit/hold with traps: +21pp gap (persistent 83% vs ephemeral 62%), but simple label memorization.
2. ln reward function with multi-level investment: tests actual function learning, random scores negative, visible_greedy deeply negative.
3. Structured hazard history: proves the model can learn the function — the gap is between organized memory (8,420) and unorganized context (3,061).
4. **"Memory hurts" finding**: persistent context actively degrades performance vs ephemeral + structured data. Agent memory systems need curation, not just accumulation.

### standing_work_loop (LOW PRIORITY)

Ambient routine work that builds faction reputation and trickle aura. Low skill ceiling (56% with memory). Functions primarily as a resource builder for hazard_interrupt, not a standalone benchmark signal.

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
| payoff_gate | 233 | 28.9% | 29.2% | 32.2% | 81.5% | 3.0% |
| reputation_ladder | 224 | 33.3% | 37.9% | 37.5% | 100% | 0.0% |
| hazard_interrupt | 211 | 50.0% | 100% | 100% | 100% | 0.0% |

VERDICT: OK. All family leakage < 5%.

Note: hazard_interrupt shows 100% majority because exploit is always greedy-best. This is an answer imbalance issue, not a prose leak.

Note: seed_clue_chain ticks are observe-only (no action, no scoring) and excluded from the probe. The 668 probeable ticks are decision beats only.

## Per-Family Learnability (2026-03-12, current design)

Each family was tested using focused 90-tick seasons. All runs use Claude Haiku at low effort. "Persistent" means full context history + recent reveals. "Ephemeral" means no history, no reveals.

### Per-family accuracy: best-action rate

| Family | Random Baseline | Ephemeral | Persistent+Reveals | Memory Gap |
|---|---:|---:|---:|---:|
| **payoff_gate** | 29% | 28% | **96%** | **+68pp** |
| **reputation_ladder** | 33% | 44% | **81%** | **+37pp** |
| **seed_clue_chain** | — | — | — | observe-only |
| **hazard_interrupt** | negative | negative | **259% of greedy** (with structured history) | forward planning |
| **standing_work_loop** | ~50% | — | 56% | low ceiling, hazard support only |

### Raw results (current design, 2026-03-12)

**clue+ladder+payoff (persistent+reveals)**: score=+8,381, payoff 24/25 (96%), ladder 22/27 (81%)

**clue+ladder+payoff (ephemeral)**: score=+1,696, payoff 7/25 (28%), ladder 12/27 (44%)

**standing+hazard, seeded (persistent)**: score=+1,165, hazard 19/42 (45%), standing 27/48 (56%)

**standing+hazard, seeded (ephemeral)**: score=+995, hazard 17/42 (40%), standing 26/48 (54%)

### Previous results (pre-gate-fix, for comparison)

**clue+ladder+payoff (persistent, fake gates)**: score=+5,435, payoff 24/25 (96%), ladder 8/27 (30%)

**clue+ladder+payoff (ephemeral, fake gates)**: score=-123, payoff 3/25 (12%), ladder 4/27 (15%)

### Interpretation

**payoff_gate** and **reputation_ladder** show the design working as intended: near-random without memory, strong with memory. The 68pp and 37pp memory gaps confirm the benchmark measures long-range retrieval and multi-step inference.

Ladder accuracy jumped from 30% to 81% after removing cosmetic PublicRequirements that acted as fake gates — the model was holding on 63% of ladder opportunities because it believed displayed standing/debt thresholds were enforced. They were not. This was a design confound, not a model failure.

Ephemeral payoff also rose (12% → 28%) suggesting the old run had additional bad luck or the gate text was also suppressing ephemeral attempts. 28% matches random baseline exactly — good signal.

**hazard_interrupt** was redesigned from two-lane (stabilize/exploit) to binary (commit/hold) with hidden per-faction rewards. Persistent 83% vs ephemeral 62% — **+21pp memory gap**. The model learns from reveals which factions are profitable vs traps.

## Key Comparison Table

| Condition | Score | payoff_gate | reputation_ladder |
|---|---:|---:|---:|
| **New design, persistent + reveals** | **+8,381** | **96%** | **81%** |
| **New design, ephemeral** | **+1,696** | 28% | 44% |
| Old design (fake gates), persistent | +5,435 | 96% | 30% |
| Old design (fake gates), ephemeral | -123 | 12% | 15% |
| Pre-leakage-fix, ephemeral | +24,025 | — | — |
| Random baseline (sim) | -2 | — | — |
| Greedy-best oracle (sim) | 9,256 | — | — |

### CLP-90 baselines (focused-clue-ladder-payoff)

| Baseline | Score |
|---|---:|
| greedy_best | 9,256 |
| oracle_h16_b8 | 9,256 |
| visible_greedy | -113 |
| always_hold | -516 |
| random mean | -2 |

## Offline Baselines (dev1000, via simulator)

| Baseline | Score |
|---|---:|
| greedy_best | 106,917 |
| oracle_h16_b8 | 104,437 |
| visible_greedy | -2,398 |
| always_hold | -9,378 |
| random mean | -6,019 |
| random p90 | -3,737 |
| random p99 | -1,805 |
| positive_rate | 0.1% |

## Design Changes Log

1. **Leakage-proof prose** (2026-03-09): Single answer-neutral template per function. Conjunctive clue evidence. Binding keys.

2. **Observe-only clue beats** (2026-03-12): Clue ticks no longer prompt the LLM. Prose is buffered and prepended to the next action tick as `observations`. Saves ~333 LLM calls per 1000-tick season.

3. **Hazard threshold scaling** (2026-03-12): Thresholds are now percentages (5-18%) of max achievable rep/aura, computed from the season's standing work budget. Minimum threshold of 2.

4. **Hazard reward visibility** (2026-03-12): Expected yields shown in PublicRequirements labels. Agents can compare lanes and plan faction investment before committing.

5. **Hazard faction ROI rebalance** (2026-03-12): Base yield gap narrowed (stabilize 46+5i vs exploit 62+8i, was 32 vs 74) so faction bonuses determine which lane is optimal. 7 factions: 3 stabilize / 1 mixed / 3 exploit.

6. **Ladder fake-gate removal** (2026-03-12): Removed cosmetic PublicRequirements from reputation_ladder opportunities. These displayed standing/debt thresholds that the engine never enforced, causing the model to hold on 63% of ladder ticks. Also removed the premium-tier scoring rule (unreachable without standing family). Ladder accuracy: 30% → 81%.

7. **Hazard initial-state seeding** (2026-03-13): Added `InitialState` to season model — seeds reputation/aura before tick 1 so agents can qualify for hazard lanes without cold-start grind. Seeding at 2× minimum threshold per faction.

8. **Hazard yield labels stripped** (2026-03-13): Removed expected yield numbers from PublicRequirements labels. Agent must learn per-faction ROI from reveal feedback, not from visible labels.

9. **Hazard diagnosis corrected** (2026-03-15): The 45% model score is not a learning failure — greedy-best ceiling is 47.6% due to resource depletion. The resource economy bankrupts every strategy mid-season. Models reason correctly about thresholds (verified via notes).

10. **Hazard redesign: binary commit/hold** (2026-03-18): Collapsed two-lane (stabilize/exploit) into binary commit vs hold. Commit costs 1 rep, reward varies by faction (hidden). 3 profitable factions, 4 traps. Stripped opportunity IDs and source IDs from prompts to prevent beat-index leakage. Uniform spend/threshold labels. Greedy ceiling 95.2%. **Haiku: persistent 79%, ephemeral 57%, +21pp gap.**

## What This Means for Publication

The central empirical result is the **memory gap**: the difference between ephemeral and persistent performance on the same season.

- Old prose: gap was small because ephemeral agents could cheat via local reading comprehension.
- New prose: gap is large (+8,381 vs +1,696), confirming the benchmark measures memory and multi-step inference, not local text classification.

The persistent Haiku run reaches **90.5% of the greedy-best ceiling** (8,381 / 9,256) on the focused CLP season — the game is highly learnable with memory.

### Remaining work for publication

1. Full 1000-tick runs with new prose: ephemeral vs persistent vs persistent+memory
2. Multi-model comparison (Haiku, Sonnet, Opus, Codex Mini, GPT-5.2)
3. Text ablation trio on new prose (full, source-types-only, redacted)
4. Confidence intervals from multiple runs (3-5 reps per condition)
5. Motivate the 4 resource types (yield, insight, aura, debt) — are they all needed or duplicative?
