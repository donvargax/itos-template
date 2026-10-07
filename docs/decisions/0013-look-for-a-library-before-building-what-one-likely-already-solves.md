---
status: accepted
date: 2026-10-06
---

# Look for a library before building what one likely already solves

## Context and Problem Statement

Before writing code for a problem a library likely already solves (merging, case conversion, YAML, terminal prompts, archives), do we look for one first, and how do we choose between using it and building it ourselves?

Asked as q-13.

## Considered Options

- Weigh an existing library first, by activity, complexity, likelihood of change and value
- Build by default, reaching for a library only when stuck

## Decision Outcome

Yes, for a problem a library likely already solves, not for all code: look for an existing library first and weigh it against building it. The weighing: how active it is (releases, maintainers, open issues answered), how complex it is beside how complex the problem is, how likely we are to need to change it, and the value it gives against what building and keeping our own costs. Safe to use means a licence the AGPL-3.0 can take, no known vulnerability (govulncheck), and pinned and checked as every dependency is. There is no right or wrong answer, so the choice is never an agent's alone: the coordinator weighs the candidates with the user, and the user and the coordinator review it together. It is settled while the item is specified, so it is in the spec before the agent starts; an agent that meets such a problem unforeseen stops and proposes the candidates rather than choosing. The choice and why go in the spec and in the commit body that adds the library or the code instead. The user's calls, 2026-10-06; the safety terms are the coordinator's reading of safe, the user free to change them.

### Consequences

AGENTS.md tells an implementing session to stop and propose, never choose; docs/ORCHESTRATING.md has the coordinator settle it with the user when specifying an item.
