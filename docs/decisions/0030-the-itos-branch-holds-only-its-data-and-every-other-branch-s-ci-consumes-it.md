---
status: superseded by ADR-0032
date: 2026-10-09
---

# The itos branch holds only its data, and every other branch's CI consumes it

## Context and Problem Statement

The itos branch holds only the itos data, and every other branch's CI consumes it, with no copy of the code on it and no merges into it. A run checks out the branch being gated as the working tree and the itos data beside it, points ITOS_CONFIG at that data, and runs the full plan. With no copy, the check that guarded against a fix made in the wrong place has nothing to guard; with every push running the gates against its own branch, the check that itos holds the current head of every branch is unnecessary too, so staleness is impossible by construction. Or: keep a merged copy on itos, which needs both checks. Or: keep the data on each branch and accept the duplication ADR-0029 rejected. Which does a template use?

Asked as q-39, about first-template.

## Considered Options

- Data only: the other branches' CI checks out the data and runs the full plan
- A merged copy of the whole template on itos, needing both of ADR-0029's checks
- The data on every branch, duplicated

## Decision Outcome

Data only, consumed by the other branches, no copy and no merges into it. A run checks out the branch being gated as the working tree and the itos data beside it, points ITOS_CONFIG at the data, and runs the full plan, so every gate that needs code and every gate that judges commits run in one place from one copy of the data. Neither of the two checks ADR-0029 asked for is needed: with no copy there is no wrong place to make a fix in, and with every push to any branch running the gates against that branch, no branch can be stale. The itos branch keeps the config, the ledger and registry, and the generated rules block; the code branches keep the code, the scenarios, the mutation records and plain CI that needs no config. Every template follows this as a recommendation, and nothing in new, check, update or sync may require it.

### Consequences

A made project inherits no itos data and a template's data is edited in one place, and neither copy can drift. The code gates and the commit gates meet in a single run over one branch's own head, so the whole rule set is proved there without any code being duplicated. Neither check ADR-0029 asked for is needed: with no copy there is no wrong place to fix, and with every push to any branch running the gates against that branch a branch cannot report green for code nobody renders, so staleness is impossible rather than detected. The itos branch stops being a branch you can read code from, which is the cost, and the two-checkout wiring is small non-obvious YAML someone has to understand later. A change to the data gates every branch, so a push to the itos branch runs the matrix over the others. The data branch holds the config, the ledger and registry and the generated rules block; the code branches hold the code, the scenarios, the mutation records and plain CI needing no config. A template author's layout is their own: nothing in new, check, update or sync requires this.

## More Information

Supersedes ADR-0029.
