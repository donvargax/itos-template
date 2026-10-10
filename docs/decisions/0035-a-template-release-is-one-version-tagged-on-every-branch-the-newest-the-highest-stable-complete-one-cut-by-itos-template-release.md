---
status: superseded by ADR-0036
date: 2026-10-09
---

# A template release is one version tagged on every branch, the newest the highest stable complete one, cut by itos-template release

## Context and Problem Statement

How is one template release tagged across its branches, which release is the newest, and who cuts one? A git tag names one commit, so one name cannot sit on main, stack/go and go/cli at once; decision 9 has --ref pick a tag, by default the newest release, and decision 10 left the tagging for later. Recommended: a tag per branch under one version, v1.2.0 on the root branch and <branch>/v1.2.0 on each other; the newest is the highest stable complete vX.Y.Z; a command, itos-template release, cuts one once check passes. Or: one tag on the root naming the other commits in a file, or a ref namespace of ours; the newest by tag date; the template's own CI or people by hand cutting it. Which?

Asked as q-44, about template-releases.

## Considered Options

- A tag per branch under one version (v1.2.0 on the root, <branch>/v1.2.0 on the others)
- One tag on the root branch, a file in its commit naming the other branches' commits
- A ref namespace of ours, refs/itos-template/releases/<version>/<branch>

## Decision Outcome

A tag per branch under one version: v1.2.0 on the root branch and <branch>/v1.2.0 on each other branch the manifest lists (stack/go/v1.2.0, go/cli/v1.2.0), plain git tags any clone fetches; a release is complete when every listed branch has its tag, and an incomplete one is refused. The newest is the highest stable vX.Y.Z that is complete, a pre-release taken only when --ref names it, and a template with no release renders its branch heads as decision 21 allows, saying so. A command, itos-template release, cuts a release, tagging every branch only once check passes over them.

### Consequences

Releases are plain git tags, so any clone fetches them and GitHub shows them, and a template can be tagged by hand in a pinch, though itos-template release is the way: it runs check over the branch heads and tags them only if every combination passes, so a release is a proved one. new and check read a release as complete only when every branch the manifest lists carries its tag, and refuse an incomplete one named by --ref; a default skips it. The newest is the highest semver, so a backport cut after a newer release never becomes the default, and a pre-release is never a default. A template with no release still renders its branch heads (decision 21), saying so, so templates keep working before their first release. The record names the release beside the exact branch commits, which stay what reproduces a render (decision 10). Branch names hold slashes, so a feature's tag reads go/cli/v1.2.0; a version is the tag's last path segment.
