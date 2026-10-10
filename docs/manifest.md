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

| Key             | What it holds                                                                                           |
| --------------- | ------------------------------------------------------------------------------------------------------- |
| `version`       | `5`, this page's format; each earlier version lacks the keys a later one adds. Required.                |
| `stacks`        | The stacks, at least one, each a `name` and its `checks` and `setup`, if any.                           |
| `features`      | The features: each a `name`, its `stack`, the features it `needs` and its `checks` and `setup`, if any. |
| `questions`     | The literals the answers replace, and the questions asked for them.                                     |
| `template_only` | Paths only the template keeps, left out of every render.                                                |
| `checks`        | The root's checks, run in every render first. Version 2.                                                |
| `unsupported`   | The combinations the template cannot support, each its `stack` and `features`. Version 2.               |
| `first_commit`  | The made project's first commit message, a string. Version 4. Optional.                                 |
| `setup`         | The root's setup steps, printed first for the person to run once a project is made. Version 5.          |

The manifest is JSON data written as YAML, and read strictly (`docs/CONFIG.md`, rule 1): a tool
may write JSON into `itos-template.yaml` and it reads the same, while what JSON cannot say is
refused, every problem named with its line. That is a tag (`!foo`, or a core one written out, as
`!!str`), an anchor, an alias, a merge key (`<<`), a second document after a `---`, a key that is
not a string and a key given twice. The non-specific tag `!` is the one tag taken, read as if it
were not there: it marks a plain value as no other tag's, building no object and expanding nothing,
and the reader takes what is only spelling, as quotes, comments and flow style are.

A key the format does not list is refused, so a misspelt key never passes for an option. Version 2
adds `checks` (on the top, on a stack and on a feature) and `unsupported`; a manifest of version 1
is read as it always was, a template with no checks, and refuses those keys, so a manifest written
for version 2 is never misread as one without them. Version 3 adds a check's long form, which says
what the check scans a render for ([Checks](#checks)); version 2 refuses it. Version 4 adds
`first_commit`, the message of a made project's first commit ([The first
commit](#the-first-commit)); version 3 refuses it. Version 5 adds `setup` (on the top, on a stack and
on a feature), the steps `new` prints for the person to run ([Setup steps](#setup-steps)); version
4 refuses it, an empty list too.

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

From version 3 a check may be written in its long form, `{run: [words…], scans: [what it scans]}`,
which runs `run` as the list of words runs, and says what the check scans each render for. The list
of words stays a check in every version. `scans` takes one value for now, `credentials`: the check
scans the render for leaked credentials (decision 21). Any other value is refused.

```yaml
version: 3
checks:
  - [go, test, ./...]
  - {run: [gitleaks, dir, ., --redact], scans: [credentials]}
```

itos-template scans no render for credentials itself, and knows no tool that does: a template marks
its own check, and `check` warns when no check of the template is marked. One way is
[gitleaks](https://github.com/gitleaks/gitleaks), pinned to a version and checksum as a template
pins its other tools, run as the root's check so every render is scanned.

### Questions

| Key          | What it holds                                                                          |
| ------------ | -------------------------------------------------------------------------------------- |
| `name`       | The answer's name on the command line (`--answer name=blue-fox`) and in the record.     |
| `literal`    | The value the template uses where the answer goes.                                      |
| `question`   | The question asked on a terminal.                                                       |
| `pattern`    | A Go regular expression (RE2) the whole answer must match. Optional.                    |
| `default`    | The answer taken with `--defaults`, or by an empty line on a terminal. Optional.        |
| `case_forms` | `true` to replace the literal in its five case forms; `false` (the default) as written. |

An answer is never empty, and is free text as PRECIS (RFC 8264) takes it, its FreeformClass,
whatever the pattern: it holds no control character and none that prints as nothing or moves the
text around it, a right-to-left override, a zero-width space, a Hangul filler or a variation
selector among them, so an emoji written with U+FE0F is refused. The zero-width non-joiner and
joiner, U+200C and U+200D, are taken only where a script spells with them, after a virama or
between letters that join, as Persian and Hindi write; between Latin letters, or the people of a
family emoji, they are refused. A refusal names the character by its code point, and an answer
taken is used as given, never normalized. A `default` must be an answer its question takes, and is
held to this when the manifest is read.

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
answers may not make two files one name, a file the folder of another, or a name no file can have
(`..`, a `/` within it). A render refuses such answers before writing anything, naming every
clash, in the order of the names the answers make.

### The first commit

`new` commits the render as the made project's first commit. Without `first_commit` its message
is itos-template's own:

```text
chore: make the project from its template

Made by itos-template new from ../acme: the stack go, the features cli. .itos-template.yaml records the render.
```

A template that holds its projects to commit rules (itos, or any commit lint) judges that commit
too: on the made project's first push its CI has no green run to start from, so it judges the
whole history, and a rule the message breaks (a chore needing a `Task:` footer, say) fails it.
The template knows its own rules, so from version 4 its manifest gives the message, and the first
commit passes them. A template held to itos makes it a chore with a `Task:` footer, as its rules
ask of a chore, naming a task its ledger ships as done, the project's start (`T-1` here): the task
is in the made project's ledger, as the footer needs, and done, it leaves the project nothing to
finish:

```yaml
version: 4
first_commit: |
  chore: start acme-widget

  Made from the acme template by itos-template new; .itos-template.yaml
  records the render.

  Task: T-1
```

`first_commit` is the whole message, a string: its first line the header, then, if any, a blank
line, a body and footers, as `git commit` takes them. A block scalar (`|`) keeps its lines as
they are written. The literals in it are replaced by the answers as a text file's contents are,
case forms included, within lines (the message above makes `chore: start blue-fox`). A message
whose first line is empty or blank is refused, naming `first_commit`: git would drop that line and
take the next for the header, a footer as likely as not. So is one holding a NUL, which no commit
message can. git commits the message as it commits any given with `-m`: lines' trailing
whitespace and the blank lines at its ends removed, runs of blank lines made one, and every other
line kept, one starting with `#` too, whatever the person's `commit.cleanup` says.

`check` renders each combination through the same code, so each render's first commit carries the
message too.

### Setup steps

What a made project needs set up before it is worked in (its hooks, its tools) is the template's to
say, never the generator's (decision 7): from version 5, `setup` lists steps on the top for the
root, on each stack and on each feature, each step a list of words, the program first, as a check
is, and never one string:

```yaml
version: 5
setup:
  - [itos, init, --agent-rules]
stacks:
  - name: go
    setup:
      - [git, config, core.hooksPath, tools/hooks]
      - [sh, -c, "go mod download && go build ./cmd/acme-widget"]
```

`new` runs none of them. It prints the steps of the combination it made, after what it made, for
the person to read and run in the made project's folder: a line heading them, then one step to a
line, in the order checks run (the root's, then the stack's, then the features' in the manifest's
order). Each word has the literals replaced by the answers as a check's words have, so the step
above prints `./cmd/blue-fox` in a project named `blue-fox`.

```text
Made made from ../acme: the stack go, no features.
itos-template ran none of the template's setup steps; run them in made:
itos init --agent-rules
git config core.hooksPath tools/hooks
sh -c 'go mod download && go build ./cmd/blue-fox'
```

A step line is only the words a shell should run, each quoted for a POSIX shell (sh, bash, zsh)
and joined by one space: a word of only ASCII letters, digits and `@%+=:,./_-` as it is, an empty
one as `''`, and any other in single quotes, each `'` in it written `'"'"'`. Pasted into a shell,
the step runs its own words, not the shell's reading of them. cmd.exe and PowerShell quote
otherwise: a template meaning to set up on Windows writes words that need no quoting. With `--json`
the heading and the steps go to the error output, so the object stays alone on standard output. A
template that lists no step prints nothing about setup.

A step is the template's code, shown to a person who has not yet chosen to trust it, and quoting
cannot make a control character safe to print: an escape sequence in a word can make a terminal
show another step than the one pasted. So a word holding a control character (tab, CR and LF
included) is a manifest refused, exit 2, each named with its line, before anything is written.
A word holding a character Unicode lists as Default_Ignorable_Code_Point, or a line or paragraph
separator (U+2028, U+2029), is refused the same way, named by its code point: it prints as nothing
or moves the text around it, so the step read is not the step run. The set is Unicode's, from Go's
tables: every format character (Cf: a right-to-left override, a zero-width space or joiner), the
Hangul fillers (U+115F, U+1160, U+3164, U+FFA0), the combining grapheme joiner U+034F and the
variation selectors among them. An emoji written with a variation selector (U+FE0F after a heart)
or with joiners (U+200D between the people of a family) is refused with them: write it without
the selector, which most terminals draw the same.

### Paths only the template keeps

Each `template_only` entry is a path relative to the template's top, written with `/`: a file, or
a folder and everything under it. The template's own CI, the one that renders every combination,
is the usual one. The manifest itself is always left out.

## What check reports

`itos-template check [<template>]` renders every combination the manifest allows, each as `new`
renders it, in a temporary folder of its own, scans it for leftover literals, runs its checks there
and removes the folder. The
template is anything git clone takes, the repository `check` runs in when none is named. `check` never
asks: every answer comes from `--answer`, or with `--defaults` from its question's default, and a
missing one is refused, exit 2, each named, before anything is rendered. Each render's first commit
is made as `itos-template check`, so a CI that sets no git identity can run it, with the message
`new` gives it ([The first commit](#the-first-commit)).

A **leftover** is a literal a render kept in a form no answer replaced: a case-forms literal's words,
in order and in any case, joined by nothing, a space, `.`, `-`, `_` or `/` (`Acme Widget` in a
heading, `acmewidget` in a host name), or a literal without case forms as it is written, found in a
text file's contents, a link's target or a path. A binary file is not read, as no answer replaces
anything in it. Each leftover fails its combination: write that spot in one of the five forms, so
the answer replaces it, or name it otherwise. The combination's checks still run.

Its report, on standard output, is a block per combination, in the order of
[Combinations](#combinations), then an empty line and a count:

```text
go: failed
  leftover: "acme.widget" in the path docs/acme.widget.md
  leftover: "Acme Widget" at docs/title.md:1
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
- Each leftover follows, indented two spaces: `leftover:`, the text found in double quotes as Go
  writes a string, then where: ` at ` and the path, a colon and the line (from 1), for one in a
  file, or ` in the path ` and the path, for one in a path. The path is written as a check's word
  is. A render's leftovers come in the order of its files, a file's path before its lines.
- Each check follows, indented two spaces: `passed:`, `failed:` or `skipped:` and the check as it
  ran, the answers in place. A word is written as it is, unless it is empty or holds a space, a
  quote, a backslash or a character that does not print: then it is in double quotes, as Go writes
  a string.
- A failed check is followed by what it wrote, its standard output and error as they came, each
  line indented four spaces after a `|` (a space between them when the line is not empty). A
  program that cannot be started fails, and its output says why.
- A render that failed is `not rendered`, indented two spaces, followed by why in the same way.
- The last line is `<passed> of <all> combinations passed.`

`check` exits 0 when every combination rendered, kept no literal and passed every check, 1 when
any render, scan or check failed, 2 for a missing answer or a manifest refused, and 3 when git cannot
reach the template.

When no check of the template is marked `scans: [credentials]`, `check` warns on its error output,
before its report, and the warning changes no exit code:

```text
itos-template: warning: no check of the template is marked as scanning its renders for credentials: mark the one that does with scans: [credentials], in a check's long form (docs/manifest.md)
```

### Checking a template in its CI

`check` renders the template's branch heads, as `new` does: never its working tree. With no template
named, it reads the repository it runs in, from its top whatever folder of it it runs in, and takes
each branch the manifest names from the local branch of that name, else from origin's
remote-tracking branch of that name. A checkout as git clone or actions/checkout leaves one, its
other branches only origin's, is checked as it is, with no fetch first.

The root branch, where the manifest is read, is the branch `HEAD` names. A detached `HEAD`, as
actions/checkout leaves on a pull request, names none: the root is then the branch origin's `HEAD`
names, as git clone records it, else as origin says it (`git ls-remote`). On a pull request, `check`
so renders the branches as they are, not the change the pull request proposes.

actions/checkout fetches only the commit it checks out unless told to fetch the whole history, which
every branch's remote-tracking branch needs. With GitHub Actions:

```yaml
- uses: actions/checkout@v5
  with:
    fetch-depth: 0
- run: itos-template check --answer name=blue-fox --defaults
```

## What comes later

The format grows with the items that need it, each with the version that names its keys, so a
template written for a later itos-template is refused by an earlier one rather than misread:

- nothing yet: running the setup steps for the person, under a flag naming what it risks (the idea
  new-trust), needs no key of its own.

## The record a made project keeps

`new` writes `.itos-template.yaml` at the made project's top and commits it with the render
(decision 10): the template, the stack, the features, the answers, and the commit each of the
template's branches was at, the root included. A template may not hold a file of that name.

The template is recorded by a name `update` can reach again from anywhere on the machine, holding
no credential, though `new` clones it by the name as given (record-name):

- A URL's userinfo is left out: `https://x-access-token:<token>@host/acme.git` is recorded as
  `https://host/acme.git`, as bug-4 leaves it out of every error. The record is committed, so a
  credential there would sit in the made project's history; `update` reaches the template through
  git's credential helper instead (decision 9, `git help credentials`), and `new` says so once on
  its error output. The same name is the one `new` prints, puts in `--json`'s `template` and in the
  first commit's message.
- A relative path (`../acme`) is recorded as the absolute path it named from the folder `new` ran
  in, written as the system writes a path: `/home/you/acme`, or `C:\Users\you\acme` on Windows,
  whichever separators it was given with. Recorded as given it would reach the template only from
  that folder. A made project published elsewhere then carries that machine's path, which names a
  folder, never a secret.
- An absolute path, a URL with no userinfo and an scp-like name (`git@host:path`, a user and no
  secret) are recorded as given. Each kind is told as git tells it, so the name recorded is the
  template git cloned; on Windows a path starting with a drive (`C:\acme`, `C:acme`) or a share
  (`\\server\share`) is absolute.

The clone `new` renders from is a bare repository in a temporary folder, its origin the name as
given, removed when `new` ends; the made project is a repository of its own, with no remote.

```yaml
version: 1
template: /home/you/acme
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
