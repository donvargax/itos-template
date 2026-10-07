# itos-template: the plan

itos-template makes new projects from a template that is itself a real, working project, and
keeps them up to date with it. It is an itos extension (`itos template`, also runnable on its own
as `itos-template`), written in Go. This file says what it is, how it works and in what order it
is built. The decisions behind it are records in `docs/decisions/`; what a made project contains
is `docs/template-contents.md`.

## Why

Template engines put placeholders in the files (Jinja2 in copier and cookiecutter, Go templates),
so the template is not a project: it does not build, its tests do not run, nothing proves it
works until someone renders it. The user likes copier's update story and dislikes both template
languages. itos-template keeps the update story and drops the template language.

## The model

1. **Literal substitution.** The template uses real, distinctive values: a name `acme-widget`, a
   module path `example.com/acme/widget`, a type `AcmeWidget`. A manifest maps each literal to a
   question; rendering rewrites the answers into file contents and file names, with their case
   forms (kebab, snake, camel, Pascal, upper). It reads no language's syntax, so it is
   stack-agnostic. Prior art: `dotnet new` (`sourceName`, symbols) and `gonew` (a Go module
   copied, its path rewritten), each tied to its stack, which this is not.
2. **Optional parts are git branches, never conditionals.**

   ```
   root                 docs every stack shares: AGENTS.md, itos.yaml, the CI skeleton, README
   ├── stack/go         root + a minimal working Go project
   │   ├── go/cli       a feature, branched off its stack
   │   └── go/web
   └── stack/python
       └── python/cli
   ```

   A feature needing another branches off that one (TopGit's topic branches with dependencies are
   the prior art; the manifest lists the dependencies). The root is merged down into stacks and
   features, never rebased, so commits keep their SHAs. A template release is a tag.
3. **Rendering** merges the chosen stack and features (git's merge, so what branches share is
   known, unlike a cherry-pick replaying a diff), then substitutes the literals last, so every
   branch stays a real project.
4. **Updating** is copier's and cruft's method: the project records the template tag, the stack,
   the features and the answers; an update renders the old tag and the new one with the same
   answers and merges their difference into the project (a 3-way merge, the old render the base),
   landing as an ordinary commit through the project's gates. Substitution is deterministic, so
   renders reproduce and merges stay clean.
5. **The template proves itself.** Its CI renders every combination the manifest allows and runs
   that render's own checks (the commands the manifest names: build, tests, lint), and scans each
   render for leftover literals, stray replacements and leaked credentials. A combination not
   tested is listed as unsupported, never offered.

## Principles

- Selection at generation time, never runtime indirection: a provider (package index, registry,
  cloud, deployment, CI) is a feature branch, and no generated code exists only to tell providers
  apart.
- No credentials in answers, generated files, image layers, lockfiles, fixtures or logs.
- Few combinations, all tested; simple generated projects over a clever generator.
- Standard protocols and git's own machinery over custom formats.

## Commands (first shape, to settle in the scenarios)

- `itos-template new <template> <folder> [--stack s] [--feature f]… [--answer k=v]…`: renders a
  project, asking what the answers lack on a terminal, and records what it rendered.
- `itos-template update [--to <tag>]`: brings a project to a newer template release.
- `itos-template check`: in a template repository, renders every allowed combination and runs
  its checks (the template's CI step).
- `itos-template sync`: in a template repository, merges the root down into the stacks and the
  stacks into their features.

## Order of work

1. **Bootstrap** (T-3 to T-5): the Go module and a kong command skeleton with `--version`; the
   acceptance harness (godog against the built binary) and unit tests; CI on Linux, macOS and
   Windows, and releases cut by CI with GoReleaser, attested. Harvested from itos
   (`~/tmp/llms-safe/gamedev/svg-art/itos`, github.com/donvargax/itos), not rewritten: its
   workflows, its godog harness, its release tooling and `docs/CLI.md`'s rules.
2. **`new`, the smallest useful slice:** the manifest, merging a stack with features, literal
   substitution with case forms, recording what was rendered.
3. **`check`:** every allowed combination rendered and checked; the leak and leftover scans.
4. **`update`:** the 3-way merge.
5. **`sync`.**
6. **The first real template:** this repository's own skeleton, its domain code stripped, becomes
   `stack/go` and `go/cli`, so the generator is the proof that the template works.

## What it is built with

Go, kong for commands and flags (a `--no-` pair for every switch, an environment variable per
flag), structured logs to stderr, godog acceptance tests against the binary, golangci-lint,
govulncheck, GoReleaser. itos from the first commit: its hooks, its ledger and registry,
scenarios as the specification, decision records.

## References

- copier and cruft (updates by 3-way merge of renders): https://copier.readthedocs.io,
  https://cruft.github.io/cruft/
- dotnet new templates: https://learn.microsoft.com/dotnet/core/tools/custom-templates
- gonew: https://go.dev/blog/gonew
- TopGit: https://github.com/mackyle/topgit
- kong: https://github.com/alecthomas/kong
- itos: https://github.com/donvargax/itos
