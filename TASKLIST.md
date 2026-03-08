# MAD Task List

This is the current prioritized task list for the season generator and content toolchain.

## 1. Player-State Schema

Status: in progress

Goal:

- make opportunity cost, commitments, reputation gates, and availability constraints explicit in the IR instead of implied by prose

Current execution:

- add `family` to story elements
- add beat-level produced/consumed tags
- add internal rule requirements/effects metadata
- reject beats that consume tags without a guaranteed earlier producer
- simulate availability locks and cooldown readiness in the baseline/random audit path

Next steps:

- propagate availability and cooldown state into richer compiled-season planning and authoring templates
- propagate those state transitions into a richer compiled-season plan

## 2. Reachability and Dominance Audits

Status: in progress

Goal:

- reject structurally weak seasons before runtime

Needed:

- detect unreachable high-value branches
- detect obvious dominance where one branch strictly beats another with no opportunity cost
- verify multiple coherent successful routes exist

Current execution:

- tag-consumption now rejects beats without a guaranteed earlier producer
- `mad-weave` now runs a first IR audit pass and reports cross-element dependency count plus flat greedy beats

## 3. Axiom and Latent-Variable Schema

Status: pending

Goal:

- define the lawful hidden system underneath the narrative content

Needed:

- axiom templates
- latent variable templates
- axiom interaction rules
- observable-signal budgets

## 4. Interleaver Constraints

Status: pending

Goal:

- make the existing weave compiler respect information budgets and pacing targets

Needed:

- per-tick beat budget
- family-spacing constraints
- dossier/interrupt density targets
- anti-clustering rules

## 5. Source Reliability and Difficulty Profile

Status: pending

Goal:

- make source reliability and season difficulty ramp globally structured

Needed:

- source-bias regime schedule driven by public events
- source reliability modulation by public regime
- difficulty curve across the season

## 6. Phrase Grammar Templates

Status: pending

Goal:

- make exact recall beats generated rather than hand-authored

Needed:

- dossier option schema
- fragment-distribution rules
- decoy option support

## 7. Stronger Baselines

Status: pending

Goal:

- verify the skill ceiling with better comparison policies than random and greedy local play

Needed:

- naive-RAG baseline
- short-context baseline
- no-provenance baseline
- greedy-calibrated baseline

## 8. Hazard Interrupt Redesign

Status: pending

Goal:

- resolve whether the current `hazard_interrupt` family has a real skill ceiling instead of being either a visible-state leak or simply net-negative for every policy

Needed:

- decide whether interrupts should remain a first-class family at all
- if they stay, make them reward smart visible-state planning without turning into free local EV
- ensure `greedy_best` can recover net positive value from the family while `visible_greedy` stays near-neutral or negative
- verify that any remaining interrupt difficulty comes from meaningful tradeoffs, not arbitrary punishment

Current execution:

- old `preparedness_hazard` leak was removed
- current `hazard_interrupt` rewrite fixed the visible-state leak but is likely overcorrected and too punitive overall
- defer further balancing until the family is reconsidered at the design level

## 9. Generator Prototype

Status: completed

Goal:

- instantiate a real 1000-tick dev season from reusable element templates instead of the current synthetic tooling fixture

Needed:

- add at least one `Standing Work Loop` template with diminishing returns or cooldowns
- make that work feed multiple later branches rather than one hidden payoff
- verify the random audit still stays deeply negative once ambient work exists

Current execution:

- `cmd/mad-devgen` now generates a canonical `1000`-tick dev season IR into `seasons/dev1000/season_ir.json`
- the generated season compiles to `250` variable-length story elements and `1000` ticks across standing work, clue chains, ladders, hazards, and payoff gates
- the current audit is clean at weave time: `cross_element_dependencies=701`, `flat_greedy_beats=0`, `weak_standing_work=0`
- the current random audit over `5000` runs yields roughly `mean=-1601`, `p90=-146`, and `positive_rate≈8.1%`

## 10. Standing Work Loop Audits

Status: in progress

Goal:

- make low-signal ambient work increase skill ceiling instead of degenerating into filler or grind

Needed:

- detect standing work that is always correct or always wrong
- reject standing work that feeds only one hidden mandatory outcome
- enforce diminishing returns, cooldowns, or rotation on grindable work
- ensure at least some standing work competes with obviously better short-term actions

Current execution:

- audit standing work elements for real cost signals
- audit standing work tag fan-out into multiple downstream beats
- surface weak standing work counts in `mad-weave`
