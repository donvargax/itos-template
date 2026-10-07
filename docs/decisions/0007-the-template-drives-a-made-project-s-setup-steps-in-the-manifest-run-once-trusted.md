---
status: accepted
date: 2026-10-06
---

# The template drives a made project's setup: steps in the manifest, run once trusted

## Context and Problem Statement

Should the generator know itos and the stacks' hook tools (decisions 5 and 6), or should the template drive what a made project needs set up?

Asked as q-7, about new-setup.

## Considered Options

- The generator declares itos's hooks and the stack's own (decisions 5 and 6)
- The manifest lists setup steps per branch; new and itos template setup run them once trusted

## Decision Outcome

The template drives it; the generator knows no itos and no stack's hook tool. The manifest lists setup steps per branch: the root of a template using itos lists itos hook install, stack/go its git config lines for tools/hooks/pre-commit, a vite-plus stack none (its package install sets its hooks up), a template without itos nothing of it. new runs the chosen branches' steps after rendering; itos template setup, the same code, runs them in a clone new did not make. Steps are a template's code, so, as copier's --trust, new shows them and asks on a terminal, or runs them under --trust, never silently. itos-template stays an itos extension (decision 4): that is how it is installed and called, not what it knows. The user's calls, 2026-10-06.

### Consequences

new-setup builds the steps, new running them and itos template setup. Decisions 5 and 6 are superseded: no generator code names itos, Go or vite-plus. What a stack's pre-commit hook is stays its own (a vite-plus stack's .vite-hooks, the Go stack's sh script in the git config), now as the stack's setup steps. This repository, the first template's root and stack/go, keeps its README's lines until new-setup lands.

## More Information

Supersedes ADR-0006.
