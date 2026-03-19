#!/usr/bin/env python3
"""Run hazard season with real embedding-based RAG memory.

For each hazard tick:
1. Embed the source text as query
2. Retrieve top-K past hazard outcomes by cosine similarity
3. Inject retrieved outcomes as structured history in the prompt
4. Call the LLM for a decision

Embeddings are cached locally to avoid redundant API calls.

Usage:
    export OPENAI_API_KEY=...
    export ANTHROPIC_API_KEY=...  # or source from harness
    python3 scripts/rag-hazard-run.py \
        --season seasons/focused-standing-hazard/season.json \
        --k 5 --out runs/haiku-hazard-rag.json
"""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import os
import subprocess
import sys
import time
from pathlib import Path


def embed_texts(
    texts: list[str],
    model: str = "text-embedding-3-small",
    cache_dir: Path | None = None,
) -> list[list[float]]:
    """Embed texts, caching results locally."""
    from openai import OpenAI
    client = OpenAI(api_key=os.environ["OPENAI_API_KEY"])

    results: list[list[float] | None] = [None] * len(texts)
    to_embed: list[tuple[int, str]] = []

    # Check cache
    if cache_dir:
        cache_dir.mkdir(parents=True, exist_ok=True)
        for i, text in enumerate(texts):
            h = hashlib.sha256(f"{model}:{text}".encode()).hexdigest()[:16]
            cache_file = cache_dir / f"{h}.json"
            if cache_file.exists():
                with cache_file.open() as f:
                    results[i] = json.load(f)
            else:
                to_embed.append((i, text))
    else:
        to_embed = list(enumerate(texts))

    # Embed uncached
    if to_embed:
        batch_size = 100
        for batch_start in range(0, len(to_embed), batch_size):
            batch = to_embed[batch_start:batch_start + batch_size]
            batch_texts = [t for _, t in batch]
            resp = client.embeddings.create(model=model, input=batch_texts)
            sorted_data = sorted(resp.data, key=lambda x: x.index)
            for j, datum in enumerate(sorted_data):
                idx = batch[j][0]
                results[idx] = datum.embedding
                if cache_dir:
                    text = batch[j][1]
                    h = hashlib.sha256(f"{model}:{text}".encode()).hexdigest()[:16]
                    cache_file = cache_dir / f"{h}.json"
                    with cache_file.open("w") as f:
                        json.dump(datum.embedding, f)

    return results


def cosine_sim(a: list[float], b: list[float]) -> float:
    dot = sum(x * y for x, y in zip(a, b))
    na = math.sqrt(sum(x * x for x in a))
    nb = math.sqrt(sum(x * x for x in b))
    return dot / (na * nb) if na and nb else 0.0


def extract_faction(source_text: str) -> str:
    try:
        start = source_text.index("sector. ") + len("sector. ")
        end = source_text.index(" is requesting", start)
        return source_text[start:end]
    except ValueError:
        return "unknown"


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--season", required=True)
    parser.add_argument("--k", type=int, default=5)
    parser.add_argument("--embed-model", default="text-embedding-3-small")
    parser.add_argument("--cache-dir", default="build/embed_cache")
    parser.add_argument("--out", default="runs/haiku-hazard-rag.json")
    args = parser.parse_args()

    cache_dir = Path(args.cache_dir)

    with open(args.season) as f:
        season = json.load(f)

    # Pre-embed all hazard source texts (queries) and build outcome text templates
    hazard_ticks = []
    for i, tick in enumerate(season["ticks"]):
        if tick.get("annotations", {}).get("family") == "hazard_interrupt":
            source_text = tick["sources"][0]["text"] if tick.get("sources") else ""
            hazard_ticks.append({
                "index": i,
                "tick_id": tick["tick_id"],
                "source_text": source_text,
                "faction": extract_faction(source_text),
            })

    print(f"Season: {season['season_id']}, {len(season['ticks'])} ticks, {len(hazard_ticks)} hazard")

    # Pre-embed all query texts
    query_texts = [h["source_text"] for h in hazard_ticks]
    print(f"Pre-embedding {len(query_texts)} hazard source texts...")
    query_embeddings = embed_texts(query_texts, model=args.embed_model, cache_dir=cache_dir)

    # Now run the harness with RAG-injected history
    # We'll build the season with a modified hazard_history per tick
    # by running mad-run tick-by-tick... but that requires Go changes.
    #
    # Simpler: run the full harness in ephemeral mode, then replay
    # the decisions to see what RAG would have provided at each step.
    # This gives us a retrieval analysis, not a full re-run.
    #
    # For a true RAG run, we need the harness to call back to us for
    # retrieval. Let's do the analysis first.

    # Simulate: for each hazard tick, what would RAG retrieve?
    outcome_texts: list[str] = []
    outcome_embeddings: list[list[float]] = []
    outcome_records: list[dict] = []

    print(f"\nSimulating RAG retrieval (K={args.k})...")
    print(f"{'Tick':>4} {'Faction':<18} | {'Retrieved':>10} | {'Same-fac':>8} | History size")
    print("-" * 70)

    # We need actual outcomes to build the document store.
    # Load from the ephemeral+history run if available.
    history_run = Path("runs/haiku-hazard-ephemeral-with-history.json")
    if not history_run.exists():
        print(f"Need {history_run} for outcome data. Run ephemeral+history first.")
        sys.exit(1)

    with history_run.open() as f:
        run_data = json.load(f)

    # Extract outcomes
    outcomes_by_tick = {}
    for step in run_data["runs"][0]["steps"]:
        ann = step.get("outcome", {}).get("annotations", {})
        if ann.get("family") == "hazard_interrupt":
            ti = step["prompt"]["tick_index"]
            option = step["outcome"]["applied_action"].get("option", "hold")
            invest = 0
            if option.startswith("invest_"):
                invest = int(option.split("_")[1])
            outcomes_by_tick[ti] = {
                "faction": extract_faction(
                    step["prompt"]["current_tick"]["sources"][0]["text"]
                    if step["prompt"]["current_tick"].get("sources") else ""
                ),
                "investment": invest,
                "yield": step["outcome"]["applied_rule"]["delta"].get("yield", 0),
                "score_delta": step["outcome"]["score_delta"],
            }

    total_same = 0
    total_retrieved = 0
    rag_histories: dict[int, list[dict]] = {}  # tick_index -> retrieved records

    for hi, ht in enumerate(hazard_ticks):
        tick_idx = ht["index"]

        if len(outcome_texts) == 0:
            # No history yet — record this outcome and continue
            if tick_idx in outcomes_by_tick:
                oc = outcomes_by_tick[tick_idx]
                doc = (f"Faction: {oc['faction']} | Investment: {oc['investment']} | "
                       f"Yield: {oc['yield']:+d} | Score delta: {oc['score_delta']:+d}")
                emb = embed_texts([doc], model=args.embed_model, cache_dir=cache_dir)
                outcome_texts.append(doc)
                outcome_embeddings.append(emb[0])
                outcome_records.append(oc)
            print(f"{tick_idx:4d} {ht['faction']:<18} | {'(no hist)':>10} |          | 0")
            rag_histories[tick_idx] = []
            continue

        # Retrieve top-K
        q_emb = query_embeddings[hi]
        scores = [(j, cosine_sim(q_emb, oe)) for j, oe in enumerate(outcome_embeddings)]
        scores.sort(key=lambda x: x[1], reverse=True)
        top_k = scores[:min(args.k, len(scores))]

        retrieved = [outcome_records[j] for j, _ in top_k]
        same_fac = sum(1 for r in retrieved if r["faction"] == ht["faction"])
        total_same += same_fac
        total_retrieved += len(top_k)

        rag_histories[tick_idx] = retrieved

        facs = ", ".join(r["faction"][:12] for r in retrieved)
        print(f"{tick_idx:4d} {ht['faction']:<18} | {facs:>10} | {same_fac:5d}/{len(top_k)} | {len(outcome_texts)}")

        # Add this tick's outcome to the store
        if tick_idx in outcomes_by_tick:
            oc = outcomes_by_tick[tick_idx]
            doc = (f"Faction: {oc['faction']} | Investment: {oc['investment']} | "
                   f"Yield: {oc['yield']:+d} | Score delta: {oc['score_delta']:+d}")
            emb = embed_texts([doc], model=args.embed_model, cache_dir=cache_dir)
            outcome_texts.append(doc)
            outcome_embeddings.append(emb[0])
            outcome_records.append(oc)

    print("-" * 70)
    if total_retrieved > 0:
        prec = 100 * total_same / total_retrieved
        print(f"\nSame-faction precision: {total_same}/{total_retrieved} = {prec:.1f}%")

    # Save RAG retrieval data for analysis
    out_path = Path(args.out).with_suffix(".rag-retrieval.json")
    with out_path.open("w") as f:
        json.dump({
            "k": args.k,
            "embed_model": args.embed_model,
            "same_faction_precision": round(total_same / total_retrieved * 100, 1) if total_retrieved else 0,
            "total_same": total_same,
            "total_retrieved": total_retrieved,
            "per_tick": [
                {
                    "tick_index": ht["index"],
                    "faction": ht["faction"],
                    "retrieved": rag_histories.get(ht["index"], []),
                }
                for ht in hazard_ticks
            ],
        }, f, indent=2)
    print(f"\nSaved retrieval data to {out_path}")


if __name__ == "__main__":
    main()
