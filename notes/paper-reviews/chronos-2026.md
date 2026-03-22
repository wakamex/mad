---
paper: "Chronos: Temporal-Aware Conversational Agents with Structured Event Retrieval for Long-Term Memory"
authors: Sahil Sen, Elias Lumer, Anmol Gulati, Vamse Kumar Subbiah (PwC)
url: https://arxiv.org/abs/2603.16862
date: 2026-03-17
reviewed: 2026-03-22
relevance: high (foil for MAD positioning)
---

# Summary

Decompose dialogue into SVO event tuples with resolved datetime ranges + entity aliases, index them in a structured event calendar alongside a turn calendar (raw conversational context). At query time, use "dynamic prompting" -- query-conditioned retrieval guidance that tells the agent what to retrieve, how to filter temporally, and how to chain multi-hop reasoning via iterative tool-calling over both calendars.

Results: Chronos Low 92.60%, Chronos High 95.60% on LongMemEvalS -- new SOTA with 7.67% improvement over best prior system. Event calendar alone accounts for 58.9% gain.

# Strengths

1. **Dual-calendar architecture is well-motivated.** Both structured temporal events (for date filtering, sequence reasoning, cross-session aggregation) and raw turns (for semantic context the structured extraction might miss). Most prior systems pick one representation and lose information.

2. **Dynamic prompting is the interesting contribution.** Rather than rewriting the search query (standard RAG), they generate per-query retrieval instructions -- a meta-prompt for how to approach retrieval for that specific question type. Neat middle ground between autonomous agent reasoning and rigid pipelines.

3. **Strong ablation.** 58.9% gain from event calendar alone, 15-22% from each other component. Better attribution than most papers in this space.

4. **Breadth of LLM evaluation.** 8 models (open and closed) adds credibility.

# Weaknesses

1. **LongMemEvalS is a narrow benchmark.** Tests passive retrieval from existing dialogue history. Questions are well-structured, evidence is planted in conversations, task is fundamentally "find and synthesize information that was explicitly said." Doesn't require active write decisions under uncertainty, doesn't test prospective memory. Can be substantially solved by good embedding + temporal metadata (which is literally what Chronos does). 95.6% here doesn't tell us much about settings where the agent needs to decide what's worth remembering.

2. **Extraction pipeline is LLM-heavy and fragile.** Every turn gets LLM-processed for SVO event tuples. Expensive, compounds extraction errors. No deep analysis of extraction failure modes (hallucinated dates, misresolved entity aliases).

3. **No write-time decisions.** Indexes everything -- all turns, all extractable events. No selectivity, forgetting, or prioritization. Sidesteps scaling problems by evaluating on fixed dataset.

4. **"Dynamic prompting" oversells.** Reading between the lines: query classification + templated retrieval strategy selection. Conditional dispatch on query features, not truly dynamic. Still useful but framing is generous.

5. **Single benchmark.** Only LongMemEvalS. No LoCoMo, no real-world deployment, no adversarial/stress tests. Single-benchmark paper at 95.6% feels like leaderboard optimization.

6. **PwC affiliation, no code release.** Reproducibility concerns.

# Relevance to MAD

- **Good foil for positioning.** Represents systems that achieve high scores on retrieval-from-history benchmarks through clever retrieval engineering but don't address prospective memory and active write decision problems. Useful citation for "these approaches work on retrieval benchmarks but fail when the agent must decide what to remember proactively."

- **Event extraction (SVO tuples + datetime ranges)** is a structured variant of "extract everything" -- temporal metadata does the heavy lifting that embedding similarity can't. Compare against MAD's hazard_history structured memory.

- **95.6% ceiling on LongMemEvalS** suggests benchmark saturation. Strengthens case for why MAD is needed as a harder, more discriminating eval.

- **Key contrast with MAD hazard:** Chronos indexes everything and retrieves. MAD's hazard family shows that indexing everything (raw persistent context) actually *hurts* performance -- the agent needs to curate, not accumulate. Chronos doesn't test this failure mode.
