---
status: accepted
date: 2026-10-10
---

# update merges with git's own merge through its command line, the old render the base

## Context and Problem Statement

What does update's 3-way merge (decision 13: a library or our own code is the person's call)? Recommended: git's own merge through its command line: both renders written to the project's object database as unreferenced trees and merged against HEAD by ort, the old render the base, so renames are followed and conflicts take git's format; go-git stays out, merging only by fast-forward. Or: the old-to-new diff applied with git apply -3, as cruft and copier do (a rename the project already made can fail); or our own merge in Go, a diff3 library or our code.

Asked as q-49, about update-merges.

## Considered Options

- git's ort merge over the two renders as unreferenced trees
- The old-to-new diff applied with git apply -3, as cruft and copier do
- Our own merge in Go, a diff3 library or our code

## Decision Outcome

git's own merge through its command line, ort over the two renders written as unreferenced trees, the old render the base; tried on git 2.56, git merge-tree --write-tree --merge-base takes a tree and gives git's conflict stages. The person's call, 2026-10-10.

### Consequences

Renames follow as git follows them, and a conflict takes git's format, so the person resolves it with git's tools. Tried on git 2.56: git merge-tree --write-tree --merge-base accepts a tree, so no commit is invented for either render; slice-13 names the oldest git that has it. go-git stays out while it merges only by fast-forward.
