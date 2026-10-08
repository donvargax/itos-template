---
status: accepted
date: 2026-10-07
---

# Code that differs by OS takes the OS as a value; build-tagged files only for a call one OS lacks

## Context and Problem Statement

How does code that differs by OS look, now that T-20's code proof refuses a mutant no test runs, and a branch only Windows takes never runs on the Linux machine closing an item? Recommended: knowledge that differs by OS (file names, extensions, separators, exec bits) is a function taking the OS and what it reads (PATHEXT) as values, so unit tests run every branch on any machine, and only a thin caller reads runtime.GOOS; build-tagged files only for a call one OS lacks (syscall.Exec), as thin as can be; no runtime.GOOS branch inside logic. Or: build-tagged files for every difference, tested only on their OS; or runtime.GOOS branches as now.

Asked as q-26, about T-20.

## Considered Options

- The OS as a value, tagged files only for missing calls
- Build-tagged files for every difference
- runtime.GOOS branches in the logic

## Decision Outcome

Knowledge that differs by OS is passed as values, build-tagged files only for a call one OS lacks, no runtime.GOOS branch in logic: for now. If Windows-only work brings too many such parameters, refactor toward tagged files. The user's call, 2026-10-08.

### Consequences

Unit tests reach every OS's branch on any machine, so the code proof (T-20) never meets a line only another OS runs; the scenarios on the three systems still prove the real behaviour. internal/git's names() and executable() read runtime.GOOS inside their logic and move to it when an item next touches them. Revisited if Windows-only work makes the parameters many.
