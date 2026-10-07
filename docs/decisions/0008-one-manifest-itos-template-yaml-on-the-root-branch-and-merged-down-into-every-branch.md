---
status: accepted
date: 2026-10-06
---

# One manifest, itos-template.yaml, on the root branch and merged down into every branch

## Context and Problem Statement

Where does a template's manifest live, and in what format? Recommended: one YAML file, itos-template.yaml, at the top of the root branch, merged down into every stack and feature with the rest of the root, so each branch carries the same manifest and stays a real project. It lists the branches (each stack, each feature with its stack and the features it needs), the questions (each literal, its question, a default, a pattern its answer must match), and per branch its setup steps (decision 7) and later its checks (for check). It also lists the paths only the template keeps (its own CI that renders every combination, say); the manifest itself is always left out of a render. Or: a manifest per branch, each holding that branch's part, merged when a project is made (it reads close to the branch it describes, but the merge of manifests is a second merge to get right); or TOML or JSON instead of YAML (YAML is what itos and the CI already use).

Asked as q-8, about new-renders.

## Considered Options

- One itos-template.yaml on the root branch, merged down
- A manifest per branch, merged when a project is made; or TOML or JSON

## Decision Outcome

One YAML file, itos-template.yaml, at the top of the root branch, merged down into every stack and feature with the rest of the root, so each branch carries the same manifest and stays a real project. It lists the branches (each stack, each feature with its stack and the features it needs), the questions (each literal, its question, a default, a pattern its answer must match), per branch its setup steps (decision 7) and later its checks, and the paths only the template keeps; the manifest itself is always left out of a render. The user's call, 2026-10-06.

### Consequences

new-renders reads the manifest from the root branch's file; check, update, sync and new-setup extend the same file.
