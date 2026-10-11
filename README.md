# itos-template

New projects from a template that is a real, working project, kept up to date with it.

No template language: the template builds and passes its own tests like any project. Answers
replace distinctive literals in it (a name, a module path) when a project is made; optional parts
are git branches merged in, not conditionals in the files; updates are 3-way merges of the old and
new renders, as copier does them. It is an [itos](https://github.com/donvargax/itos) extension,
`itos template`, and runs on its own as `itos-template`.

`itos-template new` makes a project from a template's newest release, the one `--ref` names, or
its branch heads while it has none:

```sh
itos-template new ../acme made --stack go --feature cli --answer name=blue-fox --defaults
itos-template new ../acme made --stack go --answer name=blue-fox --defaults --ref v1.2.0
```

A release is one version tagged `<branch>/<version>` on every branch the template's manifest lists
(decision 36). `itos-template release`, run in the template's repository, cuts one: it runs `check`
over the branch heads, each local one or else origin's, and only once every combination passes tags
each branch there, annotated. It needs every local branch at origin's head, and pushes nothing: it
prints the `git push --atomic origin …` that publishes the tags, for you to run:

```sh
itos-template release v1.2.0 --answer name=blue-fox --defaults
```

`itos-template adopt` links a project made before the template to the release it matches, writing
only the record `new` writes, `.itos-template.yaml`, for you to commit; `--dry-run` previews how the
project differs from that release's render, and `--force` accepts the differences. `--merge` takes
that render in instead, for a project that grew on its own: git's own merge, over an empty base,
adds each file the project lacks and leaves a file both hold differently as a conflict, staged with
the record for you to commit, and `git reset --hard` undoes it. It needs git 2.45 or later.
`--decline <path>`, once for each, turns down a file or a folder of the release the project does not
want, such as a sample app: it is left out of the merge, and the record lists it, so no later update
brings it back:

```sh
itos-template adopt ../acme --ref v1.0.0 --stack go --feature cli --answer name=blue-fox --defaults --dry-run
itos-template adopt ../acme --ref v1.0.0 --stack go --feature cli --answer name=blue-fox --defaults --merge
itos-template adopt ../acme --ref v1.0.0 --stack go --feature cli --answer name=blue-fox --defaults --merge --decline docs/sample
```

`itos-template update`, in a made or adopted project, merges the template's newest release, or the
one `--to` names, into it with git's own merge, the render its record names the base, and leaves
the merge and the new record staged for you to commit; a conflict is left as `git merge` leaves one,
and `git reset --hard` undoes the update. It needs git 2.45 or later:

```sh
itos-template update
itos-template update --to v1.2.0
```

A template says what it offers in its manifest, `itos-template.yaml` (`docs/manifest.md`). Syncing a
template's branches comes next: `PLAN.md` says what is planned and in what order.

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
