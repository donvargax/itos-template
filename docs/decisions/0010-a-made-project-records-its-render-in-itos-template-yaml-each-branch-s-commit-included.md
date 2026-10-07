---
status: accepted
date: 2026-10-06
---

# A made project records its render in .itos-template.yaml, each branch's commit included

## Context and Problem Statement

What does a made project record of its render, and where? Recommended: one committed file at its top, .itos-template.yaml, as copier's .copier-answers.yml: the template as it was named, the release it was made from when there was one, the stack, the features, the answers, and the commit each branch was rendered from. The commits make the old render reproducible for update even without tags, so new-renders needs no answer yet on how a release is tagged across branches. No answer is ever a credential (PLAN.md), so the file is safe to commit. Or: a folder, .itos-template/, holding the record and later state (room to grow, more to explain); or git notes on the first commit (nothing in the tree, but lost on a clone that fetches no notes).

Asked as q-10, about new-renders.

## Considered Options

- One committed .itos-template.yaml, each branch's commit included
- A .itos-template/ folder; or git notes on the first commit

## Decision Outcome

One committed file at the made project's top, .itos-template.yaml, as copier's .copier-answers.yml: the template as it was named, the release it was made from when there was one, the stack, the features, the answers, and the commit each branch was rendered from, so update can reproduce the old render even without tags. No answer is ever a credential (PLAN.md), so the file is safe to commit. The user's call, 2026-10-06.

### Consequences

How one release is tagged across a template's branches waits for update and sync: the recorded commits are enough to reproduce a render.
