---
status: accepted
date: 2026-10-07
---

# The code proof is a gate from the start: itos-cc's mutation check over the functions an item changes

## Context and Problem Statement

Is the code proof (itos's proof.code through itos-cc's mutation check) a gate from the start, or advice first? Recommended: a gate from the start: itos work done refuses an item touching cmd or internal until mutation check --since {base} --fail-uncovered passes over cmd and internal, results recorded with --all-tests and committed in .metrics/mutate/, survivors killed by a test or excepted with a reason for the person, an uncovered mutant needing a test. Or: advisory first, agents reporting survivors with no gate.

Asked as q-27, about T-20.

## Considered Options

- A gate from the start
- Advisory first

## Decision Outcome

A gate from the start, judged by function, as T-20 built it. Until itos runs the proof in CI (donvargax/itos#28) it runs at work done only, with no CI step of our own. The user's calls, 2026-10-08.

### Consequences

T-20 set proof.code; AGENTS.md's Finishing says how an item meets it, and that no code is reshaped only to hold nothing to mutate. Function-level judging stays; the gaps it leaves are itos-cc's to close (donvargax/itos-cc#23, #24), the timing itos's (donvargax/itos#28, the idea proof-in-ci). A template's projects carry the same gate.
