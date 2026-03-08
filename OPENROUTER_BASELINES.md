# OpenRouter Baseline Candidates

Practical notes from direct OpenRouter testing on MAD's `1-token logprob-choice`
path.

Method used:
- prompt: `Choose exactly one action. Valid actions: A, B, C, D. Reply with only the action letter.`
- API path: chat completions
- params: `max_tokens=1`, `temperature=0`, `logprobs=true`, `top_logprobs=5`

This matters because OpenRouter's advertised `TPS` is not the same as actual
`MAD ticks/sec` for tiny outputs. For this benchmark path, the critical question
is whether a model/provider route returns **usable non-null top_logprobs on the
first token**.

## Current Recommendation

- Default logprob baseline: `openai/gpt-4o-mini`
- Fastest confirmed working logprob model: `openai/gpt-3.5-turbo`
- Cheapest confirmed working logprob model: `neversleep/llama-3.1-lumimaid-8b`

## Best Confirmed Working Candidates

| Model | Advertised Best Route | Advertised p50 TPS | Advertised p50 Lat | Actual Route | Actual mean | Actual p50 | Logprobs OK | Notes |
|---|---|---:|---:|---|---:|---:|---:|---|
| `openai/gpt-3.5-turbo` | `OpenAI` | `101` | `489ms` | `OpenAI` | `341ms` | `345ms` | `3/3` | Fastest clean result seen so far. |
| `openai/gpt-4o-mini` | `Azure` | `47` | `509ms` | `OpenAI` | `549ms` | `619ms` | `3/3` | Current default logprob baseline. |
| `neversleep/llama-3.1-lumimaid-8b` | `NextBit` | `94` | `395ms` | `NextBit` | `483ms` | `433ms` | `3/3` | Cheapest confirmed working option. |
| `thedrummer/unslopnemo-12b` | `NextBit` | `70.5` | `430ms` | `NextBit` | `526ms` | `584ms` | `3/3` | Works, but no clear win over `lumimaid-8b`. |
| `qwen/qwen3.5-122b-a10b` | `Alibaba Cloud Int.` | `125` | `614ms` | `Novita` | `2877ms` | `2868ms` | `2/3` | Too slow and inconsistent for this lane. |

## Other Models That Returned Real Logprobs In One-Shot Probes

These returned non-null `top_logprobs` at least once on the exact `1-token`
probe path, but were not promoted into the default shortlist:

- `openai/gpt-4o`
- `openai/gpt-4o-2024-11-20`
- `openai/gpt-4o-2024-05-13`
- `openai/gpt-3.5-turbo-0613`
- `mancer/weaver`
- `undi95/remm-slerp-l2-13b`

## Models Tested That Were Not Usable For This Path

- `openai/gpt-oss-20b`
  - with `reasoning.exclude=true`, sometimes emitted a bare answer, but still
    frequently returned `content=null` with reasoning tokens consuming the
    budget.
- `openai/gpt-oss-120b`
  - same pattern as `gpt-oss-20b`; no stable logprob path found.
- `qwen/qwen3-32b`
  - often returned `content=null`; no usable `top_logprobs`.
- `meta-llama/llama-3.1-8b-instruct`
  - returned content, but `logprobs` stayed null on the tested route.

## MAD Harness Result: `gpt-4o-mini`

Current special-case harness support exists for:

- `openai/gpt-4o-mini`

In this mode the harness:
- labels actions as `A`, `B`, `C`, ...
- asks for only the action letter
- uses `max_tokens=1`
- reads `top_logprobs` locally
- does **not** force throughput routing, because the provider path that returned
  real `top_logprobs` was the stable default `OpenAI` route

5-tick smoke result:
- score: `-2`
- errors: `0/5`
- avg step: `514ms`
- p50: `484ms`
- p95: `681ms`

## Main Conclusion

For MAD's `1-token logprob-choice` baseline, the important distinction is not
"supports logprobs" on paper. It is:

- does the routed provider return non-null `top_logprobs`
- on the first token
- reliably

So far:
- `gpt-4o-mini` is the best default
- `gpt-3.5-turbo` is the fastest confirmed clean option
- `lumimaid-8b` is the cheapest confirmed clean option
