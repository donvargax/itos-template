---
status: accepted
date: 2026-10-07
---

# Shell completion is our own: a hidden __complete command answering every shell from the command line's model

## Context and Problem Statement

How does a CLI made from the template complete on a shell? Recommended: our own, as cobra does it: a hidden __complete command that answers the shell from kong's model (its commands, flags and values), and a short stub script per shell (bash, zsh, fish, PowerShell) that asks the binary, so the scripts never go stale; no dependency. Or: jotaen/kong-completion (v0.0.14, April 2026, one maintainer) or willabides/kongplete (v0.4.0, 2023), both on posener/complete v1.2.3 of 2019 with loginshell and hashicorp/go-multierror, and neither completing on PowerShell, while Windows is one of the three systems every scenario runs on.

Asked as q-25.

## Considered Options

- Our own __complete, for four shells
- jotaen/kong-completion
- willabides/kongplete

## Decision Outcome

Our own __complete, from kong's model, for bash, zsh, fish and PowerShell. The user's call, 2026-10-08.

### Consequences

The idea shell-completion builds it here first, so the template carries it proven; docs/CLI.md gains its rule. No dependency is added.
