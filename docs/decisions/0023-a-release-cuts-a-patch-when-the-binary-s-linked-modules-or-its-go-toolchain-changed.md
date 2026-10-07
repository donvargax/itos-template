---
status: accepted
date: 2026-10-07
---

# A release cuts a patch when the binary's linked modules or its Go toolchain changed

## Context and Problem Statement

Does a dependency update cut a release? Recommended: a patch when, since the last release, the modules the binary links or the Go toolchain it is built with changed; never for a test-only module, a tool of go.mod's tool block, a go.sum-only change, an unlinked indirect module or a workflow's action, which leave the binary as it was. Or: a patch for any build commit touching go.mod or go.sum (simpler, but a tool's update releases an identical binary); or no release until the next feat or fix (a security fix waits).

Asked as q-23.

## Considered Options

- Linked modules or toolchain changed
- Any build commit touching go.mod or go.sum
- No release until a feat or fix

## Decision Outcome

A patch when the binary's linked modules or its Go toolchain changed, nothing otherwise. The toolchain counts because the binary links the standard library, whose patch releases carry security fixes. The user's call, 2026-10-07.

### Consequences

tools/bin/release-version adds the rule after its others (task T-15). Renovate's Go updates (T-14) stay build commits; the release they cause carries no Upgrading steps, its notes listing what moved from the commit bodies.
