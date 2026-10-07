---
status: accepted
date: 2026-10-06
---

# A clone of a made project sets its hooks up with itos template hooks

## Context and Problem Statement

new sets up the hooks of the project it makes, but a later clone of that project (another person, a new machine) was never made by new: how does it get its hooks? Recommended: one itos-template command any clone runs (itos template hooks, say), declaring itos's hooks and the stack's from what new recorded, the same code new runs. Or: the README's lines by hand, CI holding the checks for a clone that skips them.

Asked as q-6, about new-hooks.

## Considered Options

- itos template hooks, the code new runs, declaring only the hooks the stack's manifest names
- The README's git config lines, by hand

## Decision Outcome

itos template hooks, which a clone runs, declares only the hooks its stack's manifest names, for a stack whose ecosystem has no hook tool of its own: in practice the Go stack's tools/hooks/pre-commit. It never declares itos's hooks, which itos hook install does. A stack with its own mechanism (vite-plus's .vite-hooks, set up by its package manager's install) names none, and the command does nothing there. It is the code new runs to declare the same hooks; simple to add and to maintain. The README's lines by hand are not the way. The user's calls, 2026-10-06.

### Consequences

new-hooks builds itos template hooks beside new's declaring them, and the manifest's way for a stack to name its hooks. Decision 5 holds but for its consequence that a clone declares its hooks by hand. The generator stays stack-agnostic: it declares what a manifest names and knows no stack's hook tool.

## More Information

Amends ADR-0005: its consequence that a clone declares its hooks by hand; the rest of ADR-0005 stands.
