---
status: accepted
date: 2026-10-06
---

# new writes into a missing or empty folder and commits the render as its first commit

## Context and Problem Statement

Where does new write the project? Recommended: into a folder that does not exist or is empty, which it makes a git repository with the render as its first commit, so update has a base to merge onto from day one; a folder with files in it is refused, exit 1, naming it. Or: render into any folder and leave committing to the person (copier's way; works over an existing project, but update then has no known base); or render without git and let setup steps init it (decision 7; leaves the most to each template).

Asked as q-12, about new-renders.

## Considered Options

- A missing or empty folder, made a repository whose first commit is the render
- Any folder, committing left to the person; or no git, setup steps running git init

## Decision Outcome

Into a folder that does not exist or is empty, which new makes a git repository with the render as its first commit, so update has a base to merge onto from day one; a folder with files in it is refused, exit 1, naming it. The user's call, 2026-10-06.

### Consequences

Running setup steps (decision 7) comes after that first commit, so what they change is the project's own next commit.
