---
status: accepted
date: 2026-10-06
---

# A template is anything git clone takes, fetched by git

## Context and Problem Statement

How is a template named to new? Recommended: anything git clone takes, a local path included (git clones a path as it does a URL), fetched by git itself, so private templates use git's own credentials and none pass through itos-template; --ref picks a tag or a commit, by default the newest release, else the root branch's head. Or: a git URL plus a shorthand such as gh:owner/repo (shorter, but a host baked into the generator, against PLAN.md's standard-protocols principle); or local paths only to start (simplest, but the first real use is a template on GitHub).

Asked as q-9, about new-renders.

## Considered Options

- Anything git clone takes, fetched by git, --ref picking the release
- A git URL plus a host shorthand such as gh:owner/repo; or local paths only

## Decision Outcome

Anything git clone takes, a local path included, fetched by git itself, so private templates use git's own credentials and none pass through itos-template; --ref picks a tag or a commit, by default the newest release, else the root branch's head. The user's call, 2026-10-06.

### Consequences

itos-template never handles a credential to reach a template; no host is built into it.
