---
status: accepted
date: 2026-10-07
---

# A template's own checks scan its renders for credentials; check scans for leftover literals

## Context and Problem Statement

How does a template's check find leaked credentials in its renders? Recommended: the template's own checks run gitleaks, pinned and checksummed as tools/bin/pinned pins golangci-lint, on every render, so itos-template takes no dependency and the check mechanism of slice-2 does the work; itos-template check builds the leftover-literal scan itself and warns when a template's checks name no credential scan. Or: gitleaks as a Go library inside check (always runs, a heavy dependency tree in the binary); or our own small set of patterns (light, far weaker than gitleaks' rules and harder to keep current).

Asked as q-21, about template-scans.

## Considered Options

- The template runs gitleaks as one of its checks
- gitleaks as a library inside check
- Our own patterns

## Decision Outcome

The template's own checks run gitleaks, pinned; check scans for leftover literals itself and warns when a template's checks name no credential scan. The user's call, 2026-10-07.

### Consequences

template-scans builds the leftover-literal scan and the warning, not a credential scanner; the first template's manifest runs gitleaks, pinned, as a check of every render. A template is usable from its branch heads before template-releases tags it (the user's call, 2026-10-07): first-template does not wait for releases. The queue puts template-scans and first-template after T-12 and before slice-3 and T-13.
