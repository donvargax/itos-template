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
- Windows is a platform job: what touches files, modes, line endings or paths needs a Windows
  thought (itos learned it: `go:embed` with CRLF checkouts, Unix file modes in tests).

## Lessons from this repository

None yet. Each lesson names its exit and its two dates; at most ten.
