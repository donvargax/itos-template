---
status: superseded by ADR-0019
date: 2026-10-07
---

# The domain is unit-tested in full; the scenarios hold the rest

## Context and Problem Statement

How is the code tested across its layers? Recommended: the domain unit-tested in full, its ports faked; the UI, the application and infra held by the scenarios (godog against the built binary, on Linux, macOS and Windows); a test outside the domain only for what a scenario cannot reach (asking on a terminal, removing a folder on Windows), its comment saying which. No integration-test layer. Or: unit tests in every package; or an integration layer for infra against the real git.

Asked as q-18.

## Considered Options

- Domain unit tests and the scenarios
- Unit tests in every package
- An integration layer for infra

## Decision Outcome

The domain unit-tested in full with fakes of its ports; everything else held by the scenarios; a test outside the domain only where a scenario cannot reach, saying so. No integration-test layer: the scenarios already run the real git on three systems. The user's call, 2026-10-07.

### Consequences

T-9 moves the tests with the code: what tests the domain stays and grows to cover it through fakes of its ports; a test of a slice, of the UI or of infra stays only where a scenario cannot reach, its comment saying why. A gate on the domain's coverage waits for the idea domain-coverage, its number taken from the code after T-9.
