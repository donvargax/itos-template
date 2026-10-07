---
status: accepted
date: 2026-10-07
---

# The domain is unit-tested with fakes and stubs, never mocks; the scenarios hold the rest against the real git

## Context and Problem Statement

What stands in for git and the files in tests, and is any integration test needed? Recommended: the domain unit-tested in full with our own fakes (working in-memory implementations of its ports) and stubs (fixed results, such as a merge conflict), never mocks, tests asserting outcomes and never calls; everything else against the real git through the scenarios; no contract or integration test until a trigger calls for one. Or: go-git with in-memory storage as a fuller stand-in for git, with a contract suite run against it and against the real git; or unit tests in every package.

Asked as q-19.

## Considered Options

- Our fakes and stubs in the domain, the real git elsewhere
- go-git in memory, with contract tests against both
- Unit tests in every package

## Decision Outcome

Our own fakes and stubs in the domain's unit tests, never mocks, asserting outcomes and never calls; the real git everywhere else, through the scenarios. go-git was set aside: it merges only by fast-forward, so it could not stand in for the merges rendering and update need. No contract test now: the scenarios run every real path against the real git on three systems in seconds, so a fake that drifts from git is caught there. The user's call, 2026-10-07.

### Consequences

Supersedes decision 18, keeping its rule that a test outside the domain exists only for what a scenario cannot reach (asking on a terminal, removing a folder on Windows), saying so. A fake is a working, lighter implementation of a port; a stub returns a fixed result; a mock, which scripts and checks calls, is never used, and depguard refuses the mocking libraries (task T-10). A fake shared by several packages' tests lives in its port's test-support package. A contract test (one suite per port, run against the fake and the real adapter) waits for a trigger: a fake growing behaviour of its own, such as merging trees or converting line endings; the first bug where a domain test passed and a scenario failed because a fake differed from git; or a port gaining a second real implementation. go-git is watched for a three-way merge (docs/ORCHESTRATING.md). No @real-git tag: no scenario runs against a stand-in for git.

## More Information

Supersedes ADR-0018.
