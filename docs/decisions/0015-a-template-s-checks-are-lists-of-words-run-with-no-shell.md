---
status: accepted
date: 2026-10-07
---

# A template's checks are lists of words run with no shell

## Context and Problem Statement

How does the manifest write a check command? Recommended: a list of words run with no shell, the same on Linux, macOS and Windows, a template wanting a shell writing sh -c itself. Or: a shell line run by sh -c (cmd on Windows), easier to write but meaning different things per platform.

Asked as q-15, about slice-2.

## Considered Options

- A list of words, no shell
- A shell line

## Decision Outcome

A list of words run with no shell. The user's call, 2026-10-07.

### Consequences

Manifest version 2 adds checks on the top, on each stack and on each feature, run with no shell; a check written as one string is refused. Their words have the literals replaced as file contents are. A later setup key (decision 7) is likely written the same way.
