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
  here around it; one in itos-cc (github.com/donvargax/itos-cc, the code proof's mutation check)
  goes to itos-cc as an issue. The person wants both filed as they are found, after trying the
  tool to be sure. The idea upstream-fixes lists those open and what each changes here.
- The first template, github.com/donvargax/go-template-itos, tracks its own work: each of its
  branches (main, stack/go, go/cli) keeps its own config, ledger and registry (decision 32), and
  the person works it with itos there. Work on the template goes into that registry, on the
  branch whose code it touches; this registry holds the generator's work only, and an item here
  that waits on the template names the template's item in its why.

## What a brief adds here

- Reads: `AGENTS.md`, `PLAN.md`, the decision records the item rests on, and for bootstrap work
  the itos files it copies.
- The neighbours: the scenarios of the commands the item touches.
- Code a library may already solve: weigh the candidates with the person while specifying, by
  activity, complexity, likely change and value, and put the choice in the spec (decision 13).
  A library deferred until it brings value is watched: before specifying an item that would use
  it, raise it with the person again (charmbracelet/huh, for anything asking on a terminal:
  `new-picker`). go-git is watched on a condition: it merges only by fast-forward (v5.19.3,
  v6.0.0-beta.1), so the git command line stays; raise it again if it gains a three-way merge.
- Windows is a platform job: what touches files, modes, line endings or paths needs a Windows
  thought (itos learned it: `go:embed` with CRLF checkouts, Unix file modes in tests).

## Lessons from this repository

Each lesson names its exit and its two dates; at most ten.

- **A push never ends on a registry-only commit** (2026-10-07). `itos work done` passes over
  them and waits for the run of the commit below, which GitHub never makes: it runs a push's head
  alone (itos#20). Briefs say to add ideas before the last commit that touches more, and the
  coordinator closes an item only once a push ending on such a commit has run. Exit: itos#20
  fixed and pinned.
- **A task is pushed once `itos task <id>` passes** (2026-10-07). CI runs the static checks of
  every task a push's commits name, while the commit hook only prints them before the item is
  done: T-9's first push, its build commit not yet made, turned main red (run 37668940941) until
  its second. Briefs say to push a task's commits together once its checks pass. Exit: CI judges
  a task's checks as the hook does.
- **The coordinator's pushes go with `itos push --no-wait`** (2026-10-07). Its docs, ledger and
  registry commits never block the conversation on CI; the person found minutes lost to waits on
  GitHub's API. Implementing agents push their own code and wait on their own run; the
  coordinator checks main's run in the background only when its push could turn it red. Exit:
  itos push stops waiting for a docs-only range itself.
- **No commit body line starts `with #,`** (2026-10-08, seen 2026-10-08). itos's lint then warns
  that the footers have no blank line before them, though they do (bug-3's c6cd810), and agents
  reword correct bodies around it. Briefs say so. Exit: itos#21 fixed and pinned.
- **A throwaway repository is committed with `git -C <dir> commit`** (2026-10-09, seen
  2026-10-09). The itos plugin's guard judges `cd <dir> && git commit` by the session's folder,
  this repository, and denies it, so new-steps' agent fell back to `git commit-tree`. The guard
  follows `-C`, and a folder itos does not manage gets no answer. Briefs say so. Exit: itos#34
  fixed and pinned.
- **An invisible character in a test is written as an escape, never as itself** (2026-10-09,
  seen 2026-10-09). setup-invisible's agent found its edits had put literal U+FEFF and kin into a
  Go raw string, caught only because Go refuses a BOM there; a U+202E would have compiled
  unseen. Briefs touching such characters say to write `\uXXXX` in an interpreted string and to
  scan the diff for them before committing. Exit: source-invisible-gate.
