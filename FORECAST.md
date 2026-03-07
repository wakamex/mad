# MAD Forecasts

Forecasts for the current `dev1000` season (`seasons/dev1000/season.json`), assuming the current harness, current prompt packet, and current score logic.

These are directional planning estimates, not promises. They are intended to answer:

- which configurations are worth running first
- what should count as "too easy" or "too hard"
- how much each knob should matter: model, effort, memory, context, and Codex service tier

## Scope

- Score forecasts are for a single full `1000`-tick run.
- Runtime forecasts are wall-clock estimates for a single full run on the current local CLI setup.
- Codex base forecasts assume `service_tier=flex`, `memory=on`, `context=persistent`.
- Claude base forecasts assume `memory=on`, `context=persistent`.
- To forecast any other permutation, start from the base row and then apply the adjustments in the permutation sections below.

## Observed Anchors

These are real prior runs or near-runs that anchor the forecast ranges:

- `codex:gpt-5.2-codex@high`, persistent-context harness: about `36.9k`
- `codex:gpt-5.1-codex-mini@medium`, persistent-context harness: about `15.8k`
- `claude:haiku@low`, persistent-context harness: about `23.0k`
- `codex:gpt-5.1-codex-mini@medium+mem-off+ctx-ephemeral+tier-fast`: forecasted around low-positive, likely a few thousand at most

## Codex Base Forecasts

Reference configuration:

- `memory=on`
- `context=persistent`
- `service_tier=flex`

| Model | Effort | Forecast Score | Runtime |
|---|---:|---:|---:|
| `gpt-5.4` | `medium` | `34k-46k` | `10-16h` |
| `gpt-5.4` | `high` | `38k-50k` | `12-18h` |
| `gpt-5.4` | `xhigh` | `40k-52k` | `14-20h` |
| `gpt-5.3-codex` | `medium` | `32k-44k` | `9-15h` |
| `gpt-5.3-codex` | `high` | `36k-48k` | `11-17h` |
| `gpt-5.3-codex` | `xhigh` | `38k-50k` | `13-19h` |
| `gpt-5.2-codex` | `medium` | `28k-38k` | `8-14h` |
| `gpt-5.2-codex` | `high` | `34k-42k` | `10-16h` |
| `gpt-5.2-codex` | `xhigh` | `35k-44k` | `11-17h` |
| `gpt-5.1-codex-max` | `medium` | `30k-42k` | `9-15h` |
| `gpt-5.1-codex-max` | `high` | `35k-47k` | `11-17h` |
| `gpt-5.1-codex-max` | `xhigh` | `37k-49k` | `13-19h` |
| `gpt-5.2` | `medium` | `24k-36k` | `8-14h` |
| `gpt-5.2` | `high` | `29k-40k` | `10-16h` |
| `gpt-5.2` | `xhigh` | `31k-42k` | `12-18h` |
| `gpt-5.1-codex-mini` | `medium` | `14k-20k` | `4-7h` |
| `gpt-5.1-codex-mini` | `high` | `16k-22k` | `5-8h` |

## Claude Base Forecasts

Reference configuration:

- `memory=on`
- `context=persistent`

| Model | Effort | Forecast Score | Runtime |
|---|---:|---:|---:|
| `haiku` | `low` | `18k-26k` | `5-9h` |
| `haiku` | `medium` | `20k-28k` | `6-10h` |
| `haiku` | `high` | `21k-30k` | `7-11h` |
| `sonnet` | `low` | `24k-34k` | `8-14h` |
| `sonnet` | `medium` | `28k-40k` | `10-16h` |
| `sonnet` | `high` | `31k-43k` | `12-18h` |
| `opus` | `low` | `30k-42k` | `12-20h` |
| `opus` | `medium` | `34k-46k` | `15-24h` |
| `opus` | `high` | `38k-50k` | `18-30h` |

## Permutation Effects: Codex

Start from the Codex base row above, then apply these adjustments.

### `service_tier=fast`

- Score effect: expected to be effectively `0`
- Runtime effect: usually `0.75x` to `0.90x` of `flex`
- Interpretation: fast should change latency, not strategic quality. Any score difference should be treated as noise, not signal.

### `memory=off`, `context=persistent`

- Frontier Codex models: subtract about `2k-6k`
- `gpt-5.1-codex-mini`: subtract about `1k-3k`
- Runtime: slightly better or unchanged

### `memory=on`, `context=ephemeral`

- Frontier Codex models: subtract about `8k-18k`
- `gpt-5.1-codex-mini`: subtract about `8k-14k`
- Runtime: usually `0.70x` to `0.90x` of persistent runs because prompts stay shorter

### `memory=off`, `context=ephemeral`

- Usually only slightly worse than `memory=on + context=ephemeral`
- Additional penalty beyond ephemeral context: about `0k-3k`
- This is the clean "short-horizon local competence" baseline

### Canonical Codex Permutations

| Permutation | Expected Relative Strength |
|---|---|
| `persistent + memory on + flex` | strongest and fairest Codex benchmark mode |
| `persistent + memory on + fast` | nearly same score, faster wall time |
| `persistent + memory off + flex` | strong, but loses some long-range recall |
| `persistent + memory off + fast` | similar to above, slightly faster |
| `ephemeral + memory on + flex` | much weaker; mostly visible-state and local heuristics |
| `ephemeral + memory on + fast` | same strategic weakness, faster |
| `ephemeral + memory off + flex` | "no-context/no-memory" baseline |
| `ephemeral + memory off + fast` | fastest true Codex baseline |

## Permutation Effects: Claude

Start from the Claude base row above, then apply these adjustments.

### `memory=off`, `context=persistent`

- Small to moderate score drop: about `1k-4k`
- Lower impact than removing context

### `memory=on`, `context=ephemeral`

- Large score drop: about `8k-18k`
- This is the main "Claude no-context" hit

### `memory=off`, `context=ephemeral`

- Usually only a little worse than `memory=on + context=ephemeral`
- Additional penalty beyond ephemeral context: about `0k-3k`

### Canonical Claude Permutations

| Permutation | Expected Relative Strength |
|---|---|
| `persistent + memory on` | strongest Claude mode |
| `persistent + memory off` | slightly weaker |
| `ephemeral + memory on` | much weaker |
| `ephemeral + memory off` | clean Claude no-context/no-memory baseline |

## Forecasts For The Fastest Baselines

### Fastest Codex baseline worth running first

`gpt-5.1-codex-mini@medium + memory=off + context=ephemeral + service_tier=fast`

- single-run score: roughly `0` to `6k`
- best guess: around `+3k`
- runtime: roughly `4h` to `6h`

### Fastest Claude baseline worth running first

`haiku@low + memory=off + context=ephemeral`

- single-run score: roughly `+2k` to `+10k`
- best guess: around `+6k`
- runtime: roughly `5h` to `8h`

## Forecasts For `--runs 3`

These are forecasts for the aggregate at the end of a three-run sweep, not for each individual run.

### `gpt-5.1-codex-mini@medium + mem-off + ctx-ephemeral + tier-fast`

- mean score: `+2k` to `+4k`
- median score: `+1k` to `+4k`
- p90 score: `+4k` to `+7k`
- likely interpretation: clearly above random, clearly far below persistent frontier play

### `gpt-5.2-codex@high + mem-off + ctx-ephemeral + tier-fast`

- mean score: `+5k` to `+12k`
- median score: `+4k` to `+11k`
- p90 score: `+10k` to `+18k`

### `haiku@low + mem-off + ctx-ephemeral`

- mean score: `+4k` to `+9k`
- median score: `+4k` to `+8k`
- p90 score: `+8k` to `+14k`

### `sonnet@medium + mem-off + ctx-ephemeral`

- mean score: `+8k` to `+16k`
- median score: `+7k` to `+15k`
- p90 score: `+14k` to `+22k`

## What Would Surprise Me

- A no-context/no-memory baseline consistently above `10k` on `gpt-5.1-codex-mini`
  - likely means the season is too locally legible
- A persistent frontier run below `15k`
  - likely means the prompting or action envelope is broken
- Random or near-random baselines ending positive most of the time
  - likely means too much immediate EV is exposed tick-by-tick
- Any consistent score gains from Codex `service_tier=fast`
  - that would be suspicious; it should mainly change speed, not outcome quality

## Overall Rationale

The current `dev1000` season appears to reward three layers of capability:

1. **Visible-state competence**
   - exact reputation, debt, cooldown, and opportunity surface management
   - enough to beat random

2. **Mid-horizon continuity**
   - carrying forward useful notes
   - preserving commitments and plan fragments
   - this is where persistent context starts separating from ephemeral runs

3. **Long-horizon recomposition**
   - combining distant clues
   - exploiting retroactive reinterpretation
   - spending reputation, time, and standing work for option value rather than immediate payout
   - this is where frontier persistent runs pull away

That leads to the expected ordering:

- `persistent + memory on` is best
- `persistent + memory off` is somewhat worse
- `ephemeral + memory on` is much worse
- `ephemeral + memory off` is the cleanest short-horizon baseline

The largest single knob is **context continuity**, not the native memory feature toggle.
The smallest single knob is **Codex fast vs flex**, which should be score-neutral and mostly change runtime.

In short:

- the season is hard enough that no-context baselines should not win
- the season is still locally legible enough that no-context baselines should beat random
- the real ceiling should come from persistent planning, not twitch speed
