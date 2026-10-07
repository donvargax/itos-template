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

  @ID-NEW-05 @slice-1 @wip
  Scenario: new leaves the manifest and the template's own files out of the project
    Given the template "acme"
    When itos-template runs with "new {template} made --stack go --answer name=blue-fox --answer module=example.com/blue/fox"
    Then the path "made/itos-template.yaml" does not exist
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
