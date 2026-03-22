#!/usr/bin/env python3
"""Run LIGHT memory framework (from BEAM) on a MAD hazard season.

Adapts LIGHT's three-component memory system (episodic retrieval +
working memory + scratchpad) to MAD's tick-by-tick decision loop.

After each tick:
  - Update scratchpad with outcome (LLM extracts salient facts)
  - Index outcome in episodic memory (embedding for retrieval)

Before each tick:
  - Retrieve top-K from episodic memory by similarity to current source text
  - Filter scratchpad entries for relevance
  - Build context from retrieved + working memory + filtered scratchpad
  - Call decision LLM with curated context

Requires BEAM repo at /code/beam with venv set up:
  cd /code/beam && uv venv .venv && uv pip install -r requirements.txt

Usage:
    /code/beam/.venv/bin/python scripts/eval-light-on-mad.py \
        --season seasons/focused-standing-hazard/season.json \
        --out runs/haiku-hazard-light.json
"""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import os
import re
import sys
import time
from pathlib import Path

# Add BEAM to path for imports
sys.path.insert(0, "/code/beam")

import numpy as np


# ---------------------------------------------------------------------------
# Embedding (OpenAI text-embedding-3-small)
# ---------------------------------------------------------------------------

def embed_texts(
    texts: list[str],
    model: str = "text-embedding-3-small",
    cache_dir: Path | None = None,
) -> list[list[float]]:
    from openai import OpenAI
    client = OpenAI()

    results: list[list[float] | None] = [None] * len(texts)
    to_embed: list[tuple[int, str]] = []

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

    if to_embed:
        batch_texts = [t for _, t in to_embed]
        resp = client.embeddings.create(model=model, input=batch_texts)
        sorted_data = sorted(resp.data, key=lambda x: x.index)
        for j, datum in enumerate(sorted_data):
            idx = to_embed[j][0]
            results[idx] = datum.embedding
            if cache_dir:
                text = to_embed[j][1]
                h = hashlib.sha256(f"{model}:{text}".encode()).hexdigest()[:16]
                with (cache_dir / f"{h}.json").open("w") as f:
                    json.dump(datum.embedding, f)

    return results


def cosine_sim(a: list[float], b: list[float]) -> float:
    dot = sum(x * y for x, y in zip(a, b))
    na = math.sqrt(sum(x * x for x in a))
    nb = math.sqrt(sum(x * x for x in b))
    return dot / (na * nb) if na and nb else 0.0


# ---------------------------------------------------------------------------
# LLM calls
# ---------------------------------------------------------------------------

def llm_call(prompt: str, model: str = "haiku", effort: str = "low", max_tokens: int = 256) -> str:
    """Call Claude via CLI (same method as MAD harness)."""
    import subprocess
    args = [
        "claude", "-p",
        "--model", model,
        "--tools", "",
        "--effort", effort,
        "--no-session-persistence",
        prompt,
    ]
    result = subprocess.run(
        args,
        capture_output=True,
        text=True,
        timeout=90,
    )
    if result.returncode != 0:
        raise RuntimeError(f"claude failed: {result.stderr.strip()}")
    return result.stdout.strip()


# ---------------------------------------------------------------------------
# LIGHT memory components (adapted for online/incremental use)
# ---------------------------------------------------------------------------

class LIGHTMemory:
    """Online adaptation of LIGHT's three-component memory for MAD."""

    def __init__(self, scratchpad_model: str = "haiku",
                 scratchpad_effort: str = "low",
                 embed_model: str = "text-embedding-3-small",
                 cache_dir: Path | None = None):
        self.scratchpad_model = scratchpad_model
        self.scratchpad_effort = scratchpad_effort
        self.embed_model = embed_model
        self.cache_dir = cache_dir

        # Scratchpad: running text of extracted facts
        self.scratchpad: str = ""
        self.scratchpad_token_limit = 4000  # chars, ~1K tokens

        # Episodic memory: embedded outcome records
        self.episodic_texts: list[str] = []
        self.episodic_embeddings: list[list[float]] = []
        self.episodic_records: list[dict] = []

        # Working memory: last N raw outcomes
        self.working_memory: list[str] = []
        self.working_memory_limit = 10

    def update_after_tick(self, tick_text: str, outcome_text: str, record: dict):
        """Update all memory components after a tick outcome."""
        # 1. Update scratchpad
        self._update_scratchpad(tick_text, outcome_text)

        # 2. Add to episodic memory
        emb = embed_texts([outcome_text], model=self.embed_model, cache_dir=self.cache_dir)
        self.episodic_texts.append(outcome_text)
        self.episodic_embeddings.append(emb[0])
        self.episodic_records.append(record)

        # 3. Update working memory
        self.working_memory.append(outcome_text)
        if len(self.working_memory) > self.working_memory_limit:
            self.working_memory = self.working_memory[-self.working_memory_limit:]

    def _update_scratchpad(self, tick_text: str, outcome_text: str):
        """Extract salient facts and append to scratchpad, compressing if needed."""
        # Extract facts from this outcome
        extract_prompt = (
            "Extract the key facts from this game outcome in concise key:value format.\n\n"
            f"Context: {tick_text}\n"
            f"Outcome: {outcome_text}\n\n"
            "Output only the key facts, one per line. Focus on faction name, "
            "investment level, and yield/score impact."
        )
        facts = llm_call(extract_prompt, model=self.scratchpad_model, effort=self.scratchpad_effort)
        self.scratchpad += "\n" + facts

        # Compress if too long
        if len(self.scratchpad) > self.scratchpad_token_limit * 2:
            compress_prompt = (
                "Summarize these game observations into the most important patterns. "
                "Focus on which factions give positive vs negative returns at each "
                f"investment level. Keep under {self.scratchpad_token_limit} characters.\n\n"
                f"{self.scratchpad}"
            )
            self.scratchpad = llm_call(compress_prompt, model=self.scratchpad_model, effort=self.scratchpad_effort)

    def retrieve_context(self, query_text: str, top_k: int = 5) -> str:
        """Retrieve relevant context for a decision."""
        parts = []

        # 1. Retrieve from episodic memory
        if self.episodic_embeddings:
            q_emb = embed_texts([query_text], model=self.embed_model, cache_dir=self.cache_dir)[0]
            scores = [(i, cosine_sim(q_emb, e)) for i, e in enumerate(self.episodic_embeddings)]
            scores.sort(key=lambda x: x[1], reverse=True)
            top = scores[:min(top_k, len(scores))]
            if top:
                retrieved = "\n".join(self.episodic_texts[i] for i, _ in top)
                parts.append(f"Retrieved observations:\n{retrieved}")

        # 2. Working memory (recent outcomes)
        if self.working_memory:
            recent = "\n".join(self.working_memory[-5:])
            parts.append(f"Recent outcomes:\n{recent}")

        # 3. Scratchpad (accumulated knowledge)
        if self.scratchpad.strip():
            parts.append(f"Accumulated knowledge:\n{self.scratchpad}")

        return "\n\n".join(parts)


# ---------------------------------------------------------------------------
# MAD season runner
# ---------------------------------------------------------------------------

def extract_faction(source_text: str) -> str:
    try:
        start = source_text.index("sector. ") + len("sector. ")
        end = source_text.index(" is requesting", start)
        return source_text[start:end]
    except ValueError:
        return "unknown"


def build_decision_prompt(tick: dict, state: dict, light_context: str) -> str:
    """Build decision prompt with LIGHT-curated context."""
    lines = []
    lines.append("You are playing Mutual Agent Destruction (MAD) offline.")
    lines.append("Choose exactly one action by number.")
    lines.append("Reply with only the action number, for example: 1")
    lines.append("Do not explain. Goal: maximize long-run score.")
    lines.append("")

    if tick.get("sources"):
        lines.append(f"Source: {tick['sources'][0]['text']}")
        lines.append("")

    lines.append(f"Score: {state.get('score', 0)}")
    lines.append(f"Reputation: {json.dumps(state.get('reputation', {}))}")
    lines.append("")

    if light_context:
        lines.append("Memory context:")
        lines.append(light_context)
        lines.append("")

    lines.append("Actions:")
    lines.append("1: hold")
    idx = 2
    for opp in tick.get("opportunities", []):
        for opt in opp.get("allowed_options", []):
            reqs = "; ".join(r.get("label", "") for r in opp.get("public_requirements", []))
            lines.append(f"{idx}: commit [{opt}] | req: {reqs}")
            idx += 1

    return "\n".join(lines)


def parse_action(response: str, n_options: int) -> int:
    match = re.search(r'\d+', response.strip())
    if match:
        idx = int(match.group())
        if 1 <= idx <= n_options:
            return idx
    return 1


def find_matching_rule(tick: dict, action_cmd: str, action_option: str | None, state: dict) -> dict | None:
    """Find the scoring rule that matches the action."""
    for rule in tick["scoring"]["rules"]:
        if rule["match"]["command"] != action_cmd:
            continue
        if action_cmd == "hold":
            return rule
        if rule["match"].get("option") != action_option:
            continue
        # Check requirements
        reqs = rule.get("requirements", {})
        min_rep = reqs.get("min_reputation", {})
        meets_req = all(
            state["reputation"].get(fac, 0) >= needed
            for fac, needed in min_rep.items()
        )
        if meets_req and "lacked the standing" not in rule.get("label", ""):
            return rule
        if not meets_req and "lacked the standing" in rule.get("label", ""):
            return rule
    return None


def main():
    parser = argparse.ArgumentParser(description="Run LIGHT on MAD hazard season")
    parser.add_argument("--season", required=True)
    parser.add_argument("--decision-model", default="haiku")
    parser.add_argument("--scratchpad-model", default="haiku")
    parser.add_argument("--effort", default="low")
    parser.add_argument("--top-k", type=int, default=5)
    parser.add_argument("--out", default="runs/haiku-hazard-light.json")
    args = parser.parse_args()

    with open(args.season) as f:
        season = json.load(f)

    hazard_indices = [i for i, t in enumerate(season["ticks"])
                      if t.get("annotations", {}).get("family") == "hazard_interrupt"]

    print(f"Season: {season['season_id']}, {len(hazard_indices)} hazard ticks")

    # Initialize
    cache_dir = Path("build/light_embed_cache")
    memory = LIGHTMemory(
        scratchpad_model=args.scratchpad_model,
        scratchpad_effort=args.effort,
        cache_dir=cache_dir,
    )

    initial = season.get("initial_state", {})
    state = {
        "score": 0,
        "reputation": dict(initial.get("reputation", {})),
    }

    results = []
    total_score = 0
    t_start = time.time()

    for hi, tick_idx in enumerate(hazard_indices):
        tick = season["ticks"][tick_idx]
        source_text = tick["sources"][0]["text"] if tick.get("sources") else ""
        faction = extract_faction(source_text)

        # Count options
        n_options = 1
        for opp in tick.get("opportunities", []):
            n_options += len(opp.get("allowed_options", []))

        # Retrieve context from LIGHT memory
        light_context = memory.retrieve_context(source_text, top_k=args.top_k)

        # Build prompt and decide
        prompt = build_decision_prompt(tick, state, light_context)
        response = llm_call(prompt, model=args.decision_model, effort=args.effort)
        action_idx = parse_action(response, n_options)

        # Map to action
        if action_idx == 1:
            action_cmd, action_option = "hold", None
        else:
            opt_idx = 0
            action_option = None
            for opp in tick.get("opportunities", []):
                for opt in opp.get("allowed_options", []):
                    opt_idx += 1
                    if opt_idx + 1 == action_idx:
                        action_option = opt
                        break
                if action_option:
                    break
            action_cmd = "commit" if action_option else "hold"

        # Score
        rule = find_matching_rule(tick, action_cmd, action_option, state)
        if rule is None:
            for r in tick["scoring"]["rules"]:
                if r["match"]["command"] == "hold":
                    rule = r
                    break

        delta = rule["delta"] if rule else {}
        yield_val = delta.get("yield", 0)
        score_delta = (yield_val + delta.get("insight", 0) + delta.get("aura", 0)
                       - delta.get("debt", 0) - delta.get("miss_penalties", 0))
        total_score += score_delta
        state["score"] = total_score

        # Update state
        effects = rule.get("effects", {}) if rule else {}
        for fac, d in effects.get("reputation_delta", {}).items():
            state["reputation"][fac] = state["reputation"].get(fac, 0) + d

        invest_level = 0
        if action_option and action_option.startswith("invest_"):
            invest_level = int(action_option.split("_")[1])

        correct = rule["classification"] == "best" if rule else False

        # Build outcome text for memory
        outcome_text = (
            f"Faction: {faction} | Investment: {invest_level} | "
            f"Yield: {yield_val:+d} | Score delta: {score_delta:+d} | "
            f"Label: {rule.get('label', 'unknown')}"
        )

        # Update LIGHT memory
        memory.update_after_tick(source_text, outcome_text, {
            "faction": faction,
            "investment": invest_level,
            "yield": yield_val,
            "score_delta": score_delta,
        })

        results.append({
            "tick": tick_idx,
            "faction": faction,
            "action": action_option or "hold",
            "yield": yield_val,
            "score_delta": score_delta,
            "correct": correct,
        })

        status = "+" if correct else "-"
        print(f"  [{hi+1:2d}/{len(hazard_indices)}] t={tick_idx:3d} {faction:<18} "
              f"{action_option or 'hold':<10} yield={yield_val:+6d} "
              f"score={total_score:+7d} {status}")

    elapsed = time.time() - t_start
    best_count = sum(1 for r in results if r["correct"])

    print(f"\n{'='*60}")
    print(f"LIGHT on MAD hazard")
    print(f"Score: {total_score}")
    print(f"Best-action: {best_count}/{len(results)} ({100*best_count/len(results):.1f}%)")
    print(f"Elapsed: {elapsed:.0f}s")
    print(f"LLM calls: ~{len(hazard_indices) * 3} (retrieve + scratchpad + decide)")
    print(f"Greedy reference: ~3,247")

    out_path = Path(args.out)
    out_path.parent.mkdir(parents=True, exist_ok=True)
    with out_path.open("w") as f:
        json.dump({
            "framework": "LIGHT",
            "decision_model": args.decision_model,
            "scratchpad_model": args.scratchpad_model,
            "score": total_score,
            "best_count": best_count,
            "total": len(results),
            "elapsed_s": round(elapsed, 1),
            "scratchpad_final": memory.scratchpad[:2000],
            "results": results,
        }, f, indent=2)
    print(f"Saved to {args.out}")


if __name__ == "__main__":
    main()
