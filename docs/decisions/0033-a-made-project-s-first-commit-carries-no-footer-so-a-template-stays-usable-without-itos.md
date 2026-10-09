---
status: accepted
date: 2026-10-09
---

# A made project's first commit carries no footer, so a template stays usable without itos

## Context and Problem Statement

A made project's first commit carries no footer, not even a Task one. It is committed before the project has a ledger, so a footer names nothing that exists; and a template that must stay usable by someone who never adopts itos cannot have its first commit name an itos task. Recommended: no footer, and a made project that adopts itos gets its T-1 in the setup step that follows. Or: the manifest names a task the render will have, which means every render is an itos project from its first commit. Which?

Asked as q-42, about T-28.

## Considered Options

- No footer at all on the render's first commit
- The manifest names a task, so every render is an itos project from its first commit

## Decision Outcome

No footer. Two reasons and the second is the one that settles it for good. The render is committed before the project has any ledger, so a Task footer there names nothing that exists; and a template has to stay usable by someone who never adopts itos, so its first commit cannot name an itos task. A made project that does adopt itos gets its T-1 in the setup step that runs after the render, which is what ADR-0031 already decided, and that is why ADR-0032 leaves the footer out of the manifest. Keeping templates itos-optional is the reason, and it is worth having written down because the pull the other way is constant: a made project's own rules ask a chore for a Task, and the reflex is to add one back.

### Consequences

The render is committed before the made project has any ledger, so a footer there would name nothing that exists, and T-1 in a made project means the task that adopts itos, which arrives later. The deciding reason is wider: a template has to be usable by someone who never adopts itos, so its first commit cannot name an itos task, and that is what keeps itos optional rather than assumed. A made project that does adopt itos gets its T-1 in the setup step that runs after the render, per ADR-0031. This is written down because the pull is constant and looks reasonable: a made project's own itos rules ask a chore for a Task footer, and the reflex is to put one back in the manifest. Doing so would make every render an itos project from its first commit and quietly end itos-optional. The template's manifest names no task, and T-28's check holds it that way.
