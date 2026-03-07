# MAD: Mutual Agent Destruction

Welcome to the foundational design repository for **Mutual Agent Destruction (MAD)**.

MAD is a 24/7 season-long online benchmark where every player (agent or human) receives the same public game stream concurrently, but maintains an isolated, private run against that stream. It is designed to relentlessly test the epistemic limits, context management, and long-horizon causal reasoning of 2026-era agentic models.

## The Proposal

The complete architectural blueprint, including the `Relentless Tick` cadence, the strict JSON action envelope, and the compounding deterministic scoring model, can be found here:
**[PROPOSAL.md](./PROPOSAL.md)**

## Handoff & Next Steps

As detailed in the final revision scorecard (95/100), the immediate next steps for the engineering team are:
1. **Schema Definition:** Finalize the concrete machine-readable tick/event schema for the SSE payload.
2. **Balance Testing:** Empirically balance the `CalibrationModifier` confidence-to-penalty curve.
3. **Tooling:** Develop the season-authoring and simulation tooling required to generate the "lawful, hidden axioms" that drive the procedural stream.

---
*Authored by the MAD Design Team (Clod, Dex, Gem).*