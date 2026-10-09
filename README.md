# itos-template

New projects from a template that is a real, working project, kept up to date with it.

No template language: the template builds and passes its own tests like any project. Answers
replace distinctive literals in it (a name, a module path) when a project is made; optional parts
are git branches merged in, not conditionals in the files; updates are 3-way merges of the old and
new renders, as copier does them. It is an [itos](https://github.com/donvargax/itos) extension,
`itos template`, and runs on its own as `itos-template`.

`itos-template new` makes a project from a template's branch heads:

```sh
itos-template new ../acme made --stack go --feature cli --answer name=blue-fox --defaults
```

A template says what it offers in its manifest, `itos-template.yaml` (`docs/manifest.md`). Updating
a made project, checking a template and syncing its branches come next: `PLAN.md` says what is
planned and in what order.

## Working on it

Go (the version `go.mod` names), git 2.54 or later and [itos](https://github.com/donvargax/itos).
Once in each clone, declare the hooks in its git config: itos's commit-msg and pre-push, and this
repository's pre-commit, which runs golangci-lint and govulncheck at their pinned versions
(`tools/hooks/pre-commit`):

```sh
itos hook install
git config hook.itos-template-pre-commit.event pre-commit
git config hook.itos-template-pre-commit.command tools/hooks/pre-commit
```

```sh
go build ./cmd/itos-template               # the binary
go test ./... -count=1                      # the unit tests and every live scenario
go test ./features -count=1                 # the scenarios, against the binary built from this tree
tools/bin/pinned golangci-lint run ./...    # the linters, at the pinned release
go tool govulncheck -test ./...             # the vulnerability check, at go.mod's pin
```

`features/README.md` says how the scenarios are written and run.

To enable shell completion, print the matching stub with `itos-template completion bash`,
`itos-template completion zsh`, `itos-template completion fish` or
`itos-template completion powershell`, then follow its printed install location and reload your
shell. The stub asks the executable for candidates, so it stays current with the command model.
