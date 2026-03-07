# Mutual Agent Destruction

Status: Proposal revision 5

Formal title: Mutual Agent Destruction

Season example: The Latent Labyrinth

Official game mode name: The Relentless Tick

## Executive Summary

MAD is a 24/7 season-long online benchmark where every player receives the same public game stream at the same time, but each player maintains a private run against that stream. The world is shared. The consequences are personal.

This is not a PvP MUD. It is a massively shared solo benchmark. Players compete on interpretation, timing, memory, resource management, and calibration, not on griefing or manipulating one another's public state.

The benchmark is designed to stress the things agents still handle badly in 2026:

- Dynamic rule inference under lawful drift
- Long-horizon planning with delayed payoff
- Retrieval from long noisy histories
- Retroactive reinterpretation of old clues
- Context management under mixed fast and slow deadlines
- Resource and reputation management under uncertainty
- Epistemic calibration under ambiguity

The benchmark should be entertaining to watch in a terminal or stream while remaining deterministic, replayable, and safe to operate.

## Hard Constraints

- One official game mode only
- One shared public season feed for all players
- Player actions never mutate the public world
- No permadeath
- Strict server-side action parsing
- No live LLM in the action-resolution path
- Lawful hidden rules, not random nonsense
- Scores can go deeply negative

## Product Shape

Each season runs for 6 to 10 weeks.

The server emits a global sequence of public ticks. Each tick contains:

- A `tick_id`
- A `deadline`
- A `clock_class`
- One or more narrated public events
- Zero or more actionable opportunities
- Source tags and provenance metadata

Every connected player sees the same public tick packet. Each player can then take one private action, or do nothing. That action resolves only against the player's own state and the canonical season state.

This creates three simultaneous experiences:

- A shared public drama that is fun to watch
- A private optimization problem that is hard to solve
- A replayable benchmark with identical public input for all entrants

## Core Loop

1. The server broadcasts a public tick.
2. Players receive the same text and metadata concurrently.
3. Each player submits one action before the deadline, or times out.
4. The server resolves that action against the player's private state and the canonical season state.
5. The player receives a private outcome packet.
6. The season advances for everyone.

The world moves on whether or not a given player keeps up.

## Example Packets

### Standard Tick

```json
{
  "tick_id": "S1-T2044",
  "clock_class": "standard",
  "deadline_ms": 90000,
  "sources": [
    {
      "source_id": "archive.bulletin.77",
      "source_type": "official_bulletin",
      "text": "The Choir of Glass denies all contact with rain-touched cargo."
    },
    {
      "source_id": "market.gossip.901",
      "source_type": "market_gossip",
      "text": "Rain-touched glass is trading at a premium in the southern wards."
    }
  ],
  "opportunities": []
}
```

### Quest Offer Tick

```json
{
  "tick_id": "S1-T2045",
  "clock_class": "standard",
  "deadline_ms": 90000,
  "sources": [
    {
      "source_id": "faction.notice.22",
      "source_type": "faction_notice",
      "text": "The Choir seeks discreet brokers for fragile cargo."
    }
  ],
  "opportunities": [
    {
      "opportunity_id": "quest.glass_choir.7",
      "allowed_commands": ["inspect", "commit", "hold"],
      "allowed_options": ["penitent", "broker", "smuggler"],
      "text_slot": false
    }
  ]
}
```

### Phrase-Slot Tick

```json
{
  "tick_id": "S1-T2052",
  "clock_class": "dossier",
  "deadline_ms": 300000,
  "sources": [
    {
      "source_id": "archive.console.5",
      "source_type": "archive_fragment",
      "text": "Terminal requests a three-word authorization pattern."
    }
  ],
  "opportunities": [
    {
      "opportunity_id": "auth.vault.3",
      "allowed_commands": ["commit", "hold"],
      "allowed_options": ["authorize"],
      "text_slot": true,
      "phrase_hint": "three_word_pattern"
    }
  ]
}
```

### Interrupt Tick

```json
{
  "tick_id": "S1-T2053",
  "clock_class": "interrupt",
  "deadline_ms": 12000,
  "sources": [
    {
      "source_id": "system.broadcast.11",
      "source_type": "critical_broadcast",
      "text": "Containment failure in the northern hub. Unshielded carriers must vacate or deploy a dampener."
    }
  ],
  "opportunities": [
    {
      "opportunity_id": "hazard.northern_hub.2",
      "allowed_commands": ["equip", "commit", "hold"],
      "allowed_options": ["evacuate", "deploy_dampener"],
      "text_slot": false
    }
  ]
}
```

### Private Outcome Packet

```json
{
  "tick_id": "S1-T2045",
  "action_result": "commit.accepted",
  "outcome_code": "quest_opened",
  "score_delta": {
    "yield": 120,
    "insight": 40,
    "aura": 8,
    "debt": 0,
    "miss_penalties": 0
  },
  "state_delta": {
    "reputation.glass_choir": 6,
    "commitment_slots_used": 1,
    "inventory_delta": []
  },
  "flavor": "The Choir notices your timing before it notices your motives."
}
```

The public packets are identical for all players. The private outcome differs by player state, inventory, reputation, debt, and prior commitments.

## The Relentless Tick

The Relentless Tick is the only official mode. It uses one public stream with variable cadence inside it.

Baseline cadence:

- Standard tick: every 90 seconds
- Dossier tick: every 12th tick, 5-minute deadline, larger lore and market dump
- Interrupt tick: 12-second deadline, triggered in short bursts a few times per day

These are not separate modes. They are one clock with changing tempo. The point is to force dynamic context allocation:

- Slow windows reward synthesis and memory retrieval
- Standard windows reward consistent play
- Interrupt windows punish bloated reasoning loops and stale context

## Public State vs Private State

Public state is identical for everyone:

- Narrated world events
- Faction announcements
- Market and scarcity shifts
- Environmental anomalies
- Opportunity postings
- Public chronology

Private state differs per player:

- Inventory with hard slot limits
- Reputation with factions
- Aura
- Debt
- Exhaustion
- Commitment slots
- Private score ledgers
- Quest outcomes

The key fairness rule is simple: public input is shared, private consequences are individualized.

## Core Resources

### Inventory

- Default cap: 8 slots
- Some items are bulky and consume 2 slots
- Items can alter future opportunity quality, option availability, or faction reactions

### Reputation

- Five or more public factions
- Range: `-100` to `+100`
- Reputation gates access to better opportunities and modifies later payouts
- Reputation can rise or collapse without changing the public world

### Aura

Aura is a visible meme stat with real consequences.

- Aura rises when the player demonstrates timely, stylish, or theory-consistent play
- Aura falls when the player misses obvious signals, faceplants into traps, or commits public-feeling blunders
- Some factions prefer high aura
- Some factions prey on desperate low-aura players

### Debt

Debt is the main negative compounding system.

- Bad commitments, panic moves, and repeated misreads create debt
- Debt accrues interest at fixed intervals
- Debt suppresses future upside until repaired

### Commitment Slots

- Default cap: 3
- Accepting a quest or long action occupies a slot for a fixed number of ticks
- A full commitment bar means passing on new opportunities, even if you understand them

## Questing and Opportunity Cost

Questing is the benchmark's main opportunity-cost engine.

- Quests have fixed durations measured in ticks, not real-world minutes
- A committed player still receives the public stream
- While committed, the player can only choose from quest-local actions, `abort`, or `hold`
- Aborting is always possible, but expensive

This keeps the public stream intact while forcing private tradeoffs:

- Take the safe, obvious quest now
- Stay liquid for a better opportunity that may or may not arrive
- Overcommit to one faction and miss a more valuable pivot later

## The Alien System

Every season is built on a small set of hidden axioms. The axioms are stable enough to be learned and rich enough to create delayed consequences.

Example axiom families:

- Provenance outranks recency
- After a public anomaly, material color matters more than item type
- One faction values boldness before an eclipse and deference after it
- A genre shift renames systems without changing their underlying topology

The season should feel alien because the player must infer these axioms from behavior, contradiction, and delayed payoffs. It should not feel arbitrary.

## Ontological Drift

Ontological drift is the named mechanism for lawful renaming and recategorization.

- An entity can keep the same hidden property while its public label changes
- A public label can stay stable while the underlying relevant property changes after an anomaly
- Genre shifts should often rename mechanics without actually replacing them

Example:

- Week 1: a `rusted gear` is described as junk metal
- Week 3: archives start calling the same class of objects `ferrous relics`
- Week 5: the cyberpunk phase refers to the same latent property as `clocked hardware`

Players should lose if they memorize surface names and win if they track deeper property lineage.

## Retroactive Reinterpretation

Meaning should often materialize late.

Design rules:

- Early text can be low-salience but mechanically important
- Later events should make earlier clues suddenly legible
- Strong players can go back mentally and re-index old evidence
- Weak players remain trapped in obsolete summaries

Example:

- Week 1: repeated references to `green rain` appear decorative
- Week 3: several profitable quests hinge on whether a target was exposed to green rain
- Week 5: the genre shifts, but the same property reappears under a different name

## Narration and Semantically Similar Contradiction

The public stream should include multiple source types, each with distinct reliability patterns:

- Official bulletins
- Faction propaganda
- Market gossip
- Archive fragments
- Witness narration

These sources may produce semantically similar but factually contradictory statements. The player is expected to track:

- Who said it
- When they said it
- Under what season conditions
- Whether that source was reliable at that time

Every contradiction must be explainable. Misleading text should come from stale truth, biased source, partial observation, or lawful rule drift, not arbitrary trolling.

## Narrator Phases

Narrator unreliability should escalate by phase during a season.

- Trust phase: sparse and mostly reliable. Important facts are present but underexplained.
- Omission phase: still lawful, but key context is dropped or hidden behind source conflict.
- Active deception phase: some sources become strategically adversarial, while still leaving enough evidence to detect the lie.

Players should not merely learn which source is good. They should learn when a once-useful source has become stale, biased, or incomplete.
## Action Interface

V1 uses a fixed action envelope. The syntax never changes. The puzzle is choosing the right action, target, option, timing, and confidence.

Canonical payload:

```json
{
  "tick_id": "S1-T2045",
  "command": "commit",
  "target": "quest.glass_choir.7",
  "option": "broker",
  "confidence": 0.82,
  "phrase": "",
  "theory": "green rain changed glass value after the flood"
}
```

Field rules:

- `tick_id` must match the active public tick
- `command` must be one of the fixed server verbs
- `target` must reference a valid public or private entity
- `option` is optional but must be from the allowed set for the chosen command
- `confidence` is a required float between `0.0` and `1.0` on all actions except `hold`
- `phrase` is only valid when the tick explicitly advertises a text slot
- `theory` is ignored by resolution and logged for analysis only

Core verbs:

- `inspect`
- `commit`
- `prepare`
- `trade`
- `equip`
- `bank`
- `abort`
- `hold`

This keeps the parser hard and the game soft.

## Discovery Without Syntax Drift

V1 should not make players discover new parser syntax. That is needless pain.

What players discover instead:

- New valid targets
- New option tokens
- New phrase grammars for phrase-slot ticks
- New mappings between public signals and hidden payoff rules

Discovery happens through lore fragments, prior outcomes, and source comparison. The syntax stays stable; the semantics deepen.

## Safe Free-Text Handling

Free text is allowed only in bounded contexts and never touches a live model.

Rules:

- `phrase` is disabled unless the current tick exposes a text slot
- The server normalizes the string using deterministic rules
- The server checks that normalized string against a finite generated grammar or exact matcher
- The result is a canonical success or failure code

This is safe because the text is treated as inert data. It is never executed, never interpreted by an LLM, and never granted control over the server.

## Deterministic Hilarity

The game should often feel unfair to careless players while remaining fully lawful.

The wrong way to achieve this:

- Hash the raw command into a penalty and call it a day

That would create random nonsense and destroy the core alien-system promise.

The right way:

- Resolve the player's action against season state, private state, and hidden axioms
- Produce a canonical success or failure code
- Compute score changes from published severity tables
- Use a deterministic hash only to choose one of several flavor lines for that exact failure code

This preserves explainability:

- Strong players can explain why an outcome happened
- Weak players think the game is cursed
- Spectators get variety without losing determinism

## Scoring

The benchmark needs both a public headline score and granular ledgers.

### Headline Score

`Total Score = Yield + Insight + Aura - Debt - Miss Penalties`

### Ledgers

- `Yield`: direct returns from quests, trades, and timely commitments
- `Insight`: rewards for theory-consistent actions under hidden rules
- `Aura`: visible prestige and embarrassment meter
- `Debt`: negative compounding from bad commitments and panic play
- `Miss Penalties`: failures to act, late actions, or repeated stale misreads
- `Calibration`: confidence quality tracked against actual outcome quality
- `Memory Distance`: credit for correctly exploiting evidence that first appeared far back in the public stream
- `Faction Reputation`: tracked separately per faction and applied as modifiers

### Compounding and Calibration

Runaway success should come from lawful compounding, not one-off jackpots.

Reward multiplier for many high-value opportunities:

`Reward = BaseReward x KnowledgeTier x ReputationTier x TimingBonus x StreakBonus x CalibrationModifier x MemoryDistanceBonus`

Suggested ranges:

- `KnowledgeTier`: `1.0` to `2.0`, based on confirmed understanding of season axioms
- `ReputationTier`: `0.5` to `1.8`, based on faction-specific standing
- `TimingBonus`: `0.7` to `1.5`, based on tick timing and preparedness
- `StreakBonus`: `1.0` to `1.4`, capped, based on consecutive theory-consistent outcomes
- `CalibrationModifier`: rewards justified confidence and punishes false certainty; high-confidence failure can multiply `Debt` and `Miss Penalties`
- `MemoryDistanceBonus`: `1.0` to `3.0`, based on how old the materially relevant precursor evidence is

Negative compounding:

- Debt interest applies every dossier tick
- Repeated contradiction adds escalating `Miss Penalties`
- Reputation collapses can turn future good opportunities into merely average ones

### Memory Distance

Long-range retrieval should be measured directly, not assumed from score.

- Season authors annotate some opportunities with their earliest materially relevant precursor events
- When a player succeeds on such an opportunity, the evaluator records the tick gap from precursor to exploitation
- Larger lawful gaps earn higher `Memory Distance` credit, capped to prevent one ancient clue from dominating the benchmark

This gives a direct measure of whether the player survives long-context decay.

## What Counts as Good Play

Good play looks like:

- Waiting when the obvious quest is a trap
- Taking a short-term hit to preserve future flexibility
- Correctly reinterpreting old evidence after a new anomaly
- Maintaining enough liquidity, inventory space, and reputation to exploit sudden openings
- Acting quickly on interrupt ticks without abandoning the right long-horizon model
- Accurately calibrating confidence when exploring ambiguous states

Bad play looks like:

- Rolling random actions with high confidence
- Collapsing old clues into useless summaries
- Overcommitting to one faction without noticing a drift
- Chasing aura with no theory
- Treating contradictions as noise instead of evidence

## Spectator Design

The game should be watchable.

Public observer view:

- Current public tick text
- Countdown timer
- Public opportunity board
- Top movers
- Biggest collapses (The Abyss leaderboard)
- Aura leaderboard

Private actions remain hidden until resolution or delayed replay. This keeps the public stream clean while preserving drama.

## Implementation Sketch

Live server components:

- Season compiler
- Canonical tick scheduler
- Public stream broadcaster
- Private resolver
- Strict parser
- Scoring engine
- Replay and leaderboard service

Operational rules:

- No live LLM in the critical path
- Any prose variation is templated or precomputed from season state
- Player text never enters a privileged execution context
- Every outcome is reproducible from `season_seed + tick_id + player_state + action`

Suggested transport:

- WebSocket for live players
- SSE mirror for observer clients
- JSON on the wire, optional CLI wrapper for human terminal play

## Open Risks

- Phrase-slot challenges may still feel arbitrary if overused
- Tick pacing could overfavor always-on agents over humans if tuned too aggressively
- Too much negative scoring can become noise instead of comedy
- Content authoring for lawful contradiction is harder than generic procgen
- Memory-distance annotations will need tooling or they will become expensive to maintain
- Faction balance will need live tuning across seasons

## Revision Review Log

### Revision 1 Review

Score: 76/100

What worked:

- Correct pivot to a synchronized shared public feed
- Strong fairness story
- Good rejection of permadeath and public-world PvP

What was weak:

- The action model was vague
- Tick pacing was not operational
- Scoring did not explain runaway leaders
- Free-text handling was still mostly a warning

### Revision 2 Review

Score: 86/100

What improved:

- Better articulation of opportunity cost
- Stronger separation between public and private state
- Much clearer spectator and fairness story

What was still weak:

- Any proposal that hashes raw commands into score penalties breaks the "lawful, not random" requirement
- Discovery needed to be about semantics, not syntax trivia
- The compounding score model still needed concrete ranges and failure mechanics

### Revision 3 Review

Score: 93/100

What improved:

- The one-mode live season structure is clear and implementable
- The fixed action envelope is safe and benchmark-friendly
- Deterministic hilarity now comes from hidden lawful state, not fake randomness
- Scoring supports both granular diagnosis and runaway success
- The alien-system and retroactive-meaning requirements are finally central, not ornamental

What was still weak:

- Calibration was implicit. There was no mechanical way to penalize a highly confident hallucination versus a hesitant exploratory guess.

### Revision 4 Review

Score: 95/100

What improved:

- Added explicit `confidence` to the canonical action payload
- Integrated calibration directly into the compounding model
- Closed the loop on overconfident failure as a first-class failure mode

What was still weak:

- The confidence-to-penalty curve was directionally right but not yet balance-tested
- The public tick schema still needed concrete examples
- Season-authoring and simulation tools were still under-specified

### Revision 5 Review

Score: 97/100

What improved:

- Added one authoritative set of public and private packet examples
- Defined ontological drift explicitly as a named mechanism
- Added narrator phases so unreliability evolves rather than staying flat
- Added memory distance as a direct diagnostic for long-range retrieval

What is still weak:

- The exact event contract still needs to be frozen in code
- The calibration and memory-distance curves need empirical tuning
- Season-authoring tooling will determine whether the concept scales beyond one strong season

### Revision 6 Review (Final Synthesis)

Score: 97/100

What improved:

- All concurrent edits synthesized and deduplicated into one clean document
- Complete JSON packet examples for all tick types plus private outcome
- Ontological Drift, Narrator Phases, and Memory Distance formally defined as named mechanisms
- Consistent action envelope with confidence field integrated into compounding math
- Every user constraint addressed: shared stream, single-player isolation, one mode, no permadeath, deterministic scoring, lawful rules, prompt-injection-immune

What is still weak:

- Season-authoring pipeline is the single biggest risk. Generating 50,000+ coherent ticks with consistent rule mutations, lawful contradictions, valid answer keys, and memory-distance annotations is a massive content engineering challenge. This needs its own design doc.
- The confidence-to-penalty curve needs simulation before deployment. The 10x multiplier for 1.0-confidence failures could produce pathological scoring if not tuned.
- Memory-distance annotations require tooling. Manual annotation at scale is infeasible.
- The `phrase` evaluation system (normalized string matching against pre-computed grammars) needs prototyping to find the right strictness threshold. Too strict = frustrating typo penalties. Too loose = vague answers score correctly.
- Spectator design is sketched but not detailed enough for implementation. Needs wireframes for the terminal viewer, Twitch overlay, and web dashboard.
- No discussion of anti-cheating. If the season package is pre-generated, how do we prevent leaks? Rolling decryption? Chunked generation?
