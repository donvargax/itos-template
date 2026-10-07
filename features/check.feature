@phase-1
Feature: check renders every combination a template allows and runs its checks
  itos-template check [<template>] proves a template (PLAN.md, principle 5):
  it renders every combination the manifest allows, each as new renders it
  (decisions 2, 8 and 12), and runs that render's checks in it. The template
  is anything git clone takes (decision 9), the folder check runs in when
  none is named; check renders the template's branch heads, as new does, so
  a CI checkout fetches the template's branches first.

  The combinations are derived, never listed (the user's call, 2026-10-07):
  each stack alone, and each stack with every set of its features in which
  every feature's needs are chosen too. A combination the template cannot
  support is listed under the manifest's unsupported key, by its stack and
  its exact features; check neither renders nor checks it, and new refuses
  it with exit 1, naming it, as a combination the template refuses
  (docs/CLI.md). An unsupported entry naming no combination the manifest
  allows is a manifest refused. The cost is known: each feature that needs
  no other doubles a stack's combinations (3 make 8 renders, 10 make 1,024),
  each built and checked; the template keeps its features few, as PLAN.md's
  principles ask, and lists what it cannot support.

  A check is a command written as a list of words, run with no shell (the
  user's call, 2026-10-07), so it means the same on Linux, macOS and Windows;
  a template that wants a shell writes ["sh", "-c", "…"] itself. Checks live
  in the manifest beside what they check: checks on its top for the root,
  and checks on each stack and each feature. A combination's checks are the
  root's, then its stack's, then its features' in the order the manifest
  lists the features, each run in the render's top folder; each word has the
  literals replaced by the answers, as a file's contents, so a check can
  name a file the answers renamed. The manifest's version 2 adds checks and
  unsupported; version 1 stays read as it is, a template with no checks.

  check never asks: it is a template's CI step. The answers come from
  --answer, and with --defaults from the questions' defaults; a missing one
  is refused with exit 2, each named, before anything is rendered. Every
  combination is checked, whatever failed before it; its report says, per
  combination, each check run as it ran (the answers in place) and whether
  it passed, a failed check followed by its output, and check exits 1 when
  any check or render failed, 0 when all passed. Each render is made in a
  temporary folder and removed after its checks.

  The scans of a render for leftover literals and leaked credentials are the
  item template-scans, after this slice.

  The fixture "acme" (new.feature) takes manifest version 2 and these
  checks, each passing on every render of it:
    the root      git ls-files --error-unmatch README.md
    stack go      git ls-files --error-unmatch cmd/acme-widget/main.go
    go/cli        git ls-files --error-unmatch cli.txt
    stack python  git ls-files --error-unmatch pyproject.toml
  The checks run git, which is wherever the scenarios run, never a shell.

  @ID-CHECK-01 @slice-2
  Scenario: check renders every combination the template allows and exits 0 when every check passes
    Given the template "acme"
    When itos-template runs with "check {template} --answer name=blue-fox --defaults"
    Then it exits with code 0
    And its report says "go" passed
    And its report says "go + cli" passed
    And its report says "go + cli + web" passed
    And its report says "python" passed
    And its report says "python + cli" passed
    And its report names no other combination

  # The stack's check names cmd/acme-widget/main.go: it passes only because
  # its words had the literal replaced by the answer, as the render's files.
  @ID-CHECK-02 @slice-2
  Scenario: a combination runs the root's checks, then its stack's, then its features', the answers in place of the literals
    Given the template "acme"
    When itos-template runs with "check {template} --answer name=blue-fox --defaults"
    Then it exits with code 0
    And its report shows the checks of "go + cli" in order: "git ls-files --error-unmatch README.md", "git ls-files --error-unmatch cmd/blue-fox/main.go", "git ls-files --error-unmatch cli.txt"

  @ID-CHECK-03 @slice-2
  Scenario: a failing check fails its combination with exit 1 and the other combinations are still checked
    Given the template "acme" whose feature "web" of the stack "go" has the check "git ls-files --error-unmatch missing.txt"
    When itos-template runs with "check {template} --answer name=blue-fox --defaults"
    Then it exits with code 1
    And its report says "go + cli + web" failed at "git ls-files --error-unmatch missing.txt"
    And its report says "go + cli" passed
    And its report says "python + cli" passed

  @ID-CHECK-04 @slice-2
  Scenario: check leaves out a combination the manifest lists as unsupported
    Given the template "acme" whose manifest lists the stack "go" with the features "cli" and "web" as unsupported
    When itos-template runs with "check {template} --answer name=blue-fox --defaults"
    Then it exits with code 0
    And its report says "go + cli" passed
    And its report does not name "go + cli + web"

  @ID-CHECK-05 @slice-2
  Scenario: new refuses a combination the manifest lists as unsupported with exit 1, naming it, and writes nothing
    Given the template "acme" whose manifest lists the stack "go" with the features "cli" and "web" as unsupported
    When itos-template runs with "new {template} made --stack go --feature cli --feature web --answer name=blue-fox --defaults"
    Then it exits with code 1
    And its error output says "web"
    And the path "made" does not exist

  @ID-CHECK-06 @slice-2
  Scenario: check refuses missing answers with exit 2, naming every one, before rendering anything
    Given the template "acme"
    When itos-template runs with "check {template}"
    Then it exits with code 2
    And its error output says "name"
    And its error output says "module"
    And its report names no combination

  @ID-CHECK-07 @slice-2
  Scenario: check with no template named checks the repository it runs in
    Given the template "acme"
    When itos-template runs in the template's folder with "check --answer name=blue-fox --defaults"
    Then it exits with code 0
    And its report says "go + cli" passed

  @ID-CHECK-08 @slice-2
  Scenario: check refuses a template git cannot reach with exit 3, naming it
    When itos-template runs with "check nosuch-template --answer name=blue-fox --defaults"
    Then it exits with code 3
    And its error output says "nosuch-template"

  # slice-3 (the idea scenario-gaps): a check naming a program nothing can
  # start fails its combination as any failed check does, its output saying
  # it cannot run.
  @ID-CHECK-09 @slice-3 @wip
  Scenario: a check whose program cannot be started fails its combination, saying it cannot run it
    Given the template "acme" whose feature "web" of the stack "go" has the check "itos-template-no-such-program"
    When itos-template runs with "check {template} --answer name=blue-fox --defaults"
    Then it exits with code 1
    And its report says "go + cli + web" failed at "itos-template-no-such-program"
    And its report says "cannot run itos-template-no-such-program"
    And its report says "go + cli" passed
