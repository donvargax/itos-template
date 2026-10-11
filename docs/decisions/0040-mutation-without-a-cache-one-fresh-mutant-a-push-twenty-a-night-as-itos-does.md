---
status: accepted
date: 2026-10-11
---

# Mutation without a cache: one fresh mutant a push, twenty a night, as itos does

## Context and Problem Statement

Does the mutation proof read outcomes recorded in .metrics/mutate/, or judge a few fresh mutants itself, as itos does since its answer to itos#41?

Asked as q-55, about T-45.

## Considered Options

- Recorded outcomes in .metrics/mutate/, every changed function judged before it lands (decision 39)
- Fresh counted runs with no cache, as itos does

## Decision Outcome

Fresh mutants, no cache, as itos does (its q-41, q-52, q-55, q-57). Per push the code proof is itos-cc mutation run --count 1 --since {base} --fail-uncovered=lines --all-tests: one mutation site of the changed functions judged freshly, and every line the range adds or changes covered. Nightly, 20 sites across cmd and internal, seeded by the run. Locally an agent runs the push's command once before pushing. Nothing is recorded or committed. --all-tests until the scenarios that reach a site can be listed (the idea listed-tests). Weighed against the cache, which judged every changed function before it landed but needed refreshes, a sample to trust it, and fingerprints that turned main red (itos-cc#36, #37): the person chose speed and no upkeep, a weak test found by a later push or night instead, to revisit later. Survivor kills stay as decision 39 set them: a survivor of the whole suite gets a test or an exception. The person's call, 2026-10-11.

### Consequences

T-45 moves the proof and the nightly to counted runs and takes out the sample step, the records and the GOTOOLCHAIN line itos-cc#36 needed. The 50 survivors T-43 listed (docs/mutation-baseline.md) fail about half the nights until items kill them. Revisit when listed tests land, or if weak tests land too often.

## More Information

Supersedes ADR-0039.
