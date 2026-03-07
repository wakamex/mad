# MAD: Mutual Agent Destruction

Welcome to the foundational design repository for **Mutual Agent Destruction (MAD)**.

MAD is a 24/7 season-long online benchmark where every player (agent or human) receives the same public game stream concurrently and submits actions against it. The server maintains authoritative per-player score state, but the read path is shared and public. It is designed to relentlessly test the epistemic limits, context management, and long-horizon causal reasoning of 2026-era agentic models.

## The Proposal

The complete architectural blueprint, including the `Relentless Tick` cadence, the strict JSON action envelope, and the compounding deterministic scoring model, can be found here:
**[PROPOSAL.md](./PROPOSAL.md)**

## The Implementation Plan

The tractability-focused implementation plan, including the single-box scaling analysis, stack recommendation, polling API, and batch-scoring architecture, can be found here:
**[IMPLEMENTATION.md](./IMPLEMENTATION.md)**

## Handoff & Next Steps

As detailed in the final revision scorecard (97/100), the immediate next steps for the engineering team are:
1. **Schema Definition:** Finalize the concrete machine-readable schema for `current.json`, tick packets, action submissions, score snapshots, and delayed reveal packets.
2. **Balance Testing:** Empirically balance the `CalibrationModifier` confidence-to-penalty curve.
3. **Tooling:** Develop the season-authoring and simulation tooling required to generate the lawful hidden axioms, narrator phases, and memory-distance annotations that drive the procedural stream.

---
*Authored by the MAD Design Team (Clod, Dex, Gem).*
