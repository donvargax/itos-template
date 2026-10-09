---
status: accepted
date: 2026-10-09
---

# A template's itos setup lives on one branch beside its render root, and no render starts from it

## Context and Problem Statement

A template's own itos setup — its config, ledger and registry — belongs on one branch that no render starts from, so a made project gets its own from itos init and inherits none. Recommended: an itos branch beside the render root, the only place itos runs for that template, absorbing merges from every branch and consumed by nothing, kept honest by two checks (nothing but merges and itos-data commits on it; it holds the current head of every branch). Or: every branch carries the itos data, so each is self-gated and the data is duplicated and can drift. Or: the data stays on the render root and template_only keeps it out of renders, with a seed declared in the manifest for the made project. Which does a template follow, and is it a recommendation or a requirement?

Asked as q-38, about first-template.

## Considered Options

- One itos branch beside the render root, absorbing merges, consumed by nothing
- Every branch carries the itos data, self-gated but duplicated and able to drift
- The data stays on the render root, kept out of renders by template_only, with the made project's seed declared in the manifest

## Decision Outcome

One itos branch beside the render root, and it is the only place itos runs for that template. Its branch absorbs a merge from every other branch and is consumed by none, so the render root stays a floor and no itos data reaches a render. Two checks keep it honest: every commit on it since the last merge is that merge or touches only the itos data, and it holds the current head of every branch it absorbs. Every template we write follows this pattern as a recommendation, with its reason recorded, and nothing in new, check, update or sync may require it: a template author may lay its branches out however they like.

### Consequences

Renders never inherit itos data, so a made project runs itos init and gets its own config, hooks, ledger and registry, and the question of whose ledger a render holds has no answer to get wrong. The template's own data is edited in one place, and the gates that need code run against a merged copy on the itos branch, so no data is duplicated across branches and none can drift. Two checks make the branch safe: nothing on it but merges and itos-data commits, and it holding the current head of every branch it absorbs, so a fix made in the wrong place fails loudly and a stale branch cannot report green. The itos branch merges from everything and is read by nothing, which reads backwards and must be written down where an agent reads it. Renovate must name the branches it targets and CI must trigger on the itos branch, or dependency bumps stop silently. AGENTS.md exists in two forms, prose on the render root and prose plus the generated rules block in a render, generated in two places. A template author's branch layout is their own: nothing in new, check, update or sync may require this, and the record's branch names are a recommendation with its reason.
