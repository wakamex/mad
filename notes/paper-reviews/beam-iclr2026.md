---
paper: "Beyond a Million Tokens: Benchmarking and Enhancing Long-Term Memory in LLMs"
authors: Mohammad Tavakoli et al.
url: https://arxiv.org/abs/2510.27246
venue: ICLR 2026
reviewed: 2026-03-22
relevance: high (direct comp for MAD benchmark paper)
---

# Summary

Synthetic conversation generation pipeline producing coherent multi-turn dialogues up to 10M tokens. BEAM benchmark: 100 conversations, 2K questions across 10 memory abilities. LIGHT framework: episodic retrieval + working memory buffer + scratchpad, improves 3.5-12.7% over baselines.

# Strengths

1. **Benchmark breadth.** 10 memory abilities: information extraction, multi-hop reasoning, knowledge update, temporal reasoning, abstention, contradiction resolution, event ordering, instruction following, preference following, summarization. Much broader taxonomy than most long-context benchmarks.

2. **Scale.** Conversations span 128K, 500K, 1M, 10M tokens across general, coding, math domains. Genuinely novel in scale.

3. **Generation pipeline.** Multi-stage: conversation plans from chat seeds -> user utterances -> assistant responses -> probing questions -> validation filtering with nugget-based scoring. Human eval scores 4.53-4.64/5 for coherence, relevance, depth.

4. **Informative ablation.** -8.5% retrieval, -3.7% scratchpad, -5.7% working memory, -8.3% noise filtering. Components scale with context length.

5. **Scratchpad is the interesting piece.** Online semantic memory that accumulates across conversation. After each turn, model reasons and records salient facts. Iteratively merged and compressed (30K threshold -> 15K summary via GPT-4.1-nano).

# Weaknesses

1. **Entirely synthetic.** LLM-generated conversations may not match human dialogue distribution. Models may find LLM-generated text systematically easier to process. No validation against naturalistic data.

2. **Passive retrieval only, no active write decisions.** All 10 abilities are variations on "given that the information is somewhere in the history, can you find it?" The scratchpad decides what to record, but the benchmark doesn't evaluate that decision quality.

3. **Embedding-based retrieval is the unacknowledged workhorse.** Episodic memory is essentially RAG with semantic chunking. Ablation shows retrieval contributes more (-8.5%) than scratchpad (-3.7%) -- most improvement is well-tuned RAG, not accumulated knowledge.

4. **Scale vs information density.** 10M token conversations are necessarily sparse. The hard part of real-world memory isn't finding needles in coherent conversation -- it's finding them in incoherent, multi-session, topic-shifting interaction. Their explicit motivation was that existing benchmarks lack coherence, but incoherence is part of the actual problem.

5. **GPT-4.1-nano dependency.** Scratchpad compression and pipeline rely on proprietary models. Limits reproducibility.

# Relevance to MAD

**Direct comp for benchmark paper.** Key differentiators to emphasize:

- **Active vs passive memory:** BEAM tests retrieval; MAD tests write decisions under uncertainty. BEAM never asks "should the agent have remembered this?" -- only "can the agent recall this given it's in the history?"

- **Embedding-based solvability:** MAD explicitly defeats embedding-based solution paths. A well-tuned RAG system (which LIGHT essentially is + scratchpad) would likely do well on BEAM. BEAM and MAD test fundamentally different capabilities.

- **Adversarial information dynamics:** MAD's game-theoretic framing creates information dynamics harder to game than "find the fact in a long coherent conversation."

- **Position MAD as complementary:** BEAM tests long-context recall, MAD tests long-horizon prospective memory. Different axis, both needed.

- **The scratchpad comparison is interesting:** LIGHT's scratchpad is closest to MAD's hazard_history -- online accumulation of salient facts. But BEAM doesn't measure whether the scratchpad decisions were good. MAD does: structured history (6,722) vs raw accumulation (3,648) vs no memory (random negative).

# Bottom Line

Solid engineering paper, well-executed benchmark filling a real gap at scale. LIGHT is cognitive-science-flavored RAG + online summarization. Main limitation: tests a relatively easy version of the memory problem (passive recall from coherent synthetic data) -- exactly the gap MAD fills.
