# Code design rules, and how each is held

Every code design rule of `AGENTS.md` and the live decisions, and what holds it (T-49). A rule a
linter can decide without false alarms is held by golangci-lint, among the linters the release
`tools/bin/pinned` pins already ships, configured in `.golangci.yml`, whose comments name the
decision each rule holds. The hooks and CI run it on every commit and push. A rule no linter can
decide is judgement, for the author and the reviewer, and says why. What the code does, as against
how it is built, is the scenarios' to hold (`features/`), and is not listed here.

| Rule | Decision | How it is held |
| --- | --- | --- |
| A slice imports no other slice | 17 | depguard, rule `slices` |
| The domain and its ports import no infra, slice, UI or kong | 17 | depguard, rule `domain` |
| The domain's code does no I/O: no `os`, `net`, `syscall` or `log`, so it reads no environment and prints nothing | 17 | depguard, rule `domain-io` (its tests may read a fixture) |
| Infra imports nothing of ours but the ports it implements | 17 | depguard, rule `infra` |
| kong stays in the UI: `cmd`, `internal/cli` and the slices | 4, 17 | depguard, rules `kong` and `domain` |
| Every error kind is given its exit code, in one switch per sealed set, a default not counting | 17, `docs/CLI.md` rule 31 | gochecksumtype; whether a new error joins a sealed set is judgement, an unclassified one exiting 70 |
| Output goes through the writers the UI hands down, never to the process's stdout behind them | 17 | forbidigo: `fmt.Print`, `fmt.Printf`, `fmt.Println`, `print`, `println` |
| main alone exits, with the code the slice returned | 17 | forbidigo: `os.Exit`, `syscall.Exit`, `log.Fatal*`, `cmd/itos-template/main.go` excepted, and tests' `os.Exit` and `syscall.Exit`: TestMain's `os.Exit(m.Run())` and a helper process exiting with a code (`internal/git`'s stub git) are standard Go, and the rule is about production code (T-48) |
| A bug exits 70, as an error nobody classified, never as a panic | 17, 41, `docs/CLI.md` rule 31 | forbidigo: `panic`, tests left out, as a test's panic is its own failure, not the binary's exit (T-52) |
| A slice stays in one file until it grows large: 600 lines of code | 17 | revive, `file-length-limit` |
| Commands carry domain objects: no DTO or options struct copying a type | 17 | Judgement: whether a struct copies a type is what its fields stand for, which no linter reads; Run assembling a command from kong's flags is the one copy allowed |
| An anti-corruption layer only where an outside model would leak in | 17 | Judgement: whether a model would leak is a question about the outside, not a shape in the code |
| A handler only wires; the work is in the domain | 17 | Judgement: what counts as work rather than wiring is a reading of the code, though depguard keeps the domain from reaching the slices |
| No mocking library, in any package or test | 19 | depguard, rule `mocks` |
| Tests use our fakes and stubs, never a mock written by hand, and assert outcomes, never calls | 19 | Judgement: a hand-written mock is a type of ours like a fake, told apart only by scripting and checking calls; a name proves nothing either way |
| The domain is unit-tested in full | 19 | `tools/bin/domain-coverage`, a floor of 80% per domain package, in CI |
| A test outside the domain exists only for what a scenario cannot reach, saying so | 19 | Judgement: whether a scenario can reach something is not in the code |
| rapid only in the domain packages' own tests | 20 | depguard, rule `rapid` |
| Code that differs by OS takes the OS as a value: `runtime.GOOS` is read only at the edge, handed in as a value | 26 | forbidigo: every read of `runtime.GOOS` refused, the edges excepted by their source, the slices' `template.SystemOf(runtime.GOOS)` and `internal/git`'s `bin(runtime.GOOS)` (T-48), which infra hands in having no `template` to import; tests left out (below) |
| The domain differs by OS through no package: no `path/filepath`, no `runtime` | 26 | depguard, rule `domain-io` |
| A build-tagged file only for a call one OS lacks, holding no mutation site | 26 | Judgement: the decision allows such files, and whether a call is one an OS lacks is not a shape; none exists today |
| Configuration is read strictly, refusing what JSON cannot say | 22 | forbidigo: `yaml.Unmarshal` and `yaml.NewDecoder` refused outside `manifest.Decode`, tests left out (below); what the reader refuses, its unit tests and scenarios |
| A library is weighed before our own code, and the choice is the person's | 13 | Judgement: whether a library likely solves a problem is a question about the world |
| The libraries the decisions set aside stay out: cobra and urfave/cli, go-git, gopter and `testing/quick`, gitleaks as a library, the completion libraries | 4, 19, 20, 21, 25, 38 | depguard, rule `set-aside` |
| A provider is chosen when a project is made, never by runtime indirection | 2 | Judgement: what a template's code is for is not a shape a linter sees |
| A changed function's mutants are killed, or excepted with a reason | 27, 40 | The code proof, itos-cc's mutation run, in CI over a push's range |
| An exception is one finding, on its line, naming its linter and saying why, and goes with its finding | T-22, gosec's comment | nolintlint: `require-specific`, `require-explanation`, unused ones reported |

## What the linters leave to judgement

- **The edge line is allowed whole.** forbidigo excepts the line that hands the OS in, by its source,
  `SystemOf(runtime.GOOS)`, or `bin(runtime.GOOS)` in `internal/git/git.go`, so a comparison
  written on that same line, `template.SystemOf(runtime.GOOS) == template.Windows`, passes. A slice
  that branches on the System it made is the review's to catch. A new edge is added to the
  exception by name, as T-48 added `internal/git`'s, so each is a reviewed change.
- **Tests may ask which OS they run on.** A test of the real OS, an adapter's or a scenario's step,
  skips or expects what that OS does, and the code proof decision 26 serves mutates no test, so the
  `runtime.GOOS` rule holds production code only. A test copying production logic that branches
  on the OS (the scenario harness's own `executable` in `features/git_test.go`) is the review's.
- **Tests read YAML loosely.** They read what the binary wrote to check it; the strict reader is
  for the input a template brings.
- **tools/bin's programs** are the repository's own scripts, each a main printing to its stdout and
  exiting with its code, outside the layers decision 17 draws, so the printing and exiting rules
  leave them out.

## What the linters found

When T-49 turned the rules on, they found:

- `internal/git/git.go`: `names` and `executable` compared `runtime.GOOS` inside their logic,
  which decision 26 recorded as moving when an item next touched them. T-49 left each a
  `//nolint:forbidigo` above it; T-48 had them take the OS as a value, handed in by `Bin` as
  `bin(runtime.GOOS)`, the edge `.golangci.yml` excepts, and deleted both.
- `internal/manifest/manifest.go`, `Replacements`: a `panic` when an answer `Check` did not take
  reaches it. A panic exits 2 with a trace, where rule 31 has a bug exit 70. Returning an error
  instead changes `Replacements` and its three callers in `internal/template` (a render's plan,
  `Check`, and `Setup`, which returns no error today, so its callers too) and needs tests for each
  new path, so it is not this task's: the panic rule stays off until the
  person decides, the idea `no-panic`. T-52 settled it as decision 41 has it: `Question.Take` keeps
  the words it parsed in a `manifest.Answer`, `Replacements` takes those and cannot fail, and
  forbidigo refuses `panic`.
- `tools/bin/release-notes` and `tools/bin/release-version` print and exit in their main, which is
  theirs to do (above).
