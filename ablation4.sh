#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

./scripts/mad-run \
  --provider claude \
  --model haiku \
  --effort low \
  --memory off \
  --context ephemeral \
  --recent-reveals 6 \
  --text-mode full \
  --runs 1 \
  --season ./seasons/dev1000/season.json \
  --max-ticks 1000 \
  --name ablation-haiku-ephemeral-full-reveals
