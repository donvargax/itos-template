@phase-1
Feature: check renders every combination a template allows and runs its checks
  itos-template check [<template>] proves a template (PLAN.md, principle 5):
  it renders every combination the manifest allows, each as new renders it
  (decisions 2, 8 and 12), and runs that render's checks in it. The template
  is anything git clone takes (decision 9), the repository check runs in
  when none is named; check renders the template's branch heads, as new
  does, never its working tree (slice-7, below, says how it reads a CI
  checkout's).

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

  A render's scan for leftover literals, and the mark of a check that scans
  it for credentials, are slice-4's, below.

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

  # bug-4: the same message as new's, built in one place (internal/cli/codes.go), so the
  # credential a URL's userinfo holds would reach check's error output too. Same closed
  # local port, so no network and no token leaves the machine.
  @ID-CHECK-23 @bug-4 @wip
  Scenario: a template URL's credential never reaches check's error output
    When itos-template runs with "check https://x-access-token:ghp_EXAMPLETOKENNOTREAL@127.0.0.1:1/acme.git --answer name=blue-fox --defaults"
    Then it exits with code 3
    And its error output does not say "ghp_EXAMPLETOKENNOTREAL"
    And its error output says "127.0.0.1:1/acme.git"

  # slice-3 (the idea scenario-gaps): a check naming a program nothing can
  # start fails its combination as any failed check does, its output saying
  # it cannot run.
  @ID-CHECK-09 @slice-3
  Scenario: a check whose program cannot be started fails its combination, saying it cannot run it
    Given the template "acme" whose feature "web" of the stack "go" has the check "itos-template-no-such-program"
    When itos-template runs with "check {template} --answer name=blue-fox --defaults"
    Then it exits with code 1
    And its report says "go + cli + web" failed at "itos-template-no-such-program"
    And its report says "cannot run itos-template-no-such-program"
    And its report says "go + cli" passed

  # slice-4 (template-scans; decision 21): check scans every render for the
  # template's literals left in a form no answer replaces, and warns when no
  # check of the template scans the renders for credentials.
  #
  # A leftover is a literal's words, in order and in any case, joined by
  # nothing, a space, ".", "-", "_" or "/", found in a text file's contents
  # or in a path of a render after its answers replaced the five case forms:
  # "Acme Widget" in a heading, "acmewidget" in a host name. The person's
  # call, 2026-10-07. Each leftover fails its combination, the report naming
  # the path, the line and the text found, so the author either writes that
  # spot in one of the five forms or names it otherwise. A literal without
  # case forms is looked for as written. Binary files are not read, as no
  # answer replaces anything in them.
  #
  # The credential scan stays the template's own (decision 21): a check is
  # marked as one with scans: [credentials], in manifest version 3's long
  # form of a check, {run: [words…], scans: [what it scans]}, beside the list
  # of words, which stays. The person's call, 2026-10-07: itos-template knows
  # what a check is for, never which tools exist; docs/manifest.md suggests
  # gitleaks. scans takes credentials alone for now; any other value is
  # refused. The warning goes to the error output, never fails the run, and
  # names no tool.
  @ID-CHECK-10 @slice-4
  Scenario: a literal left in a form no answer replaces fails its combination, naming where
    Given the template "acme" whose branch "stack/go" holds the file "docs/title.md" with the line "# Acme Widget"
    When itos-template runs with "check {template} --answer name=blue-fox --defaults"
    Then it exits with code 1
    And its report says "go" failed with the leftover "Acme Widget" at "docs/title.md:1"
    And its report says "python" passed

  @ID-CHECK-11 @slice-4
  Scenario: a literal left in a path fails its combination, naming the path
    Given the template "acme" whose branch "stack/go" holds the file "docs/acme.widget.md" with the line "notes"
    When itos-template runs with "check {template} --answer name=blue-fox --defaults"
    Then it exits with code 1
    And its report says "go" failed with the leftover "acme.widget" at "docs/acme.widget.md"

  @ID-CHECK-12 @slice-4
  Scenario: check warns when no check is marked as scanning for credentials, and still passes
    Given the template "acme"
    When itos-template runs with "check {template} --answer name=blue-fox --defaults"
    Then it exits with code 0
    And its error output says "warning"
    And its error output says "scans: [credentials]"

  @ID-CHECK-13 @slice-4
  Scenario: a check marked as scanning for credentials runs as any check, and check gives no warning
    Given the template "acme" whose root has, after its own, the check "git ls-files" marked as scanning "credentials"
    When itos-template runs with "check {template} --answer name=blue-fox --defaults"
    Then it exits with code 0
    And its report shows the checks of "go" in order: "git ls-files --error-unmatch README.md", "git ls-files", "git ls-files --error-unmatch cmd/blue-fox/main.go"
    And its error output does not say "warning"

  @ID-CHECK-14 @slice-4
  Scenario: check refuses a check marked as scanning something the format does not know with exit 2
    Given the template "acme" whose root has, after its own, the check "git ls-files" marked as scanning "licences"
    When itos-template runs with "check {template} --answer name=blue-fox --defaults"
    Then it exits with code 2
    And its error output says "licences"

  # slice-7 (check-ci-branches): a template's CI checks out its repository
  # as git clone does, its default branch local and the others only as
  # origin's remote-tracking branches, and a step may run in any folder of
  # it. check with no template named reads the repository it runs in, from
  # its top whatever folder it runs in, and each branch the manifest names
  # from the local branch of that name, else from origin's, so no fetch
  # recipe is needed. The person's call, 2026-10-07. Before slice-7, check
  # in such a clone said the template has no branch stack/go (exit 1), and
  # in a subfolder that git cannot reach the template "." (exit 3).
  @ID-CHECK-15 @slice-7
  Scenario: check in a clone holding only its default branch reads the others from origin
    Given the template "acme"
    And a clone "ci" of the template, only its default branch local
    When itos-template runs in the folder "ci" with "check --answer name=blue-fox --defaults"
    Then it exits with code 0
    And its report says "go + cli + web" passed
    And its report says "python + cli" passed

  @ID-CHECK-16 @slice-7
  Scenario: check run in a folder below the template's top checks the whole template
    Given the template "acme"
    And a clone "ci" of the template, only its default branch local
    When itos-template runs in the folder "ci/.github" with "check --answer name=blue-fox --defaults"
    Then it exits with code 0
    And its report says "go + cli" passed

  # slice-8 (check-detached-head): slice-7's root lookup, checked by hand
  # then, and found uncovered by itos-cc's code proof (T-20: rootOf's nine
  # mutants). With no template named, check reads the manifest from the root
  # branch: the branch the checkout's HEAD names; a detached HEAD
  # (actions/checkout on a pull request) names none, so the branch origin's
  # HEAD names, as git clone records it, else as origin says when asked (git
  # ls-remote, never waiting for a password), since actions/checkout records
  # none. When none answers, the template has no default branch (exit 2). A
  # bare repository is read as a checkout is. The behaviour is slice-7's:
  # these scenarios pass from the start, and itos-cc's mutation run shows each
  # can fail.
  @ID-CHECK-17 @slice-8
  Scenario: check in a clone whose HEAD is detached reads the root from the branch origin's HEAD names
    Given the template "acme"
    And a clone "ci" of the template, only its default branch local
    And the clone "ci" has its HEAD detached
    When itos-template runs in the folder "ci" with "check --answer name=blue-fox --defaults"
    Then it exits with code 0
    And its report says "go + cli + web" passed

  @ID-CHECK-18 @slice-8
  Scenario: check in a checkout as actions/checkout leaves a pull request's asks origin for the root
    Given the template "acme"
    And a clone "ci" of the template, only its default branch local
    And the clone "ci" has its HEAD detached, no local branch and no record of origin's HEAD
    When itos-template runs in the folder "ci" with "check --answer name=blue-fox --defaults"
    Then it exits with code 0
    And its report says "python + cli" passed

  @ID-CHECK-19 @slice-8
  Scenario: check refuses with exit 2 a detached checkout whose origin cannot say its root
    Given the template "acme"
    And a clone "ci" of the template, only its default branch local
    And the clone "ci" has its HEAD detached, no local branch and no record of origin's HEAD
    And the clone "ci" has an origin that cannot be reached
    When itos-template runs in the folder "ci" with "check --answer name=blue-fox --defaults"
    Then it exits with code 2
    And its error output says "no default branch"

  @ID-CHECK-20 @slice-8
  Scenario: check in a bare clone of the template checks the whole template
    Given the template "acme"
    And a bare clone "ci.git" of the template
    When itos-template runs in the folder "ci.git" with "check --answer name=blue-fox --defaults"
    Then it exits with code 0
    And its report says "go + cli" passed

  # What git clone recorded is read before origin is asked: a detached
  # checkout whose origin cannot be reached still has its root.
  @ID-CHECK-21 @slice-8
  Scenario: check in a clone whose HEAD is detached reads the root git clone recorded, never asking origin
    Given the template "acme"
    And a clone "ci" of the template, only its default branch local
    And the clone "ci" has its HEAD detached
    And the clone "ci" has an origin that cannot be reached
    When itos-template runs in the folder "ci" with "check --answer name=blue-fox --defaults"
    Then it exits with code 0
    And its report says "go + cli + web" passed

  # An origin that answers but whose HEAD names no branch (a detached HEAD
  # there) says no root either.
  @ID-CHECK-22 @slice-8
  Scenario: check refuses with exit 2 a detached checkout whose origin's HEAD names no branch
    Given the template "acme"
    And a clone "ci" of the template, only its default branch local
    And the clone "ci" has its HEAD detached, no local branch and no record of origin's HEAD
    And the clone "ci" has an origin whose HEAD names no branch
    When itos-template runs in the folder "ci" with "check --answer name=blue-fox --defaults"
    Then it exits with code 2
    And its error output says "no default branch"
