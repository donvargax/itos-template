# The manifest

A template says what it offers in one file, `itos-template.yaml`, at the top of its root branch and
merged down into every stack and feature branch with the rest of the root (decision 8), so each
branch carries the same manifest and stays a real project. `itos-template new` reads it from the
template's default branch, which is its root branch, and so does `itos-template check`. The manifest
is never part of a render.

## An example

The fixture the scenarios render (`features/testdata/acme/main/itos-template.yaml`):

```yaml
version: 2
# The root's checks, run in every render first, then the stack's, then the
# features' (features/check.feature's description).
checks:
  - [git, ls-files, --error-unmatch, README.md]
stacks:
  - name: go
    checks:
      - [git, ls-files, --error-unmatch, cmd/acme-widget/main.go]
  - name: python
    checks:
      - [git, ls-files, --error-unmatch, pyproject.toml]
features:
  - name: cli
    stack: go
    checks:
      - [git, ls-files, --error-unmatch, cli.txt]
  - name: web
    stack: go
    needs: [cli]
  - name: cli
    stack: python
questions:
  - name: name
    literal: acme-widget
    question: What is the project's name?
    pattern: "[a-z][a-z0-9]*(-[a-z0-9]+)*"
    case_forms: true
  - name: module
    literal: example.com/acme/widget
    question: What is the project's module path?
    pattern: "[a-z0-9.-]+(/[a-z0-9._-]+)+"
    default: example.com/you/project
template_only:
  - .github/workflows/template.yml
```

## The keys

| Key             | What it holds                                                                               |
| --------------- | ------------------------------------------------------------------------------------------- |
| `version`       | `2`, the format this page describes, or `1`, the same less the keys of version 2. Required. |
| `stacks`        | The stacks, at least one, each a `name` and its `checks`, if any.                           |
| `features`      | The features: each a `name`, its `stack`, the features it `needs` and its `checks`, if any. |
| `questions`     | The literals the answers replace, and the questions asked for them.                         |
| `template_only` | Paths only the template keeps, left out of every render.                                    |
| `checks`        | The root's checks, run in every render first. Version 2.                                    |
| `unsupported`   | The combinations the template cannot support, each its `stack` and `features`. Version 2.   |

A key the format does not list is refused, so a misspelt key never passes for an option. Version 2
adds `checks` (on the top, on a stack and on a feature) and `unsupported`; a manifest of version 1
is read as it always was, a template with no checks, and refuses those keys, so a manifest written
for version 2 is never misread as one without them.

### Branches

The manifest names no branch; the names follow from the stacks and features:

- The **root** is the template's default branch (`main`, say): what every stack shares.
- A **stack** `go` is the branch `stack/go`, branched off the root.
- A **feature** `cli` of the stack `go` is the branch `go/cli`, branched off its stack, or off the
  feature it needs (`go/web` off `go/cli`).

A stack or feature name is lowercase letters, digits, dots, dashes and underscores. Two stacks may
have features of the same name (`go/cli`, `python/cli`): on the command line, `--feature cli`
means the chosen stack's, and `--feature python/cli` names one by its branch.

`new` renders the stack's branch merged with the chosen features' branches by git, in the order the
manifest lists the features, so the order of the command line never changes a render. A feature
of another stack, or a feature whose `needs` are not all chosen, is refused: a needed feature is
never added unasked. A merge that leaves conflicts is a defect of the template, refused naming
the branches; merge them in the template and resolve it there.

### Combinations

A combination is a stack and the features merged onto it, written as `check` reports it: the stack,
then each feature's name in the manifest's order, joined by ` + ` (`go + cli + web`). The
combinations a manifest allows are derived, never listed: each stack alone, and each stack with
every set of its features in which every feature's needs are in the set too. The fixture allows
`go`, `go + cli`, `go + cli + web`, `python` and `python + cli`.

Each feature that needs no other doubles its stack's combinations (3 make 8 renders, 10 make
1,024), and `check` renders and checks every one, so a template keeps its features few and lists
under `unsupported` what it cannot support:

```yaml
unsupported:
  - stack: go
    features: [cli, web]
```

An entry is a stack and exactly the features it lists, by their names in the stack, in any order;
`features` left out is the stack alone. `check` neither renders nor checks it, and `new` refuses
it with exit 1, naming it. An entry naming no combination the manifest allows (a stack or a feature
it does not list, a feature named twice, a feature without the features it needs) is a manifest
refused.

### Checks

A check is a command, written as a list of words, the program first:

```yaml
checks:
  - [go, build, ./...]
  - [go, test, ./...]
```

It runs with no shell, so it means the same on Linux, macOS and Windows: no quoting, no pipes, no
variables, no globs. A template that wants a shell writes `[sh, -c, "…"]` itself, and runs where
that shell is. A check written as one string is refused.

Checks live beside what they check: `checks` on the manifest's top for the root, and `checks` on
each stack and each feature. A combination's checks are the root's, then its stack's, then its
features' in the order the manifest lists the features. Each runs in the render's top folder, its
standard input empty, in itos-template's environment (less the variables that point git at another
repository), and each word has the literals replaced by the answers as a text file's contents
have, so `[go, run, ./cmd/acme-widget]` runs `./cmd/blue-fox` in a render whose name is
`blue-fox`. A check passes when it exits 0. A combination's checks stop at its first failure, as
a CI job's steps do; the checks after it are reported skipped.

### Questions

| Key          | What it holds                                                                          |
| ------------ | -------------------------------------------------------------------------------------- |
| `name`       | The answer's name on the command line (`--answer name=blue-fox`) and in the record.     |
| `literal`    | The value the template uses where the answer goes.                                      |
| `question`   | The question asked on a terminal.                                                       |
| `pattern`    | A Go regular expression (RE2) the whole answer must match. Optional.                    |
| `default`    | The answer taken with `--defaults`, or by an empty line on a terminal. Optional.        |
| `case_forms` | `true` to replace the literal in its five case forms; `false` (the default) as written. |

An answer is never empty. A `default` must be an answer its question takes.

**Case forms.** With `case_forms: true` the literal is written in kebab case, lowercase words of
letters and digits joined by dashes, two words or more, and its five forms must be five different
strings. The answer is written the same way. Each form of the literal is replaced by the same form
of the answer:

| Form        | Literal       | Answer `blue-fox` |
| ----------- | ------------- | ----------------- |
| kebab       | `acme-widget` | `blue-fox`        |
| snake       | `acme_widget` | `blue_fox`        |
| camel       | `acmeWidget`  | `blueFox`         |
| Pascal      | `AcmeWidget`  | `BlueFox`         |
| upper snake | `ACME_WIDGET` | `BLUE_FOX`        |

The dashes are the only word breaks: nothing is guessed, so `http-server` is `HttpServer`, never
`HTTPServer`, and a render is the same whatever version of itos-template makes it.

Two forms are one string when the first word starts with a digit: `2fa-code` is `2faCode` in both
camel and Pascal case. When every word is digits alone, snake and upper snake are one too: `1-2` is
`1_2` in both. One string cannot be replaced by two answers, so the manifest refuses such a
literal, naming the forms that collide; start its first word with a letter (`code-2fa`,
`v1-2`). An answer is not held to this: the answer `2fa-code` only renders the literal's camel and
Pascal forms alike.

**Where literals are replaced.** In every text file's contents (a file with no NUL byte in its
first 8000 bytes, as git tells text from binary; any other file is copied as it is), in each
element of every file and folder name, and in a symbolic link's target. Replacement works within
lines: a line ending is never touched, so a file with CRLF keeps it. Where two literals match at
one place, the longer is replaced. Two literals, or two forms, may not be the same string, and the
answers may not make two files one name or a name no file can have (`..`, a `/` within it).

### Paths only the template keeps

Each `template_only` entry is a path relative to the template's top, written with `/`: a file, or
a folder and everything under it. The template's own CI, the one that renders every combination,
is the usual one. The manifest itself is always left out.

## What check reports

`itos-template check [<template>]` renders every combination the manifest allows, each as `new`
renders it, in a temporary folder of its own, runs its checks there and removes the folder. The
template is anything git clone takes, the folder `check` runs in when none is named. `check` never
asks: every answer comes from `--answer`, or with `--defaults` from its question's default, and a
missing one is refused, exit 2, each named, before anything is rendered. Each render's first commit
is made as `itos-template check`, so a CI that sets no git identity can run it.

Its report, on standard output, is a block per combination, in the order of
[Combinations](#combinations), then an empty line and a count:

```text
go: passed
  passed: git ls-files --error-unmatch README.md
  passed: git ls-files --error-unmatch cmd/blue-fox/main.go
go + cli + web: failed
  passed: git ls-files --error-unmatch README.md
  passed: git ls-files --error-unmatch cmd/blue-fox/main.go
  passed: git ls-files --error-unmatch cli.txt
  failed: git ls-files --error-unmatch missing.txt
    | error: pathspec 'missing.txt' did not match any file(s) known to git
    | Did you forget to 'git add'?
  skipped: go test ./...
python + cli: failed
  not rendered
    | merging the template's branch python/cli into stack/python leaves conflicts in pyproject.toml: …

1 of 3 combinations passed.
```

- A combination's line is its name, a colon and `passed` or `failed`, at the start of the line.
- Each check follows, indented two spaces: `passed:`, `failed:` or `skipped:` and the check as it
  ran, the answers in place. A word is written as it is, unless it is empty or holds a space, a
  quote, a backslash or a character that does not print: then it is in double quotes, as Go writes
  a string.
- A failed check is followed by what it wrote, its standard output and error as they came, each
  line indented four spaces after a `|` (a space between them when the line is not empty). A
  program that cannot be started fails, and its output says why.
- A render that failed is `not rendered`, indented two spaces, followed by why in the same way.
- The last line is `<passed> of <all> combinations passed.`

`check` exits 0 when every combination rendered and every check passed, 1 when any check or render
failed, 2 for a missing answer or a manifest refused, and 3 when git cannot reach the template.

### Checking a template in its CI

`check` renders the template's branch heads, as `new` does, by cloning it: never its working tree.
So a template's CI fetches every branch as a local branch first, and puts `HEAD` on the root branch,
where the manifest is read. With GitHub Actions:

```yaml
- uses: actions/checkout@v5
  with:
    fetch-depth: 0
- run: |
    git fetch --update-head-ok origin '+refs/heads/*:refs/heads/*'
    git checkout --quiet main
- run: itos-template check --answer name=blue-fox --defaults
```

## What comes later

The format grows with the items that need it, each with the version that names its keys, so a
template written for a later itos-template is refused by an earlier one rather than misread:

- setup steps per branch, run once a project is made (decision 7; the idea new-setup), likely
  `setup` on the top (the root), on a stack and on a feature.

## The record a made project keeps

`new` writes `.itos-template.yaml` at the made project's top and commits it with the render
(decision 10): the template as it was named, the stack, the features, the answers, and the commit
each of the template's branches was at, the root included. A template may not hold a file of that
name.

```yaml
version: 1
template: ../acme
stack: go
features:
  - cli
answers:
  module: example.com/blue/fox
  name: blue-fox
commits:
  go/cli: 3f1c…
  main: 9a2e…
  stack/go: 7b44…
```

The release a project was made from joins it once a template release is settled (the idea
template-releases).
