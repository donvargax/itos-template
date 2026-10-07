# The manifest

A template says what it offers in one file, `itos-template.yaml`, at the top of its root branch and
merged down into every stack and feature branch with the rest of the root (decision 8), so each
branch carries the same manifest and stays a real project. `itos-template new` reads it from the
template's default branch, which is its root branch. The manifest is never part of a render.

## An example

The fixture the scenarios render (`features/testdata/acme/main/itos-template.yaml`):

```yaml
version: 1
stacks:
  - name: go
  - name: python
features:
  - name: cli
    stack: go
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

| Key             | What it holds                                                                    |
| --------------- | -------------------------------------------------------------------------------- |
| `version`       | `1`, the format this page describes. Required.                                   |
| `stacks`        | The stacks, at least one, each a `name`.                                         |
| `features`      | The features: each a `name`, its `stack`, and the features it `needs`, if any.   |
| `questions`     | The literals the answers replace, and the questions asked for them.              |
| `template_only` | Paths only the template keeps, left out of every render.                         |

A key the format does not list is refused, so a misspelt key never passes for an option.

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
letters and digits joined by dashes, two words or more, so its five forms are five different
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

## What comes later

The format grows with the items that need it, each with the version that names its keys, so a
template written for a later itos-template is refused by an earlier one rather than misread:

- setup steps per branch, run once a project is made (decision 7; the idea new-setup), likely
  `setup` on the top (the root), on a stack and on a feature;
- the checks a template's CI runs on every render (the idea template-check), likely `checks` in the
  same places.

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
