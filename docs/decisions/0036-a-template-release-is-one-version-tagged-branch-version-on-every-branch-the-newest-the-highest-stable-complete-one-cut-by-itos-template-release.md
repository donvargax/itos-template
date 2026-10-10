---
status: accepted
date: 2026-10-09
---

# A template release is one version tagged <branch>/<version> on every branch, the newest the highest stable complete one, cut by itos-template release

## Context and Problem Statement

Decision 35 tags a template release as a bare v1.2.0 on the root branch and <branch>/v1.2.0 on the others. A bare vX.Y.Z is how a project tags its own releases, so a template whose branches live beside its project (template-beside-project) would have the project's v0.20.0 read as a template release. Recommended: every branch, the root included, carries <branch>/<version> (main/v1.2.0). Or: a fixed prefix on the root (template/v1.2.0); or keep the bare tag. Which?

Asked as q-45, about releases.

## Considered Options

- Every branch <branch>/<version>, the root included (main/v1.2.0)
- A fixed prefix on the root (template/v1.2.0), <branch>/<version> on the others
- A bare v1.2.0 on the root, as decision 35 had it

## Decision Outcome

Every branch the manifest lists, the root included, carries <branch>/<version>: main/v1.2.0, stack/go/v1.2.0, go/cli/v1.2.0. One rule for every branch, and no template tag is ever a bare vX.Y.Z a project tags its own releases with.

### Consequences

It keeps all of decision 35 but the root's tag. A release is plain git tags, one per branch the manifest lists, each <branch>/<version> with the root's own name too, so any clone fetches them, GitHub shows them, and a project's own vX.Y.Z never reads as a template release, which matters once a template lives beside its project. A release is complete when every listed branch carries its tag; new and check refuse an incomplete one named by --ref, and a default skips it. The newest is the highest stable semver that is complete, a pre-release never a default, so a backport cut after a newer release never wins. new renders the newest release by default and a template with no release its branch heads, saying so (decision 21); check proves the branch heads by default, what itos-template release will tag, and a release when --ref names one. itos-template release cuts one, tagging every branch only once check passes over them. The record names the release beside the exact branch commits, which stay what reproduces a render (decision 10). A branch name holds slashes, so the version is the tag's last path segment.

## More Information

Supersedes ADR-0035.
