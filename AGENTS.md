# Working rules

Read `PLAN.md` first: what itos-template is, its model and the order of work. The decisions behind
it are records in `docs/decisions/`; what a made project contains is `docs/template-contents.md`; the
command line follows `docs/CLI.md`.

This repository is held to its rules by itos (`itos.yaml`, the block at the end of this file): the
hooks and CI enforce every rule a command can decide, on every commit, whoever made it. This file
holds what no command can check. Where a rule has a gate, this file names the gate and does not
restate it. Keep it short: every session reads it.

**Which session are you?** If you were handed an item to implement, you are an implementing
session: run `itos guide work`, then follow this file; do the work yourself and start no
subagents. If you are the session the person talks to, you coordinate: `itos go` is yours (the
generic guide, then `docs/ORCHESTRATING.md`, then `itos status`). Anything that needs a scenario
is a slice, and a slice goes to an implementing agent.

**Who works what.** `tasks/work-items.yaml` says who owns each item and what it waits on; `itos
work` shows what you can start. Take an item with `itos work take <id>` and push that first; close
it with `itos work done <id>` once its work is pushed and CI is green. Never edit an owner or a
status by hand.

## What drives a change

- **`feat` / `fix`:** scenarios in `features/`. Remove `@wip` from the ones you implement (or add a
  `@bug-<n>` scenario for a fix), implement until they pass, commit with `--scenarios` and
  `--upgrading` (what a user of itos-template must change, or `none`). If the behaviour you need
  is not described, stop and propose the scenario; never bend a scenario to fit the code.
- **Everything else:** a task in `tasks/`, named with `--task`. `itos task <id>` says when it is
  done.
- **Red first:** the steps a slice's scenarios need go alone in a `test` commit, the scenarios
  still `@wip`; run them and see each fail at the step that checks the behaviour; then build.
- **Conventional Commits,** committed with `itos commit -F <file>`, never `git commit`. The body
  says what changed and why: it is the changelog. No body line starts with a word and a colon
  (git reads it as a footer).
- **A library or our own code** for a problem one likely already solves is the person's call
  with the coordinator (decision 13), made in the spec. Meeting one the spec did not settle,
  stop and propose the candidates, weighed by activity, complexity, change and value; never
  choose alone.
- **Vertical slices, a thin UI and app, the work in the domain** (decision 17; depguard and
  gochecksumtype hold the arrows): commands carry domain objects, and the domain stays free of UI
  concerns, exit codes included. No other mapper: never a DTO or an options struct copying a
  type. An anti-corruption layer only where an outside model would leak in. Unit tests are for
  the domain, with our fakes and stubs, never mocks, asserting outcomes; the scenarios hold the
  rest against the real git (decision 19).
- **Decide the split before editing:** each commit type may touch only certain paths (the block
  below). Check a split with `itos commit check-paths --type <type> <path>…`; never relabel a
  commit to get past a rule.

## The gates run themselves

Every commit and push runs the checks for you. Don't run them by hand first; commit, and read what
the gate says. When one fails, fix that cause. Never sit blocked on a push: run `itos push` in the
background and watch its output with a Monitor, never a wait loop of your own.

## Never

- Bypass a hook (`--no-verify`, `-n`, `-c core.hooksPath`), force-push, or rewrite what is on the
  remote.
- Pipe `itos commit` or `itos push` into anything; write the output to a file and read it.
- Stage with `git add -A`, `.`, `-u` or a directory: stage your own files by path, and read `git
  status --short` before committing.
- Weaken a gate to get green. If the gate is wrong, stop and say so.
- Use a template language. Templates here are real projects; see `PLAN.md`.

## Finishing

An item whose commits touch `cmd` or `internal` closes only on its code proof (itos.yaml's
`proof`). Before its last push, run `tools/bin/pinned itos-cc mutation run --since <the parent of
the item's first commit> --fail-uncovered --all-tests --no-annotate`, commit `.metrics/mutate/`,
and kill each survivor with a test; a truly equivalent one is excepted (`itos-cc mutation except`)
with its reason, for the person to review. An uncovered mutant needs a test, never an exception.
Code that differs by OS takes the OS as a value (decision 26): no `runtime.GOOS` branch in logic,
and a build-tagged file only for a call one OS lacks, with no mutation site in it.

Push with `itos push` (it rebases onto `main`, pushes, and waits for CI). Done is that run green;
report its URL. A gap the work leaves goes into the registry as an idea (`itos work add`), never
into a commit body. A reason worth keeping goes where a reader looks: a scenario's above it, a
task's in its why, a decision in `docs/decisions/`, how the code is put together in a package's
doc comment.

<!-- itos:begin -->

## The rules itos holds this repository to

Generated by itos init from `itos.yaml`, which decides them: change the config, then run `itos init --agent-rules`, which rewrites only what is between these markers. The hooks and CI hold every commit to these rules.

### Commit types

A commit's type is one of `feat`, `fix`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `docs`, `style` or `revert`. The header is held to commitlint's config-conventional rules by itos's own lint: `type: subject` or `type(scope): subject`.

### Footers

- `Task:` names tasks of the ledger, `tasks/phase-{group}.yaml`.
- `Scenarios:` names `scenario` tests by their IDs, live ones only (not `@wip`).
- `Item:` names items of the work registry, `tasks/work-items.yaml`.
- `Upgrading:` is free text.

The footers each type needs:

- `feat`: `Scenarios:` and `Upgrading:`.
- `fix`: `Scenarios:` and `Upgrading:`.
- `refactor`: `Task:`.
- `perf`: `Task:`.
- `test`: `Task:` or `Item:`.
- `build`: `Task:`.
- `ci`: `Task:`.
- `chore`: `Task:` or `Item:`.
- `docs`: none.
- `style`: `Task:`.
- `revert`: `Task:`.

### The paths each type may touch

- `docs` may touch only `**/*.md`, `docs/**` and `tasks/**`.
- `feat`, `fix`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `style` and `revert` may touch anything.

### What the gates run

The commit-msg hook, `itos hook commit-msg`, runs these on every commit, the first to fail refusing it:

- itos's own data, when the commit stages any of it (the config, the ledger, the work registry and the smoke sets), as `itos config check` judges it.
- The paths the commit's type may touch.
- The `scenario-moves` rule: outside `feat` and `fix`, the `scenario` tests may only move between files, unchanged.
- The header and the footers.
- The static checks of the tasks `Task:` names: a failure refuses the commit once the task's work item is done, and is only printed while it is not.

The pre-push hook, `itos hook pre-push`, runs these on every push:

- The commit rules above, over every commit the push adds, as `itos verify` judges them.

CI runs its plan, `itos ci run`, on every push, in this order, stopping at the first failure:

- `! gofmt -l cmd internal features | grep .`
- `go vet ./...`
- `itos tests smoke check scenario`
- `itos config check`
- `tools/bin/doc-caps`
- `tools/bin/pinned golangci-lint run ./...`
- The static checks of the tasks the push's commits name.
- `go tool govulncheck -test ./...`
- `RAPID_CHECKS=1 go test ./cmd/... ./internal/...`
- `tools/bin/domain-coverage`
- The `scenario` tests of the smoke set and those the push's commits name, in one run.
- `tools/bin/pinned itos-cc mutation sample --count 10`
- The other checks of the tasks the push's commits name.

A push that touches only `**/*.md`, `docs/**` and `tasks/**` runs only `itos config check` and `tools/bin/doc-caps`, and the static checks of the tasks its commits name.

<!-- itos:end -->
