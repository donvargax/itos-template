---
status: accepted
date: 2026-10-06
---

# Go and kong, an itos extension, its infrastructure harvested from itos

## Context and Problem Statement

What is itos-template built with and how is it started?

Asked as q-4.

## Considered Options

The options are those the question names.

## Decision Outcome

Go, kong for commands and flags (a --no- pair per switch, an environment variable per flag), an itos extension (itos template) that also runs alone, under the AGPL-3.0 as itos is, held to itos from its first commit. Its infrastructure (CI on three platforms, releases cut by CI, the godog harness, the CLI rules) is harvested from itos, not rewritten; its own skeleton, domain stripped, becomes the first template's stack/go and go/cli branches. CLI projects come first. The user's calls, 2026-10-06.

### Consequences

None recorded.
