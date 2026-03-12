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

### hazard_interrupt (PARTIALLY VALIDATED)

Two-lane interrupt event. Each lane has explicit resource gates (reputation or aura thresholds) and visible expected yields. The agent must:
1. Read its current state to see which lanes it qualifies for
2. Compare visible yields to pick the better lane
3. Learn per-faction which lane has better ROI (stabilize vs exploit)
4. Plan resource investment across ticks to unlock high-value lanes

**Faction ROI split (2026-03-12):** Base yields narrowed (stabilize 46+5i, exploit 62+8i, was 32/74) so faction bonuses determine which lane wins. 7 factions split 3 stabilize-favored / 1 mixed / 3 exploit-favored. Civic_ward crosses over at beat 2 (stabilize early, exploit late). Greedy sim best-action split: 38 stabilize, 182 exploit, 24 ineligible.

**Haiku result (persistent, 90-tick):** score=+37, hazard 19% (8/42), standing 56% (27/48). First positive score — above random — but best-action rate still low. Hazard requires cross-tick planning that Haiku at low effort may not support.

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
| **hazard_interrupt** | 50% | — | 19% | needs stronger model |
| **standing_work_loop** | ~50% | — | 56% | low ceiling |

### Raw results (current design, 2026-03-12)

**clue+ladder+payoff (persistent+reveals)**: score=+8,381, payoff 24/25 (96%), ladder 22/27 (81%)

**clue+ladder+payoff (ephemeral)**: score=+1,696, payoff 7/25 (28%), ladder 12/27 (44%)

**standing+hazard (persistent+reveals)**: score=+37, hazard 8/42 (19%), standing 27/48 (56%)

### Previous results (pre-gate-fix, for comparison)

**clue+ladder+payoff (persistent, fake gates)**: score=+5,435, payoff 24/25 (96%), ladder 8/27 (30%)

**clue+ladder+payoff (ephemeral, fake gates)**: score=-123, payoff 3/25 (12%), ladder 4/27 (15%)

### Interpretation

**payoff_gate** and **reputation_ladder** show the design working as intended: near-random without memory, strong with memory. The 68pp and 37pp memory gaps confirm the benchmark measures long-range retrieval and multi-step inference.

Ladder accuracy jumped from 30% to 81% after removing cosmetic PublicRequirements that acted as fake gates — the model was holding on 63% of ladder opportunities because it believed displayed standing/debt thresholds were enforced. They were not. This was a design confound, not a model failure.

Ephemeral payoff also rose (12% → 28%) suggesting the old run had additional bad luck or the gate text was also suppressing ephemeral attempts. 28% matches random baseline exactly — good signal.

**hazard_interrupt** scored +37 (19% best-action), Haiku's first positive hazard score. The old design scored -1,800 (0%). Progress is real but the family likely needs a stronger model to show clear learning.

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

## What This Means for Publication

The central empirical result is the **memory gap**: the difference between ephemeral and persistent performance on the same season.

- Old prose: gap was small because ephemeral agents could cheat via local reading comprehension.
- New prose: gap is large (+8,381 vs +1,696), confirming the benchmark measures memory and multi-step inference, not local text classification.

The persistent Haiku run reaches **90.5% of the greedy-best ceiling** (8,381 / 9,256) on the focused CLP season — the game is highly learnable with memory.

### Remaining work for publication

1. Full 1000-tick runs with new prose: ephemeral vs persistent vs persistent+memory
2. Multi-model comparison (Haiku, Sonnet, Opus, Codex Mini, GPT-5.2)
3. Text ablation trio on new prose (full, source-types-only, redacted)
4. Confidence intervals from multiple runs
5. Decide: fix hazard_interrupt further or cut it from the core claim
6. Motivate the 4 resource types (yield, insight, aura, debt) — are they all needed or duplicative?
