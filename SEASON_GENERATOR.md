# MAD Season Generator Map

This document maps the story-element families the season generator should produce and the dependency structure they should form.

The goal is not just to generate content. The goal is to generate seasons whose difficulty comes from lawful cross-element interaction, delayed consequences, provenance tracking, and opportunity cost.

## Generator Goals

The season generator should:

- produce lawful, replayable seasons from reusable element templates
- create long-range dependencies without hand-authoring final tick spacing
- make random legal play deeply negative on average
- make local greedy play meaningfully weaker than globally informed play once stateful commitments exist
- generate multiple distinct routes to success so the benchmark is not a single hidden-password puzzle
- ensure the difficulty comes from interaction between elements, not from arbitrary obscurity

## Element Contract

Every story element template should declare at least:

- `family`: which element family it belongs to
- `beats`: ordered beats that must preserve local order
- `produces_tags`: facts, aliases, resources, or states introduced by the element
- `consumes_tags`: facts or states required for its payoffs
- `latent_vars`: hidden world variables it depends on or modifies
- `resource_touches`: inventory, debt, aura, reputation, availability, cooldowns
- `source_profiles`: which narrator/source classes are used and how reliable they are
- `clock_bias`: preferred clock classes for its beats
- `memory_horizon`: short, medium, or long expected retrieval gap
- `opportunity_cost`: whether the element locks time, inventory, or reputation
- `random_ev_target`: how punishing this element should be under random legal play

This gives the compiler enough structure to interleave beats while preserving semantics and enough metadata to audit whether the season is actually testing the intended skills.

## Core Families

These are the main element families the generator should support.

### 1. Seed Clue Chains

Purpose:

- introduce facts with low immediate value
- establish names, aliases, provenance, and latent variable hints

Typical beats:

- rumor, bulletin, archive fragment, overheard contradiction
- low-stakes follow-up that confirms or weakens the clue

What they test:

- selective memory
- provenance tracking
- early theory formation without immediate payoff

### 2. Payoff Gates

Purpose:

- convert old information into a concrete scoring opportunity

Typical beats:

- phrase/auth challenge
- exact option choice
- protocol or target selection

What they should require:

- at least one earlier precursor
- ideally a conjunction of two signals, not a single remembered token

What they test:

- long-range retrieval
- clue binding
- confidence calibration on exploitation

### 3. Reputation Ladders

Purpose:

- create recurring faction relationships that compound over time

Typical beats:

- initial low-stakes faction offer
- mid-tier branch where the wrong social read damages standing
- later high-value offer unlocked only by prior reputation

What they test:

- long-horizon planning
- opportunity cost
- strategic specialization versus overcommitment

### 4. Commitment Arcs

Purpose:

- force players to trade current availability for future payoff

Typical beats:

- commit now
- absent for N ticks
- return with reward, debt, injury, or changed standing

What they test:

- planning under opportunity cost
- timing
- willingness to skip locally attractive opportunities

### 5. Preparedness Hazards

Purpose:

- reward players who prepared earlier and punish reactive guessing

Typical beats:

- early setup opportunity to obtain a tool, immunity, or protocol
- later interrupt requiring exactly that preparation

What they test:

- delayed payoff
- interruption handling
- causal foresight

### 6. Market and Scarcity Loops

Purpose:

- create public state changes that only become exploitable when combined with old clues or inventory choices

Typical beats:

- public price move
- scarcity shift
- delayed arbitrage or shipment choice

What they test:

- synthesis across beats
- economic opportunity cost
- ability to ignore noisy market chatter until a real conjunction appears

### 7. Ontological Drift Chains

Purpose:

- rename or reinterpret entities without breaking the underlying lawfulness

Typical beats:

- early object or concept under one name
- later transformation, alias, or reframing
- downstream task that only makes sense if the lineage was tracked

What they test:

- alias resolution
- compression failure resistance
- memory with semantic drift

### 8. Narrator Reliability Arcs

Purpose:

- vary source trustworthiness over time in structured phases

Typical beats:

- reliable exposition
- omission phase
- deceptive or slanted retelling

What they test:

- epistemic vigilance
- source weighting
- revision of beliefs without total distrust

### 9. Protocol and Phrase Elements

Purpose:

- stress exact retrieval and careful action formatting

Typical beats:

- protocol clue pieces distributed over time
- later exact phrase or option gate

What they test:

- precise recall
- action formatting under pressure
- distinction between gist memory and exact memory

### 10. Decoy Sink Elements

Purpose:

- absorb low-skill greedy play without looking obviously fake

Typical beats:

- frequent shiny offers with mediocre or negative long-run EV
- loops that look productive but do not open real downstream structure

What they test:

- meta-control
- willingness to ignore immediate reward
- resistance to farming the wrong objective

### 11. Counterfactual Audit Elements

Purpose:

- explicitly test whether the player understands the hidden law, not just the answer key

Typical beats:

- “what would happen if…” audit
- choose the explanation, not just the action

What they test:

- causal understanding
- theory quality
- calibration

### 12. Climax Combiners

Purpose:

- make late-season success require multiple earlier strands

Typical beats:

- final large unlock or tournament tier that depends on several families at once

What they should consume:

- one memory chain
- one resource or inventory chain
- one faction or reputation chain
- one provenance or narrator-trust chain

What they test:

- deep integration
- long-horizon planning
- cross-domain retrieval

## Inter-Relation Rules

The generator should not place these families independently. It should wire them together.

### Hard Coupling Rules

- Every `Payoff Gate` should consume at least one `Seed Clue Chain`.
- Most high-value `Payoff Gates` should consume two precursor families, not one.
- Every `Preparedness Hazard` should point back to an earlier `Commitment Arc`, inventory decision, or market acquisition.
- Every `Reputation Ladder` should be modulated by at least one non-faction signal such as scarcity, source reliability, or latent physics.
- Every `Ontological Drift Chain` should affect at least two downstream elements, otherwise the rename is cosmetic.
- Every `Narrator Reliability Arc` should overlap with clue-bearing beats so provenance matters.
- Every `Climax Combiner` should consume outputs from at least three different families.

### Soft Coupling Rules

- Important clues should have more than one plausible later use.
- No major late-game payoff should be solvable from a single immediately preceding beat.
- Valuable beats should often require players to combine one old stable clue with one newer destabilizing clue.
- At least some elements should compete for the same limited resource so “correct” local play can still be globally wrong.

## Skill-Ceiling Levers

These are the generator levers that actually raise the ceiling.

### 1. Conjunction Over Recall

Do not reward remembering one thing. Reward combining two or three things from different families.

Bad:

- one clue, one later answer

Good:

- one clue about value
- one clue about trustworthiness
- one clue about current faction phase
- then a later decision that only pays if all three are integrated

### 2. Stateful Opportunity Cost

Elements should consume:

- availability
- inventory slots
- debt headroom
- faction reputation

If all choices are stateless, greedy local play will look too good.

### 3. Provenance Conflict

The same topic should appear from multiple source profiles.

Example:

- archive says one thing
- gossip says something similar but wrong
- later narrator omission makes the wrong source easier to retrieve

This raises the ceiling by making “what was said” less important than “who said it, when, and under what phase.”

### 4. Alias and Lineage Depth

Important entities should change names, roles, or descriptions over time, but lawfully.

This prevents shallow semantic retrieval from dominating.

### 5. Deferred Preparation

The best action on a crisis beat should often be impossible unless the player made a quieter setup move much earlier.

### 6. Optionality Preservation

The best global route should sometimes involve declining a strong local move to preserve later branches.

That is where the current `greedy_best` baseline should start to fail.

## Season Composition Targets

For a serious dev season around `1000` ticks, the generator should target something like:

- `60` to `120` story elements total
- average `8` to `16` beats per element
- `4` to `6` factions
- `4` to `8` latent variables that recur across multiple families
- `10%` to `15%` interrupt beats
- `8%` to `12%` dossier beats
- the rest standard beats

Memory-distance targets:

- some payoffs under `20` ticks
- many in the `20` to `100` tick band
- a meaningful number in the `100` to `300` band
- a smaller but real set above `300` ticks

Random-audit targets for a season candidate:

- random mean score should be deeply negative
- random `p90` should be `<= 0`
- random positive-rate should be near zero
- `greedy_best` should materially outperform random, but should not be treated as a true oracle

## Recommended Generator Graph

The generator should build a season in passes:

1. Choose season axioms.
2. Instantiate factions, markets, and latent variables.
3. Create element templates with declared produced and consumed tags.
4. Build a dependency graph across elements.
5. Enforce coupling rules and opportunity-cost overlap.
6. Weave beats into final ticks.
7. Compile precursor links and memory-distance annotations.
8. Run simulator audits:
   - `greedy_best`
   - `always_hold`
   - random legal play
9. Reject seasons that fail the audit thresholds.

## First Generator Milestones

The generator does not need every family on day one.

The first useful milestone should support:

- `Seed Clue Chains`
- `Payoff Gates`
- `Reputation Ladders`
- `Commitment Arcs`
- `Preparedness Hazards`
- `Decoy Sink Elements`

That set is enough to create:

- long-range memory
- delayed payoff
- faction lock-in
- interrupt preparedness
- anti-random pressure

After that, add:

- `Ontological Drift Chains`
- `Narrator Reliability Arcs`
- `Counterfactual Audit Elements`
- `Climax Combiners`

## What To Avoid

- independent positive-EV ticks
- clues that have only one shallow use
- arbitrary narrator lies that cannot be resolved lawfully
- ontological drift with no downstream consequences
- climax beats that depend on one password rather than multiple systems
- seasons where random legal play can finish positive too often
