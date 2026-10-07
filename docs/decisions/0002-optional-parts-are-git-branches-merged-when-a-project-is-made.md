---
status: accepted
date: 2026-10-06
---

# Optional parts are git branches merged when a project is made

## Context and Problem Statement

How are the optional parts of a template (a stack, a feature, a provider) chosen when a project is made?

Asked as q-2.

## Considered Options

The options are those the question names.

## Decision Outcome

As git branches, never as conditionals in the files: a root branch with what every stack shares, a branch per stack off it, a branch per feature off its stack, a feature needing another branched off that one (TopGit's topic branches are the prior art). Rendering merges the stack and the chosen features, then substitutes the literals last. The root is merged down, never rebased; a template release is a tag. Providers are chosen this way at generation time, never by runtime indirection. The user's calls, 2026-10-06.

### Consequences

None recorded.
