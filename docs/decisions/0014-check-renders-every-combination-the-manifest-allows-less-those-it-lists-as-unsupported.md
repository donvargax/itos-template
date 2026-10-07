---
status: accepted
date: 2026-10-07
---

# check renders every combination the manifest allows, less those it lists as unsupported

## Context and Problem Statement

Which combinations does itos-template check render, and which does new offer? Recommended: derived, every combination the stacks, features and needs allow, less those the manifest lists as unsupported, which check skips and new refuses. Or: listed explicitly, check rendering exactly the listed ones and new refusing any other (a fixed count, but a list to keep up to date).

Asked as q-14, about slice-2.

## Considered Options

- Derived, less those listed as unsupported
- Listed explicitly

## Decision Outcome

Derived, less those the manifest lists as unsupported. The user's call, 2026-10-07, knowing each feature that needs no other doubles a stack's combinations.

### Consequences

Manifest version 2 adds unsupported, a stack and its exact features; check skips such a combination and new refuses it with exit 1. Each feature that needs no other doubles a stack's combinations, each rendered and checked, so a template keeps its features few (PLAN.md's principles); the idea check-one-combination is the reminder for when a template's CI grows slow.
