---
status: accepted
date: 2026-10-07
---

# The domain gets property-based tests with rapid: one case per push, many nightly

## Context and Problem Statement

Does the domain get property-based tests, and with what? Recommended: pgregory.net/rapid (v1.3.0, March 2026, modelled on Hypothesis, no dependencies of its own, in tests only): properties of the domain's functions (round-trips, determinism, invariants of derived values) with generators and shrinking, and stateful (model-based) tests through t.Repeat for operations that run in sequence, such as update and sync; the count of random cases per property set by RAPID_CHECKS, low on every push and high in a nightly run; rapid.MakeFuzz pointing Go's fuzzer at the parsers. Or: Go's built-in fuzzing alone (basic types only, no structured inputs); gopter (last released April 2024); testing/quick (frozen, no shrinking).

Asked as q-20.

## Considered Options

- rapid
- Go's built-in fuzzing alone
- gopter
- testing/quick

## Decision Outcome

rapid, for properties of inputs and for stateful tests of the system. One random case per property on every push, so the property tests stay as fast as unit tests while each push tries a new case; many in the nightly run. The user's call, 2026-10-07.

### Consequences

Properties are unit tests of the domain (decision 19), in its packages' tests, never outside it. Every push runs them with RAPID_CHECKS=1 (the CI plan and the platform jobs set it), a failure printing the seed and writing the file that replays it; the nightly run sets a high count. Input properties now (task T-12): case forms, determinism of a render, line endings and binary files untouched, derived combinations, Parse never panicking, a Record's round-trip. Stateful tests through t.Repeat when update and sync are specified: sequences of template releases, edits to the project and updates, their invariants checked after every step. rapid.MakeFuzz points Go's fuzzer at the parsers in the nightly run.
