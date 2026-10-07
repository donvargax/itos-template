---
status: accepted
date: 2026-10-07
---

# A template lives in its own repository, and the project it came from adopts it and takes its updates

## Context and Problem Statement

Where does the first template live, and how does itos-template prove it? Recommended: the template in its own repository (a root, stack/go, go/cli: this repository's machinery with minimal code that sets its rules), which itos-template adopts and then takes updates from with itos-template update, each landing as one ordinary commit on main through its gates, so itos and itos-cc need nothing new; the end goal recorded: the template's branches beside main in this repository, main descending from go/cli, which needs branch-aware rules and sanctioned merges in itos, per-branch baselines in itos-cc, and a named root and sync into main in itos-template. Or: this repository as is (a render would copy all of itos-template); or the end goal now.

Asked as q-24.

## Considered Options

- Its own repository, adopted and updated
- This repository as is
- The template branches beside main now

## Decision Outcome

The template in its own repository, adopted here and kept current by update; the same-repository shape is the end goal, its cross-project needs written down for itos and itos-cc to weigh. The user's calls, 2026-10-07: a template is a real project's stripped base, all its machinery kept and its code minimal but setting its rules, for every template the owner makes.

### Consequences

Supersedes the part of decision 4 that left where the template lives to first-template. first-template makes the template repository; template-releases, update-merges and adopt follow, and this repository adopts it. Template-first: an improvement to the machinery goes into the template, is released, and reaches this repository by update. The end goal is the idea template-beside-project, with an itos follow-up for branch-aware rules and merges.
