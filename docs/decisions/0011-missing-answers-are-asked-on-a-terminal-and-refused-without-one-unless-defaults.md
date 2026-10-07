---
status: accepted
date: 2026-10-06
---

# Missing answers are asked on a terminal, and refused without one unless --defaults

## Context and Problem Statement

What does new do when an answer is not given with --answer? Recommended: on a terminal it asks, showing the default, and checks the answer against the question's pattern, asking again on a mismatch; without a terminal it refuses with exit 2 before writing anything, naming every missing answer at once, unless --defaults takes each missing one's default (a question with no default still refused). Or: without a terminal, take the defaults silently (convenient in scripts, but a project made with values nobody chose); or never ask, every answer by flag (simplest to test, but unfriendly for a person).

Asked as q-11, about new-renders.

## Considered Options

- Ask on a terminal; without one refuse, naming every missing answer, unless --defaults
- Take the defaults silently without a terminal; or never ask

## Decision Outcome

On a terminal new asks, showing the default, and checks the answer against the question's pattern, asking again on a mismatch; without a terminal it refuses with exit 2 before writing anything, naming every missing answer at once, unless --defaults takes each missing one's default, a question with no default still refused. The user's call, 2026-10-06.

### Consequences

--defaults follows docs/CLI.md: a --no-defaults pair and an environment variable.
