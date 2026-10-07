# This repository's own notes

`itos go` prints the generic coordinator's guide, then this file, then `itos status`. This file
holds only what is true of this repository: what to read beyond the status, and the lessons it
teaches, in the guide's lesson format.

## Start of a session

- `PLAN.md`'s "Order of work" is the queue's backbone; the registry holds the items.
- itos itself lives at `~/tmp/llms-safe/gamedev/svg-art/itos` (github.com/donvargax/itos). This
  repository harvests its infrastructure (workflows, the godog harness, release tooling, the CLI
  rules) rather than rewriting it; a brief that copies from it names the files.
- A gap in itos found here goes to itos as an issue through its consumer-report form, never fixed
  here around it.

## What a brief adds here

- Reads: `AGENTS.md`, `PLAN.md`, the decision records the item rests on, and for bootstrap work
  the itos files it copies.
- The neighbours: the scenarios of the commands the item touches.
- Code a library may already solve: weigh the candidates with the person while specifying, by
  activity, complexity, likely change and value, and put the choice in the spec (decision 13).
  A library deferred until it brings value is watched: before specifying an item that would use
  it, raise it with the person again (charmbracelet/huh, for anything asking on a terminal:
  `new-picker`).
- Windows is a platform job: what touches files, modes, line endings or paths needs a Windows
  thought (itos learned it: `go:embed` with CRLF checkouts, Unix file modes in tests).

## Lessons from this repository

Each lesson names its exit and its two dates; at most ten.

- **A push never ends on a registry-only commit** (2026-10-07). `itos work done` passes over
  them and waits for the run of the commit below, which GitHub never makes: it runs a push's head
  alone (itos#20). Briefs say to add ideas before the last commit that touches more, and the
  coordinator closes an item only once a push ending on such a commit has run. Exit: itos#20
  fixed and pinned.
