@phase-1
Feature: new makes a project from a template
  itos-template new <template> <folder> renders a project from a template that
  is itself a real project (decision 1): the stack's branch merged with the
  chosen features' branches by git (decision 2), then each literal the
  manifest lists replaced by its answer in every case form, in file contents
  and file names. The manifest is itos-template.yaml on the root branch,
  merged down into every branch (decision 8). The template is anything git
  clone takes (decision 9). The made project records its render in
  .itos-template.yaml (decision 10), and its first commit is the render
  (decision 12). Answers not given are asked on a terminal and refused without
  one unless --defaults (decision 11); the scenarios run with no terminal, so
  asking is held by unit tests.

  This slice renders the branches' heads: --ref and rendering the newest
  release wait for template-releases, which settles how one release is tagged
  across branches.

  The fixture template "acme" is a git repository the steps build from
  testdata, its branches:
    main          itos-template.yaml; README.md naming the literal acme-widget
                  in its five forms (acme-widget, acme_widget, acmeWidget,
                  AcmeWidget, ACME_WIDGET); logo.bin, a binary file holding
                  the bytes acme-widget after a NUL; and
                  .github/workflows/template.yml, which the manifest lists as
                  the template's own
    stack/go      main, plus cmd/acme-widget/main.go and go.mod declaring the
                  module example.com/acme/widget
    go/cli        stack/go, plus cli.txt
    go/web        go/cli, plus web.txt (the feature web needs cli)
    stack/python  main, plus pyproject.toml
    python/cli    stack/python, plus py-cli.txt
  Its manifest's questions: name, the literal acme-widget, a lowercase word
  or words joined by dashes, no default, replaced in its five case forms; and
  module, the literal example.com/acme/widget, default
  example.com/you/project, replaced as written, with no case forms. A feature
  is named by its name within the stack (cli) or by its branch (python/cli).
  The manifest's format is documented in docs/manifest.md, written by this
  slice.

  A text file is one with no NUL byte in its first 8 KB, as git tells them;
  any other file is copied as it is. In a step {template} stands for the
  fixture's path, and a folder is relative to the scenario's scratch folder.

  @ID-NEW-01 @slice-1
  Scenario: new renders the stack into a new folder, the answers in every case form in contents
    Given the template "acme"
    When itos-template runs with "new {template} made --stack go --answer name=blue-fox --answer module=example.com/blue/fox"
    Then it exits with code 0
    And the file "made/README.md" contains "blue-fox", "blue_fox", "blueFox", "BlueFox" and "BLUE_FOX"
    And the file "made/go.mod" contains "module example.com/blue/fox"
    And no text file of the project in "made" contains "acme-widget", "acme_widget", "acmeWidget", "AcmeWidget", "ACME_WIDGET" or "example.com/acme/widget"

  @ID-NEW-02 @slice-1
  Scenario: new replaces a literal in file and folder names too
    Given the template "acme"
    When itos-template runs with "new {template} made --stack go --answer name=blue-fox --answer module=example.com/blue/fox"
    Then the file "made/cmd/blue-fox/main.go" exists
    And the path "made/cmd/acme-widget" does not exist

  # A binary file is left byte for byte: a replacement there could corrupt it.
  @ID-NEW-03 @slice-1
  Scenario: new leaves a binary file as the template has it
    Given the template "acme"
    When itos-template runs with "new {template} made --stack go --answer name=blue-fox --answer module=example.com/blue/fox"
    Then the file "made/logo.bin" is the same as the template's "logo.bin"

  @ID-NEW-04 @slice-1
  Scenario: new merges the chosen features onto the stack and leaves the others out
    Given the template "acme"
    When itos-template runs with "new {template} made --stack go --feature cli --answer name=blue-fox --answer module=example.com/blue/fox"
    Then it exits with code 0
    And the file "made/cli.txt" exists
    And the path "made/web.txt" does not exist
    And the path "made/pyproject.toml" does not exist

  @ID-NEW-05 @slice-1
  Scenario: new leaves the manifest and the template's own files out of the project
    Given the template "acme"
    When itos-template runs with "new {template} made --stack go --answer name=blue-fox --answer module=example.com/blue/fox"
    Then it exits with code 0
    And the path "made/itos-template.yaml" does not exist
    And the path "made/.github/workflows/template.yml" does not exist

  @ID-NEW-06 @slice-1
  Scenario: the made project is a git repository whose one commit holds the render
    Given the template "acme"
    When itos-template runs with "new {template} made --stack go --feature cli --answer name=blue-fox --answer module=example.com/blue/fox"
    Then the folder "made" is a git repository with exactly 1 commit
    And the working tree of "made" has no changes

  @ID-NEW-07 @slice-1
  Scenario: the record names the template, the stack, the features, the answers and the commit of each branch rendered
    Given the template "acme"
    When itos-template runs with "new {template} made --stack go --feature cli --answer name=blue-fox --answer module=example.com/blue/fox"
    Then the record in "made" names the template as "{template}"
    And the record in "made" names the stack "go" and the features "cli"
    And the record in "made" has the answer "name" as "blue-fox" and "module" as "example.com/blue/fox"
    And the record in "made" names the commit of the template's branches "main", "stack/go" and "go/cli"

  @ID-NEW-08 @slice-1
  Scenario: new takes a template by a git URL as by a path
    Given the template "acme"
    When itos-template runs with "new file://{template} made --stack go --answer name=blue-fox --answer module=example.com/blue/fox"
    Then it exits with code 0
    And the file "made/cmd/blue-fox/main.go" exists

  @ID-NEW-09 @slice-1
  Scenario: new writes into an existing empty folder
    Given the template "acme"
    And an empty folder "made"
    When itos-template runs with "new {template} made --stack go --answer name=blue-fox --answer module=example.com/blue/fox"
    Then it exits with code 0
    And the file "made/cmd/blue-fox/main.go" exists

  @ID-NEW-10 @slice-1
  Scenario: new refuses a folder that has files in it with exit 1, naming it, and leaves it as it was
    Given the template "acme"
    And a folder "made" holding the file "keep.txt"
    When itos-template runs with "new {template} made --stack go --answer name=blue-fox --answer module=example.com/blue/fox"
    Then it exits with code 1
    And its error output says "made"
    And the folder "made" holds only "keep.txt"

  # Without a terminal nothing can be asked: every missing answer is named at
  # once, so a script's author fixes them in one go.
  @ID-NEW-11 @slice-1
  Scenario: without a terminal new refuses missing answers with exit 2, naming every one, and writes nothing
    Given the template "acme"
    When itos-template runs with "new {template} made --stack go"
    Then it exits with code 2
    And its error output says "name"
    And its error output says "module"
    And the path "made" does not exist

  @ID-NEW-12 @slice-1
  Scenario: --defaults takes a missing answer's default
    Given the template "acme"
    When itos-template runs with "new {template} made --stack go --answer name=blue-fox --defaults"
    Then it exits with code 0
    And the file "made/go.mod" contains "module example.com/you/project"

  @ID-NEW-13 @slice-1
  Scenario: --defaults still refuses a missing answer that has no default with exit 2
    Given the template "acme"
    When itos-template runs with "new {template} made --stack go --defaults"
    Then it exits with code 2
    And its error output says "name"
    And the path "made" does not exist

  @ID-NEW-14 @slice-1
  Scenario: new refuses an answer that does not match its question's pattern with exit 2, and writes nothing
    Given the template "acme"
    When itos-template runs with "new {template} made --stack go --answer name=Blue_Fox --answer module=example.com/blue/fox"
    Then it exits with code 2
    And its error output says "name"
    And the path "made" does not exist

  @ID-NEW-15 @slice-1
  Scenario: new refuses a stack the manifest does not list with exit 2, naming it
    Given the template "acme"
    When itos-template runs with "new {template} made --stack rust --answer name=blue-fox --answer module=example.com/blue/fox"
    Then it exits with code 2
    And its error output says "rust"
    And the path "made" does not exist

  # A combination the template refuses is a check saying no (docs/CLI.md's
  # exit 1), not a usage error: the names are known, only together refused.
  @ID-NEW-16 @slice-1
  Scenario: new refuses a feature of another stack with exit 1, naming both
    Given the template "acme"
    When itos-template runs with "new {template} made --stack go --feature python/cli --answer name=blue-fox --answer module=example.com/blue/fox"
    Then it exits with code 1
    And its error output says "python/cli"
    And its error output says "go"
    And the path "made" does not exist

  # A feature's dependency is never brought in unasked: what is rendered is
  # what the command line says.
  @ID-NEW-17 @slice-1
  Scenario: new refuses a feature whose needed feature is not chosen with exit 1, naming both
    Given the template "acme"
    When itos-template runs with "new {template} made --stack go --feature web --answer name=blue-fox --answer module=example.com/blue/fox"
    Then it exits with code 1
    And its error output says "web"
    And its error output says "cli"
    And the path "made" does not exist

  @ID-NEW-18 @slice-1
  Scenario: new renders a feature with the feature it needs when both are chosen
    Given the template "acme"
    When itos-template runs with "new {template} made --stack go --feature cli --feature web --answer name=blue-fox --answer module=example.com/blue/fox"
    Then it exits with code 0
    And the file "made/web.txt" exists
    And the file "made/cli.txt" exists

  @ID-NEW-19 @slice-1
  Scenario: new refuses a template git cannot reach with exit 3, naming it, and writes nothing
    When itos-template runs with "new nosuch-template made --stack go --answer name=blue-fox --answer module=example.com/blue/fox"
    Then it exits with code 3
    And its error output says "nosuch-template"
    And the path "made" does not exist

  # slice-3 (the idea scenario-gaps): what only tests outside the domain held
  # after T-9 (decision 19), each scenario reading the exit code, the rule
  # --json names (the contract, docs/CLI.md) and what the message names,
  # never a whole sentence.
  #
  # The fixture template "tangle" is built from testdata like acme: main
  # holds itos-template.yaml (the stack go, its features a, b and c, none
  # needing another, and acme's question name), notes.txt with the lines
  # one, two and three, and run.sh, executable; stack/go starts from main;
  # go/a, go/b and go/c each start from stack/go, go/a changing the line one
  # to ONE, go/b the line three to THREE, and go/c the line one to uno.

  # git merges two branches that each changed one file: a three-way merge,
  # which needs both changes kept.
  @ID-NEW-20 @slice-3
  Scenario: new merges two features that change different lines of one file, keeping both
    Given the template "tangle"
    When itos-template runs with "new {template} made --stack go --feature a --feature b --answer name=blue-fox"
    Then it exits with code 0
    And the file "made/notes.txt" contains "ONE"
    And the file "made/notes.txt" contains "THREE"

  @ID-NEW-21 @slice-3
  Scenario: new refuses features whose branches conflict with exit 1, naming the branch and the file, and writes nothing
    Given the template "tangle"
    When itos-template runs with "new {template} made --stack go --feature a --feature c --answer name=blue-fox --json"
    Then it exits with code 1
    And its JSON output names the problem "merge-conflict"
    And its error output says "go/c"
    And its error output says "notes.txt"
    And the path "made" does not exist

  # On every system, windows too, whose file system has no execute bit: the
  # fixture records run.sh as executable in git whatever the checkout's file
  # system says.
  @ID-NEW-22 @slice-3
  Scenario: the project's first commit records a file the template holds as executable as executable
    Given the template "tangle"
    When itos-template runs with "new {template} made --stack go --answer name=blue-fox"
    Then it exits with code 0
    And the first commit of "made" records "run.sh" as executable
    And the first commit of "made" records "notes.txt" as not executable

  @ID-NEW-23 @slice-3
  Scenario: the project's first commit is by whoever git says the person is
    Given the template "acme"
    When itos-template runs with "new {template} made --stack go --answer name=blue-fox --defaults"
    Then it exits with code 0
    And the first commit of "made" is by "itos-template features"

  @ID-NEW-24 @slice-3
  Scenario: new refuses a template holding a submodule with exit 1, naming it
    Given the template "acme" whose branch "stack/go" holds a submodule at "vendor/lib"
    When itos-template runs with "new {template} made --stack go --answer name=blue-fox --defaults --json"
    Then it exits with code 1
    And its JSON output names the problem "template-defect"
    And its error output says "vendor/lib"
    And the path "made" does not exist

  # Each refusal of what the command line asks for, its exit code and its
  # rule; nothing is written.
  @ID-NEW-25 @slice-3
  Scenario Outline: new refuses what the command line gets wrong with its code and its rule
    Given the template "acme"
    When itos-template runs with "new {template} made <arguments> --json"
    Then it exits with code <code>
    And its JSON output names the problem "<rule>"
    And its error output says "<named>"
    And the path "made" does not exist

    Examples:
      | arguments                                                          | code | rule                | named      |
      | --answer name=blue-fox --defaults                                  | 2    | stack-missing       | --stack    |
      | --stack rust --answer name=blue-fox --defaults                     | 2    | stack-unknown       | rust       |
      | --stack go --feature nosuch --answer name=blue-fox --defaults      | 2    | feature-unknown     | nosuch     |
      | --stack go --feature python/cli --answer name=blue-fox --defaults  | 1    | feature-other-stack | python/cli |
      | --stack go --feature web --answer name=blue-fox --defaults         | 1    | feature-needs       | cli        |
      | --stack go --answer name --defaults                                | 2    | answer-malformed    | name       |
      | --stack go --answer colour=red --answer name=blue-fox --defaults   | 2    | answer-unknown      | colour     |
      | --stack go --answer name=blue-fox --answer name=red-fox --defaults | 2    | answer-twice        | name       |
      | --stack go --answer name=Blue_Fox --defaults                       | 2    | answer-malformed    | Blue_Fox   |
      | --stack go --answer module=example.com/blue/fox                    | 2    | answer-missing      | name       |

  # Without a terminal every missing answer is a problem of its own: --json
  # lists each, as stderr gives each its line.
  @ID-NEW-26 @slice-3
  Scenario: --json lists every missing answer as a problem of its own
    Given the template "acme"
    When itos-template runs with "new {template} made --stack go --json"
    Then it exits with code 2
    And its JSON output names the problems "answer-missing" and "answer-missing"

  # What a template gets wrong, each its code and rule.
  @ID-NEW-27 @slice-3
  Scenario Outline: new refuses a template that is not one it can render with its code and its rule
    Given the template "acme" <defect>
    When itos-template runs with "new {template} made --stack go --answer name=blue-fox --defaults --json"
    Then it exits with code <code>
    And its JSON output names the problem "<rule>"
    And its error output says "<named>"
    And the path "made" does not exist

    Examples:
      | defect                                                           | code | rule                    | named               |
      | whose root branch holds no itos-template.yaml                    | 2    | manifest-missing        | itos-template.yaml  |
      | whose manifest has the key "colour"                              | 2    | manifest-invalid        | colour              |
      | whose branch "stack/go" is missing                               | 2    | manifest-branch-missing | stack/go            |
      | whose branch "stack/go" holds the file ".itos-template.yaml"     | 1    | template-defect         | .itos-template.yaml |
      | whose branch "stack/go" holds "acme-widget.md" and "blue-fox.md" | 2    | answer-name             | blue-fox.md         |

  @ID-NEW-28 @slice-3
  Scenario: new refuses a template with no commit with exit 2, saying it has no manifest to read
    Given an empty git repository "bare-template"
    When itos-template runs with "new bare-template made --stack go --answer name=blue-fox --defaults --json"
    Then it exits with code 2
    And its JSON output names the problem "manifest-missing"
    And the path "made" does not exist

  @ID-NEW-29 @slice-3
  Scenario: new refuses a folder that is a file with exit 1, naming it, and leaves it as it was
    Given the template "acme"
    And a file "made"
    When itos-template runs with "new {template} made --stack go --answer name=blue-fox --defaults --json"
    Then it exits with code 1
    And its JSON output names the problem "folder-not-empty"
    And its error output says "made"
    And the file "made" exists

  @ID-NEW-30 @slice-3
  Scenario: new refuses to run without git with exit 3, saying to install it
    Given the template "acme"
    When itos-template runs with no git on the PATH with "new {template} made --stack go --answer name=blue-fox --defaults --json"
    Then it exits with code 3
    And its JSON output names the problem "git-missing"
    And the path "made" does not exist

  # git guesses an identity from the machine where it can, which differs by
  # system; "git knowing no one" sets user.useConfigOnly, so git refuses to
  # guess everywhere.
  @ID-NEW-31 @slice-3
  Scenario: new refuses to commit when git knows no one with exit 3, saying how to tell it
    Given the template "acme"
    When itos-template runs with git knowing no one with "new {template} made --stack go --answer name=blue-fox --defaults --json"
    Then it exits with code 3
    And its JSON output names the problem "git-identity"
    And its error output says "user.name"
    And the path "made" does not exist

  # bug-1, found by T-12's property TestCaseFormsAreDistinct: a case-forms
  # literal must have five different forms, or two of them map one string to
  # two answers and the first wins, a Pascal use rendering in camel case with
  # no warning. A first word starting with a digit makes camel and Pascal
  # one string; words of digits alone make snake and upper snake one too.
  # The manifest checks the promise itself, so a later form is held to it
  # as well, and says which forms collide and how to fix the literal.
  @ID-NEW-32 @bug-1
  Scenario Outline: new refuses a case-forms literal whose forms are not five different strings with exit 2, naming them
    Given the template "acme" whose question "name" has the literal "<literal>"
    When itos-template runs with "new {template} made --stack go --answer name=blue-fox --defaults"
    Then it exits with code 2
    And its error output says "<literal>"
    And its error output says "<collides>"
    And the path "made" does not exist

    Examples:
      | literal  | collides |
      | 2fa-code | 2faCode  |
      | 1-2      | 1_2      |

  # bug-2, found by T-12's property TestRenderIsDeterministic: render.Plan
  # walked a map, so when answers make a file's name the folder of others,
  # which clash the refusal named varied from run to run, and decision 3
  # needs a render, a refusal included, the same every time. It names every
  # clash, in path order, as new names every missing answer at once.
  @ID-NEW-33 @bug-2
  Scenario: new refuses answers that name a file as a folder with exit 2, naming every clash
    Given the template "acme" whose branch "stack/go" holds the files "acme-widget", "blue-fox/a.txt" and "blue-fox/b.txt"
    When itos-template runs with "new {template} made --stack go --answer name=blue-fox --defaults"
    Then it exits with code 2
    And its error output says "blue-fox/a.txt"
    And its error output says "blue-fox/b.txt"
    And the path "made" does not exist

  # slice-5 (strict-yaml; docs/CONFIG.md rule 1, decision 22): a manifest is
  # JSON data written as YAML, and comes from any template git can clone, so
  # it is read strictly. What JSON cannot say is refused as a manifest
  # problem, exit 2, naming what was found: a custom tag can make a reader
  # build any object, an alias can expand without bound, a merge key's
  # overrides surprise, a second document is not read, and a key given twice
  # keeps one value silently. What JSON can say reads the same however it is
  # written, so a tool may write JSON into itos-template.yaml. The record,
  # .itos-template.yaml, is read by the same reader once a command reads it
  # (update, adopt). Each variant is a file of features/testdata, acme's
  # manifest with the one change its row names.
  @ID-NEW-34 @slice-5
  Scenario Outline: new refuses a manifest using what JSON cannot say with exit 2, naming it
    Given the template "acme" whose manifest is acme's <change>
    When itos-template runs with "new {template} made --stack go --answer name=blue-fox --defaults"
    Then it exits with code 2
    And its error output says "itos-template.yaml"
    And its error output says "<named>"
    And the path "made" does not exist

    Examples:
      | change                                                                   | named    |
      | with the custom tag !foo before the literal of the question name         | !foo     |
      | with the anchor &shared on the stack go's checks and *shared in python's | shared   |
      | with a merge key, <<, bringing the stack go's keys into python's         | <<       |
      | followed by a second document, after a line ---                          | document |

  # yaml.v3 already refuses a key given twice, and reads a manifest written
  # as JSON, so ID-NEW-36 and ID-NEW-35 hold before slice-5; they stay
  # scenarios so the strict reader keeps them, and are the two of the slice
  # that show no red first.
  @ID-NEW-36 @slice-5
  Scenario: new refuses a manifest giving a key twice with exit 2, naming it
    Given the template "acme" whose manifest is acme's with the key stacks given twice
    When itos-template runs with "new {template} made --stack go --answer name=blue-fox --defaults"
    Then it exits with code 2
    And its error output says "stacks"
    And the path "made" does not exist

  @ID-NEW-35 @slice-5
  Scenario: new reads a manifest written as JSON as the same manifest
    Given the template "acme" whose manifest is acme's written as JSON
    When itos-template runs with "new {template} made --stack go --feature cli --answer name=blue-fox --defaults"
    Then it exits with code 0
    And the file "made/cmd/blue-fox/main.go" exists
    And the file "made/cli.txt" exists
    And the record in "made" names the stack "go" and the features "cli"

  # slice-6 (new-first-commit-rules): new commits the render as the made
  # project's first commit, and a template that holds its projects to commit
  # rules (itos, or any commit lint) judges that commit on the project's
  # first push, when its CI has no green run to start from: today's message,
  # a chore with no footer, fails a rule that asks a chore for a Task footer.
  # The template knows its own rules, so its manifest gives the message, from
  # version 4: first_commit, the whole message (its header, a body and its
  # footers), the literals in it replaced by the answers as a file's contents
  # are. The person's call, 2026-10-07, over itos starting a project's rules
  # after its root commit, which would need itos to change and would help
  # itos's rules alone. A message with no header line refuses the manifest.
  # Without first_commit the message stays today's, so a template written
  # for an earlier version renders as it did.
  @ID-NEW-37 @slice-6
  Scenario: the manifest's first commit message, its literals replaced, is the made project's first commit's
    Given the template "acme" whose manifest gives the first commit the message "chore: start acme-widget" with the footer "Task: T-1"
    When itos-template runs with "new {template} made --stack go --answer name=blue-fox --defaults"
    Then it exits with code 0
    And the first commit of "made" has the header "chore: start blue-fox"
    And the first commit of "made" has the footer "Task: T-1"

  # The message today's slice-1 gives; it holds before slice-6, and stays so
  # a template without first_commit keeps rendering as it did.
  @ID-NEW-38 @slice-6
  Scenario: without a first commit message in the manifest the made project's first commit keeps its message
    Given the template "acme"
    When itos-template runs with "new {template} made --stack go --answer name=blue-fox --defaults"
    Then it exits with code 0
    And the first commit of "made" has the header "chore: make the project from its template"

  @ID-NEW-39 @slice-6
  Scenario: new refuses a manifest whose first commit message has no header with exit 2
    Given the template "acme" whose manifest gives the first commit the message "" with the footer "Task: T-1"
    When itos-template runs with "new {template} made --stack go --answer name=blue-fox --defaults"
    Then it exits with code 2
    And its error output says "first_commit"
    And the path "made" does not exist

  # bug-3, found with slice-6: new committed first_commit with git commit -m,
  # which follows the person's commit.cleanup; set to strip, git dropped each
  # line starting with #, so the message the template's commit rules judged
  # differed from the manifest's on one machine and not another. new gives
  # git the cleanup itself, keeping every line but trailing whitespace.
  @ID-NEW-40 @bug-3 @wip
  Scenario: the first commit keeps a line starting with # whatever git's commit.cleanup says
    Given the template "acme" whose manifest gives the first commit the message "chore: start acme-widget" with the body line "# Notes" and the footer "Task: T-1"
    When itos-template runs with git's commit.cleanup set to strip with "new {template} made --stack go --answer name=blue-fox --defaults"
    Then it exits with code 0
    And the first commit of "made" has the body line "# Notes"
