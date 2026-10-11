---
status: accepted
date: 2026-10-11
---

# Testing tiers: two-run mutation locally, changed-line proof per push, sampled nightly

## Context and Problem Statement

Which mutation survivors get a test or an exception: those of own tests alone, or those own tests and then the whole suite leave (itos#41)?

Asked as q-54, about T-43.

## Considered Options

The options are those the question names.

## Decision Outcome

Own tests first, then the whole suite on what they leave: a survivor of both gets a test or an exception. This repository takes itos#41's tiers: locally an item's two runs, --fail-fast while fixing and a sample of one; per push the code proof over changed lines (--fail-uncovered=lines) reading .metrics/mutate/, and a small sample; nightly a larger sample seeded by the day. The person's calls, 2026-10-10.

### Consequences

None recorded.
