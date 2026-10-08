---
status: accepted
date: 2026-10-07
---

# The owner's own tools, itos and itos-cc, are pinned when out; every other tool waits 7 days

## Context and Problem Statement

Does tools/bin/pinned's rule that a pinned version is at least 7 days old hold for the owner's own tools, itos and itos-cc? Recommended: no: the rule guards against a supply-chain attack on others' releases, found within days; the owner's own are pinned when out, as itos 6.4.0 was. Or: the same 7 days for every tool, itos's own repository waiting so for itos-cc.

Asked as q-28, about T-20.

## Considered Options

- Own tools pinned when out
- 7 days for every tool

## Decision Outcome

The owner's own tools are pinned when out; the 7 days hold for every other tool. The user's call, 2026-10-08.

### Consequences

tools/bin/pinned's comment says so; itos-cc v0.5.0 was pinned the day after its release (T-20), itos 7.0.0-rc.1 the day of it (T-18).
