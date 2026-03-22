#!/usr/bin/env python3
"""Evaluate a local HuggingFace model on a MAD hazard season.

Supports:
- Base model (Qwen3.5-0.8B)
- Base + LoRA + reservoir sidecar (from /code/rc checkpoint)

Usage:
    # Base model only
    python3 scripts/eval-local-model.py \
        --season seasons/focused-standing-hazard/season.json \
        --model-path Qwen/Qwen3.5-0.8B-Base

    # With sidecar checkpoint
    python3 scripts/eval-local-model.py \
        --season seasons/focused-standing-hazard/season.json \
        --model-path Qwen/Qwen3.5-0.8B-Base \
        --sidecar-checkpoint /code/rc/checkpoints/scaling/seq1024/final
"""

from __future__ import annotations

import argparse
import json
import math
import re
import sys
import time
from pathlib import Path

import torch


def load_season(path: str) -> dict:
    with open(path) as f:
        return json.load(f)


def extract_faction(source_text: str) -> str:
    try:
        start = source_text.index("sector. ") + len("sector. ")
        end = source_text.index(" is requesting", start)
        return source_text[start:end]
    except ValueError:
        return "unknown"


def build_prompt(tick: dict, state: dict, hazard_history: list[dict]) -> str:
    """Build a text prompt for the model from tick data."""
    lines = []
    lines.append("You are playing Mutual Agent Destruction (MAD) offline.")
    lines.append("Choose exactly one action by number.")
    lines.append("Reply with only the action number, for example: 1")
    lines.append("Do not explain. Goal: maximize long-run score.")
    lines.append("")

    # Source text
    if tick.get("sources"):
        lines.append(f"Source: {tick['sources'][0]['text']}")
        lines.append("")

    # State
    lines.append(f"Score: {state.get('score', 0)}, Reputation: {json.dumps(state.get('reputation', {}))}")
    lines.append("")

    # Hazard history
    if hazard_history:
        lines.append("Past hazard outcomes:")
        for rec in hazard_history[-10:]:  # last 10
            lines.append(f"  {rec['faction']}: invest_{rec['investment']} → yield {rec['yield']:+d}")
        lines.append("")

    # Action choices
    lines.append("Actions:")
    lines.append("1: hold")
    opps = tick.get("opportunities", [])
    idx = 2
    for opp in opps:
        for opt in opp.get("allowed_options", []):
            reqs = "; ".join(r.get("label", "") for r in opp.get("public_requirements", []))
            lines.append(f"{idx}: commit [{opt}] | req: {reqs}")
            idx += 1
    lines.append("")
    lines.append("Answer:")

    return "\n".join(lines)


def parse_action(response: str, n_options: int) -> int:
    """Parse action index from model response. Returns 1-based index."""
    # Find first digit
    match = re.search(r'\d+', response.strip())
    if match:
        idx = int(match.group())
        if 1 <= idx <= n_options:
            return idx
    return 1  # default to hold


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--season", required=True)
    parser.add_argument("--model-path", default="Qwen/Qwen3.5-0.8B-Base")
    parser.add_argument("--sidecar-checkpoint", default=None)
    parser.add_argument("--device", default="cuda" if torch.cuda.is_available() else "cpu")
    parser.add_argument("--max-new-tokens", type=int, default=8)
    parser.add_argument("--out", default=None)
    args = parser.parse_args()

    device = torch.device(args.device)
    dtype = torch.bfloat16

    season = load_season(args.season)
    print(f"Season: {season['season_id']}, {len(season['ticks'])} ticks")

    # Count hazard ticks
    hazard_indices = [i for i, t in enumerate(season["ticks"])
                      if t.get("annotations", {}).get("family") == "hazard_interrupt"]
    print(f"Hazard ticks: {len(hazard_indices)}")

    # Load model
    from transformers import AutoModelForCausalLM, AutoTokenizer, AutoConfig

    print(f"Loading {args.model_path}...")
    config = AutoConfig.from_pretrained(args.model_path, trust_remote_code=True)
    text_config = getattr(config, "text_config", None)
    tokenizer = AutoTokenizer.from_pretrained(args.model_path, trust_remote_code=True)
    if tokenizer.pad_token is None:
        tokenizer.pad_token = tokenizer.eos_token

    model = AutoModelForCausalLM.from_pretrained(
        args.model_path,
        config=text_config if text_config is not None else config,
        torch_dtype=dtype,
        device_map=str(device),
        trust_remote_code=True,
    )
    model.eval()

    # Load sidecar if specified
    esn = None
    hook_manager = None
    embed_layer = None
    sidecar_label = "base"

    if args.sidecar_checkpoint:
        ckpt = Path(args.sidecar_checkpoint)
        sidecar_label = "sidecar"

        # Load LoRA
        lora_path = ckpt / "lora_adapter"
        if lora_path.exists():
            print(f"Loading LoRA from {lora_path}...")
            from peft import PeftModel
            model = PeftModel.from_pretrained(model, str(lora_path))
            model.eval()

        # Load sidecar
        sidecar_config_path = ckpt / "sidecar_config.json"
        sidecar_weights_path = ckpt / "sidecar_weights.pt"
        if sidecar_config_path.exists() and sidecar_weights_path.exists():
            print("Loading reservoir sidecar...")
            sys.path.insert(0, str(Path("/code/rc")))
            from src.reservoir.esn import ESN
            from src.types import ReservoirConfig
            from scripts.train_track_a_readonly import ReadOnlySidecarBundle, SidecarHookManager

            with open(sidecar_config_path) as f:
                sc_cfg = json.load(f)

            hidden_dim = model.config.hidden_size if hasattr(model.config, 'hidden_size') else model.base_model.config.hidden_size
            reservoir_size = sc_cfg["reservoir_size"]
            sidecar_layers = sc_cfg["sidecar_layers"]

            reservoir_cfg = ReservoirConfig(
                size=reservoir_size, spectral_radius=0.9, leak_rate=0.5,
                input_scaling=1.0, sparsity=0.01, seed=42,
            )
            esn_cpu = ESN(reservoir_cfg, input_dim=hidden_dim)
            esn = esn_cpu.to_gpu(str(device)) if device.type == "cuda" else esn_cpu

            sidecar_bundle = ReadOnlySidecarBundle(
                layer_indices=sidecar_layers,
                reservoir_dim=reservoir_size,
                hidden_dim=hidden_dim,
                num_heads=sc_cfg.get("num_heads", 8),
                dropout=0.0,
                gate_init=sc_cfg.get("gate_init", 0.0),
                sidecar_type=sc_cfg.get("sidecar_type", "cross_attention"),
                use_delta=sc_cfg.get("use_delta", False),
                proj_hidden=sc_cfg.get("proj_hidden", 0),
            )
            sidecar_bundle.load_state_dict(
                torch.load(sidecar_weights_path, map_location=device), strict=False
            )
            sidecar_bundle = sidecar_bundle.to(device).to(dtype)
            sidecar_bundle.eval()

            embed_layer = model.get_input_embeddings()
            hook_manager = SidecarHookManager(model, sidecar_bundle, sidecar_layers)
            print(f"  Reservoir size: {reservoir_size}, sidecar layers: {sidecar_layers}")

    print(f"\nModel: {sidecar_label}")
    print("Running season...\n")

    # Initialize state from season
    initial = season.get("initial_state", {})
    state = {
        "score": 0,
        "yield": 0,
        "insight": 0,
        "aura": initial.get("aura", 0),
        "debt": initial.get("debt", 0),
        "reputation": dict(initial.get("reputation", {})),
    }

    hazard_history = []
    results = []
    total_score = 0

    for tick_idx in hazard_indices:
        tick = season["ticks"][tick_idx]
        sources = tick.get("sources", [])
        source_text = sources[0]["text"] if sources else ""
        faction = extract_faction(source_text)

        # Build prompt
        prompt = build_prompt(tick, state, hazard_history)

        # Count options
        n_options = 1  # hold
        for opp in tick.get("opportunities", []):
            n_options += len(opp.get("allowed_options", []))

        # Generate
        if esn is not None and hook_manager is not None:
            input_ids = tokenizer(prompt, return_tensors="pt", truncation=True, max_length=1024)["input_ids"].to(device)
            embeddings = embed_layer(input_ids)
            esn.reset()
            emb_np = embeddings[0].detach().float().cpu().numpy()
            states = esn.forward(emb_np)
            hook_manager.set_reservoir_states(states[None])

            with torch.no_grad():
                output_ids = model.generate(
                    input_ids, max_new_tokens=args.max_new_tokens,
                    pad_token_id=tokenizer.eos_token_id,
                )
            hook_manager.clear_reservoir_states()
        else:
            input_ids = tokenizer(prompt, return_tensors="pt", truncation=True, max_length=1024)["input_ids"].to(device)
            with torch.no_grad():
                output_ids = model.generate(
                    input_ids, max_new_tokens=args.max_new_tokens,
                    pad_token_id=tokenizer.eos_token_id,
                )

        response = tokenizer.decode(output_ids[0, input_ids.shape[1]:], skip_special_tokens=True)
        action_idx = parse_action(response, n_options)

        # Map action to scoring rule
        if action_idx == 1:
            # Hold
            action_cmd = "hold"
            action_option = None
        else:
            # Investment level
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

        # Find matching rule and score
        applied_rule = None
        for rule in tick["scoring"]["rules"]:
            if rule["match"]["command"] == action_cmd:
                if action_cmd == "hold":
                    applied_rule = rule
                    break
                if rule["match"].get("option") == action_option:
                    # Check requirements (simplified — just check rep)
                    reqs = rule.get("requirements", {})
                    min_rep = reqs.get("min_reputation", {})
                    meets_req = True
                    for fac, needed in min_rep.items():
                        if state["reputation"].get(fac, 0) < needed:
                            meets_req = False
                            break

                    if meets_req == (rule["classification"] in ("best", "bad") and "enough standing" not in rule.get("label", "")):
                        applied_rule = rule
                        break
                    elif not meets_req and "lacked the standing" in rule.get("label", ""):
                        applied_rule = rule
                        break

        if applied_rule is None:
            # Fallback to hold
            for rule in tick["scoring"]["rules"]:
                if rule["match"]["command"] == "hold":
                    applied_rule = rule
                    break

        delta = applied_rule["delta"] if applied_rule else {}
        yield_val = delta.get("yield", 0)
        score_delta = yield_val + delta.get("insight", 0) + delta.get("aura", 0) - delta.get("debt", 0) - delta.get("miss_penalties", 0)
        total_score += score_delta

        # Update state
        state["score"] = total_score
        effects = applied_rule.get("effects", {}) if applied_rule else {}
        for fac, d in effects.get("reputation_delta", {}).items():
            state["reputation"][fac] = state["reputation"].get(fac, 0) + d

        # Record
        invest_level = 0
        if action_option and action_option.startswith("invest_"):
            invest_level = int(action_option.split("_")[1])

        hazard_history.append({
            "faction": faction,
            "investment": invest_level,
            "yield": yield_val,
            "score_delta": score_delta,
        })

        correct = applied_rule["classification"] == "best" if applied_rule else False
        results.append({
            "tick": tick_idx,
            "faction": faction,
            "action": action_option or "hold",
            "yield": yield_val,
            "score_delta": score_delta,
            "correct": correct,
            "response": response.strip()[:50],
        })

        status = "✓" if correct else "✗"
        print(f"  [{tick_idx:3d}] {faction:<18} {action_option or 'hold':<10} yield={yield_val:+6d} score={total_score:+7d} {status}")

    # Summary
    best_count = sum(1 for r in results if r["correct"])
    print(f"\n{'='*60}")
    print(f"Model: {sidecar_label}")
    print(f"Score: {total_score}")
    print(f"Best-action: {best_count}/{len(results)} ({100*best_count/len(results):.1f}%)")
    print(f"Greedy reference: ~3,247")

    if args.out:
        out_path = Path(args.out)
        out_path.parent.mkdir(parents=True, exist_ok=True)
        with out_path.open("w") as f:
            json.dump({
                "model": sidecar_label,
                "model_path": args.model_path,
                "sidecar": args.sidecar_checkpoint,
                "score": total_score,
                "best_count": best_count,
                "total": len(results),
                "results": results,
            }, f, indent=2)
        print(f"Saved to {args.out}")


if __name__ == "__main__":
    main()
