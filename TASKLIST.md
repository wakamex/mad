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

- phrase template schema
- normalization rules
- fragment-distribution rules
- decoy fragment support

## 7. Stronger Baselines

Status: pending

Goal:

- verify the skill ceiling with better comparison policies than random and greedy local play

Needed:

- naive-RAG baseline
- short-context baseline
- no-provenance baseline
- greedy-calibrated baseline

## 8. Generator Prototype

Status: pending

Goal:

- instantiate a real 1000-tick dev season from reusable element templates instead of the current synthetic tooling fixture

Needed:

- add at least one `Standing Work Loop` template with diminishing returns or cooldowns
- make that work feed multiple later branches rather than one hidden payoff
- verify the random audit still stays deeply negative once ambient work exists

## 9. Standing Work Loop Audits

Status: pending

Goal:

- make low-signal ambient work increase skill ceiling instead of degenerating into filler or grind

Needed:

- detect standing work that is always correct or always wrong
- reject standing work that feeds only one hidden mandatory outcome
- enforce diminishing returns, cooldowns, or rotation on grindable work
- ensure at least some standing work competes with obviously better short-term actions
