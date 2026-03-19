#!/usr/bin/env python3
"""Simulate RAG-based hazard memory by embedding past outcomes and retrieving top-K.

For each hazard tick in a completed run:
1. Embed the current tick's source text as the query
2. Embed all past hazard outcome records as documents
3. Retrieve top-K by cosine similarity
4. Report what was retrieved vs what faction-filter would have given

This is an offline analysis — it doesn't re-run the harness, just measures
whether embedding retrieval naturally finds same-faction outcomes.

Usage:
    export OPENAI_API_KEY=...
    python3 scripts/rag-hazard-sim.py \
        --season seasons/focused-standing-hazard/season.json \
        --run runs/haiku-hazard-ephemeral-with-history.json \
        --k 5
"""

import argparse
import json
import os
import sys
import time

def embed_texts(texts: list[str], model: str = "text-embedding-3-small") -> list[list[float]]:
    """Embed texts using OpenAI API."""
    from openai import OpenAI
    client = OpenAI(api_key=os.environ["OPENAI_API_KEY"])
    all_embeddings = []
    batch_size = 100
    for i in range(0, len(texts), batch_size):
        batch = texts[i:i+batch_size]
        resp = client.embeddings.create(model=model, input=batch)
        sorted_data = sorted(resp.data, key=lambda x: x.index)
        all_embeddings.extend([d.embedding for d in sorted_data])
    return all_embeddings


def cosine_similarity(a: list[float], b: list[float]) -> float:
    dot = sum(x * y for x, y in zip(a, b))
    norm_a = sum(x * x for x in a) ** 0.5
    norm_b = sum(x * x for x in b) ** 0.5
    return dot / (norm_a * norm_b) if norm_a and norm_b else 0.0


def main():
    parser = argparse.ArgumentParser(description="Simulate RAG retrieval on hazard outcomes")
    parser.add_argument("--season", required=True, help="Season JSON file")
    parser.add_argument("--run", required=True, help="Run report JSON file")
    parser.add_argument("--k", type=int, default=5, help="Top-K to retrieve")
    parser.add_argument("--embed-model", default="text-embedding-3-small")
    args = parser.parse_args()

    with open(args.season) as f:
        season = json.load(f)
    with open(args.run) as f:
        run = json.load(f)

    # Extract hazard ticks and outcomes from the run
    hazard_steps = []
    for step in run["runs"][0]["steps"]:
        if step.get("outcome", {}).get("annotations", {}).get("family") == "hazard_interrupt":
            # Extract faction from source text
            sources = step["prompt"]["current_tick"].get("sources", [])
            source_text = sources[0]["text"] if sources else ""
            faction = "unknown"
            if "sector. " in source_text and " is requesting" in source_text:
                start = source_text.index("sector. ") + len("sector. ")
                end = source_text.index(" is requesting", start)
                faction = source_text[start:end]

            option = step["outcome"]["applied_action"].get("option", "hold")
            invest_level = 0
            if option.startswith("invest_"):
                invest_level = int(option.split("_")[1])

            hazard_steps.append({
                "tick_index": step["prompt"]["tick_index"],
                "faction": faction,
                "source_text": source_text,
                "investment": invest_level,
                "yield": step["outcome"]["applied_rule"]["delta"].get("yield", 0),
                "score_delta": step["outcome"]["score_delta"],
                "correct": step["outcome"]["correct"],
            })

    print(f"Found {len(hazard_steps)} hazard steps")
    print(f"Embedding {len(hazard_steps)} source texts + outcome records...")

    # Build all texts to embed
    # Queries: source text for each hazard tick
    query_texts = [s["source_text"] for s in hazard_steps]

    # Documents: outcome records as text strings
    doc_texts = []
    for s in hazard_steps:
        doc_texts.append(
            f"Faction: {s['faction']} | Investment: {s['investment']} | "
            f"Yield: {s['yield']:+d} | Score delta: {s['score_delta']:+d}"
        )

    # Embed everything in one batch
    all_texts = query_texts + doc_texts
    print(f"Embedding {len(all_texts)} texts with {args.embed_model}...")
    all_embeddings = embed_texts(all_texts, model=args.embed_model)

    query_embeddings = all_embeddings[:len(query_texts)]
    doc_embeddings = all_embeddings[len(query_texts):]

    # Simulate retrieval for each hazard tick
    print(f"\n{'Tick':>4} {'Faction':<18} {'K':>2} | {'Retrieved factions':<50} | {'Same-faction hits':>5}")
    print("-" * 100)

    total_same_faction = 0
    total_retrieved = 0
    total_ticks_with_history = 0

    for i, step in enumerate(hazard_steps):
        # Available documents: all outcomes BEFORE this tick
        if i == 0:
            continue  # no history yet

        total_ticks_with_history += 1
        q_emb = query_embeddings[i]

        # Score all prior documents
        scores = []
        for j in range(i):
            sim = cosine_similarity(q_emb, doc_embeddings[j])
            scores.append((j, sim))

        scores.sort(key=lambda x: x[1], reverse=True)
        top_k = scores[:min(args.k, len(scores))]

        # Check what was retrieved
        retrieved_factions = [hazard_steps[j]["faction"] for j, _ in top_k]
        same_faction_hits = sum(1 for f in retrieved_factions if f == step["faction"])
        total_same_faction += same_faction_hits
        total_retrieved += len(top_k)

        faction_summary = ", ".join(f[:12] for f in retrieved_factions)
        print(f"{step['tick_index']:4d} {step['faction']:<18} {len(top_k):2d} | {faction_summary:<50} | {same_faction_hits:5d}/{len(top_k)}")

    print("-" * 100)
    if total_retrieved > 0:
        precision = 100 * total_same_faction / total_retrieved
        print(f"\nSame-faction precision: {total_same_faction}/{total_retrieved} = {precision:.1f}%")
        print(f"(Ideal RAG would achieve 100% — always retrieves same-faction outcomes)")
        print(f"(Random retrieval baseline: ~{100/7:.0f}% for 7 factions)")


if __name__ == "__main__":
    main()
