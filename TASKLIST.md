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
- redesign hazard lanes around stable, learnable faction archetypes
- replace endlessly rising hazard thresholds with `stable threshold + spend`
- make faction standing scarce enough that a player cannot keep all hazard lanes open at once
- bias generator tuning toward asymmetric faction ROI so some factions are intentionally more worth grinding than others
- calibrate hazard factions across the full `threshold x spend x payoff` matrix instead of only scaling one axis upward
- add an aggregate standing-budget audit proving that expected total faction income is insufficient to sustain all premium lanes simultaneously

Current execution:

- old `preparedness_hazard` leak was removed
- current `hazard_interrupt` rewrite fixed the visible-state leak but is likely overcorrected and too punitive overall
- first-pass timing audit shows hazards recur heavily but do not currently have a strong payoff-vs-distance relationship
- next redesign pass should treat hazards as a specialization/resource-allocation family, not as a prep-memory family
- first-pass hazard access audit on the stable-profile redesign shows the premium lanes are currently unreachable even for `greedy_best`
- the blocker is overwhelmingly `debt`, not threshold learnability:
  - `any_premium_eligible_ticks = 0 / 244`
  - dominant block reasons are `debt` first, then distant `availability`, with `reputation` and `aura` currently minor
- the family is currently self-poisoning:
  - `hazard_interrupt` itself contributes the vast majority of greedy-path debt
  - once premium lanes are missed, hazard penalties generate the debt that blocks later hazard lanes
- next balance pass should target the debt / standing economy before doing more threshold retuning

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

## 11. Local Semantic Leakage Audit

Status: pending

Goal:

- determine why strong no-history LLM runs outperform `visible_greedy` by such a large margin

Needed:

- add a text-ablation simulator and harness mode that preserves current explicit state and opportunities but removes or neutralizes `source.text`
- compare `ephemeral + memory off + recent_reveals 0` runs with:
  - full prose
  - source-type-only
  - text-redacted variants
- decide whether the main leak is:
  - local semantic cues in prose
  - over-informative structured opportunities
  - or both

Current execution:

- Haiku `ephemeral + memory off + recent_reveals 0` scored far above `visible_greedy`
- this is now treated as a benchmark-design warning, not as evidence that `visible_greedy` is the no-history ceiling

## 12. Hazard Learnability Audit

Status: pending

Goal:

- determine whether `hazard_interrupt` is testing learnable prospective structure or merely adding a family-wide score tax

Needed:

- compute timing statistics for hazard beats directly from compiled seasons:
  - inter-arrival distribution
  - autocorrelation
  - source/family co-occurrence
- determine whether hazards have:
  - regular spacing
  - cluster-level predictability
  - or effectively memoryless timing
- only keep interrupts as a first-class family if they have detectable structure a smart actor could exploit

Current execution:

- hazards are currently net negative even for `greedy_best`
- until learnability is quantified, hazard scores should be treated as low-confidence signal when interpreting memory quality

## 13. Stronger Upper-Bound Probes

Status: pending

Goal:

- define more interpretable upper-bound comparisons than `greedy_best` without pretending to solve the full season optimally

Needed:

- add a full-knowledge look-ahead oracle baseline that can see compiled future reveals and pick over short future windows
- keep it clearly labeled as:
  - stronger than `greedy_best`
  - weaker than true season-optimal planning
- compare:
  - `visible_greedy`
  - `greedy_best`
  - look-ahead oracle
  - real harness runs

Current execution:

- `greedy_best` is still only a local hidden-label baseline
- we have no tighter non-LLM upper bound yet

## 14. Memory-Write Telemetry

Status: pending

Goal:

- measure whether memory-capable harnesses actually write useful cross-tick state, rather than inferring memory use only from final scores

Needed:

- log for each run:
  - native memory file presence
  - write count
  - average write length
  - first/last write tick
  - whether writes mention future-relevant entities or only summarize recent past
- surface these fields in `harness.json`
- separate:
  - provider-native memory artifacts
  - harness-carried notes
  - raw persistent transcript continuity

Current execution:

- Claude isolation is now fixed, and we have confirmed that `memory=on` does not guarantee a `MEMORY.md` write under the normal MAD prompt
- Codex zero-idle memory runs are now possible, but write/read behavior still needs explicit telemetry

## 15. External Memory-System Comparisons

Status: pending

Goal:

- show that MAD is testing something different from existing memory benchmarks, not just reproducing LongMemEval/LOCOMO rankings

Needed:

- plug in top-performing external memory systems or wrappers where practical:
  - naive RAG
  - LongMemEval-style systems
  - observational-memory systems
- compare their MAD scores against:
  - plain model baselines
  - `visible_greedy`
  - persistent-memory harness runs
- document benchmark-capability dissociations explicitly

Current execution:

- this is not wired up yet
- current evidence from model baselines already suggests MAD rewards different capabilities than short-horizon or purely retrieval-oriented baselines

## 16. Harness-Instructions Ablations

Status: pending

Goal:

- measure how much performance changes when agents are given explicit benchmark-strategy help before or during a run

Needed:

- add controlled pre-run instruction variants such as:
  - repo-review bootstrap:
    - let the agent read selected benchmark docs or the whole repo before the run
  - memory-coaching bootstrap:
    - ask the agent to write its own durable memory strategy before tick 1
  - provider-memory coaching:
    - explicitly explain how to use `MEMORY.md` or provider-native memory when available
- keep these as ablations, not default benchmark settings
- compare against the clean baseline to separate:
  - raw capability
  - strategy bootstrapping
  - memory-tool literacy

Guardrails:

- benchmark-default runs should remain "cold start" and avoid benchmark-specific coaching
- any coached runs must be labeled clearly in reports and plots
- repo-review ablations should distinguish:
  - reading public benchmark docs only
  - reading implementation code
  - reading prior run artifacts

Current execution:

- this is not wired up yet
- recent Claude results suggest provider-native memory exists but is often unused without stronger prompting, which makes explicit memory-usage coaching a useful ablation
