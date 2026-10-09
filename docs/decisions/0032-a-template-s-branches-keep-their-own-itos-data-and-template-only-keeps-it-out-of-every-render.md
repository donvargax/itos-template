---
status: accepted
date: 2026-10-09
---

# A template's branches keep their own itos data, and template_only keeps it out of every render

## Context and Problem Statement

Where does a template keep its itos setup, so that no render inherits it? Recommended: nothing changes on the template's branches at all; each already keeps its own config, ledger and registry, scoped to the work that happened on it, and the manifest's template_only keeps those paths out of every render, with the made project getting its own setup from the template's declared setup steps. Or: a branch beside the render root holding a merge of the branches, with a check that nothing but merges lands on it. Or: a branch holding only the itos data, with the other branches' CI consuming it. Or: the data on every branch with no exclusion. Which?

Asked as q-41, about T-28.

## Considered Options

- Nothing changes; template_only keeps each branch's own data out of renders
- A branch beside the render root holding a merge of the branches, with a commit-shape check
- A branch holding only the itos data, with the other branches' CI consuming it
- The data on every branch with no exclusion at all

## Decision Outcome

Nothing changes on the template's branches. Each already keeps its own config, ledger and registry, scoped to the work that happened on it: main holds T-1, stack/go adds T-2, go/cli adds T-3 and the count-cli slice, and no branch holds another's work. The manifest's template_only keeps those paths out of every render, and the made project gets its own setup from the template's declared setup steps, itos init --agent-rules, per ADR-0031. No branch is added, nothing merges into anything, and no CI plan moves.

### Consequences

The whole problem was one of exclusion, not of placement. Each branch already keeps its own config, ledger and registry, scoped to the work that happened on it, so there is nothing duplicated that ought to be one thing: main holds T-1, stack/go adds T-2, go/cli adds T-3 and the count-cli slice, and the root's shared tasks and the manifest are merged down one way as ADR-0008 already decides for the manifest. So no branch is added, nothing merges up into anything, and no CI plan moves: every branch keeps the gates, the plan, the proof and the task checks it has today, which the two records this supersedes would each have taken away. A render receives none of it, and the made project gets its own from the template's declared setup steps per ADR-0031. What this template changes is two lines of its manifest. The three-branch shape was verified rather than assumed: three different itos.yaml, three different ledgers, three different registries, and main's own plan is three static steps with no toolchain. The earlier shapes failed for concrete reasons worth keeping: one config cannot plan branches with different contents, and ITOS_CONFIG reads the config file from wherever it points but resolves the data against the working tree, so it half-works and then fails; both are reported upstream. A record's subject still may never have been built, which is the gap this very decision came through.

## More Information

Supersedes ADR-0030.
