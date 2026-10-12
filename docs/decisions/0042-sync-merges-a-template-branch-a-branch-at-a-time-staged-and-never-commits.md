---
status: accepted
date: 2026-10-11
---

# sync merges a template branch a branch at a time, staged, and never commits

## Context and Problem Statement

How does sync make each merge? Recommended: run on a template branch, it merges into it the branch it descends from (the root into a stack, a stack into its feature, a needed feature into the one needing it) and leaves the merge staged, or conflicted, for the person to commit with their own tools and gates, as update does (decision 37); the person walks the tree down a branch at a time. Or: sync walks the whole tree, committing each clean merge with a message it writes and stopping at the first conflict. Or: sync only reports which branches are behind.

Asked as q-57, about slice-19.

## Considered Options

- Staged, a branch at a time: sync merges into the branch checked out the branch it descends from and leaves the merge for the person to commit
- sync walks the whole tree, committing each clean merge with a message it writes and stopping at the first conflict
- sync only reports which branches are behind

## Decision Outcome

Staged, a branch at a time: sync merges into the checked-out template branch the branch it descends from and leaves the merge staged or conflicted for the person to commit through their own gates, as update does (decision 37). The person's call, 2026-10-11.

### Consequences

A template's merges keep passing through its own gates with whatever message and footers its rules ask, as update's do (decision 37), since each branch of a template can hold its own itos config (decision 32) whose commit rules sync cannot know. The person walks the tree down, a branch at a time, so sync refuses a branch whose parent itself lags. sync uses git merge --no-commit --no-ff through git's command line, as decision 38 uses git's own merge. Paths each branch keeps its own get no special treatment (q-59): a conflict the person resolves. sync --status reports the whole tree (q-58).

## More Information

slice-21 (q-60) moved both commands into the noun group branch, as docs/CLI.md rule 18 asks: the merge is branch sync and the report, once sync --status, is branch list. The old sync exits 2 naming them (rule 41).
