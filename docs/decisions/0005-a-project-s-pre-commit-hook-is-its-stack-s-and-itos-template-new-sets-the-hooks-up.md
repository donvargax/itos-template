---
status: superseded by ADR-0007
date: 2026-10-06
---

# A project's pre-commit hook is its stack's, and itos-template new sets the hooks up

## Context and Problem Statement

How does a project made from the template get its pre-commit hook, beside itos's commit-msg and pre-push, without fighting the hook tool its stack's ecosystem already has (vite-plus's .vite-hooks, say)?

Asked as q-5.

## Considered Options

- A hook manager in every template: lefthook or pre-commit
- itos owns the pre-commit hook too, from a hooks.pre_commit key
- Each stack's own hook tool beside itos's git config hooks, set up by itos-template new

## Decision Outcome

The pre-commit hook is the stack's, in the stack's own tool: a vite-plus stack's is .vite-hooks with vp staged, which its package manager's install sets up; a Go stack, whose ecosystem has none, declares a POSIX sh script (tools/hooks/pre-commit) in the git config, as itos declares its own, git running it under Git for Windows' sh on Windows. itos's hooks stay in the git config beside it and itos owns no pre-commit (its decision 37); the root branch carries none. itos-template new sets the hooks up when it makes a project, so no project carries a setup tool of its own. Neither lefthook nor pre-commit: each is a second install step and a tool to pin, and pre-commit brings Python to every stack. The user's calls, 2026-10-06.

### Consequences

new-hooks makes new declare the hooks. A clone of a made project, not made by new, still declares them by hand (its README's lines) until a later decision says otherwise. This repository, the first template's stack/go, keeps tools/hooks/pre-commit as it is.
