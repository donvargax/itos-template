@phase-1
Feature: release tags every branch of a template with one version once check passes
  itos-template release <version> runs in a template's repository and cuts
  a release as decision 36 defines one: every branch the manifest lists
  carries the tag <branch>/<version> (main/v1.2.0, stack/go/v1.2.0,
  go/cli/v1.2.0), a branch name holding slashes and the version the tag's
  last path segment. It takes check's --answer and --defaults, and a
  missing answer is refused with exit 2 before anything is rendered.

  It runs check over the heads check reads when no template is named (each
  listed branch's local head, else origin's; check.feature) and tags exactly
  those commits, only once every combination passes; a failure is check's
  report and exit 1, no tag made. It tags commits, never the working tree,
  which it neither renders nor judges (q-53).

  A release is for others to fetch: release needs an origin, and refuses,
  naming it, any listed branch whose local head differs from origin's,
  ahead or behind, before running check. A branch with no local head is
  origin's already (q-53).

  The version is vX.Y.Z, with an optional pre-release (-rc.1); anything
  else is a usage error, exit 2. A version any listed branch already
  carries as <branch>/<version>, a partial release included, is refused
  with exit 1, naming the tag. A version lower than the newest is a
  backport and is cut; a pre-release too. Neither becomes the default new
  renders, since the newest is the highest stable complete release
  (decision 36, q-52).

  The tags are annotated, each tag's message naming the version (q-51). They
  are made in the template's repository and never pushed: release prints the
  one command that publishes them all, git push --atomic origin and each
  tag, and publishing is the person's act (q-50). Nothing is half-made: a
  refusal or a failure leaves no tag, and a tag git refuses midway takes
  back those made before it.

  With --json, stdout is one object with schema: 1 and ok, version, tags
  (name, branch, commit), push (the command's words), report (check's
  combinations and their outcome) and problems. The human message and fix
  fields are not a machine contract (docs/CLI.md).

  # slice-14 (release): q-50 through q-53, answered by the person 2026-10-10.
  # "A clone of the template, every branch local" is a clone whose listed
  # branches each have a local branch at origin's head, origin the fixture;
  # release runs in it as a template's CI or its author would. acme lists
  # main, stack/go, go/cli, go/web, stack/python and python/cli.
  Background:
    Given the template "acme"
    And a clone "tpl" of the template, every branch local

  @ID-RELEASE-01 @slice-14 @wip
  Scenario: release tags every listed branch at its proved head with annotated tags, pushing none
    When itos-template runs in the folder "tpl" with "release v1.0.0 --answer name=blue-fox --defaults"
    Then it exits with code 0
    And in "tpl" every branch the template lists carries the annotated tag "<branch>/v1.0.0" at its head, its message naming "v1.0.0"
    And the template has no tag "main/v1.0.0"
    And its standard output says "git push --atomic origin"
    And its standard output says "stack/go/v1.0.0"

  @ID-RELEASE-02 @slice-14 @wip
  Scenario: what release cuts, once pushed, is the release new renders by default
    Given itos-template ran in the folder "tpl" with "release v1.0.0 --answer name=blue-fox --defaults"
    And the tags of "tpl" pushed to its origin
    When itos-template runs with "new {template} made --stack go --feature cli --answer name=blue-fox --defaults"
    Then it exits with code 0
    And the record in "made" names the release "v1.0.0"

  @ID-RELEASE-03 @slice-14 @wip
  Scenario: a failing combination makes no tag and exits 1 with check's report
    Given the template "acme" whose feature "web" of the stack "go" has the check "git ls-files --error-unmatch missing.txt"
    And a clone "tpl" of the template, every branch local
    When itos-template runs in the folder "tpl" with "release v1.0.0 --answer name=blue-fox --defaults"
    Then it exits with code 1
    And its report says "go + cli + web" failed at "git ls-files --error-unmatch missing.txt"
    And "tpl" has no tag ending "/v1.0.0"

  @ID-RELEASE-04 @slice-14 @wip
  Scenario Outline: a version a listed branch already carries is refused, naming the tag, and no tag is made
    Given in "tpl" the branch "<branch>" carries the tag "<branch>/v1.0.0"
    When itos-template runs in the folder "tpl" with "release v1.0.0 --answer name=blue-fox --defaults"
    Then it exits with code 1
    And its error output says "<branch>/v1.0.0"
    And in "tpl" no branch but "<branch>" carries a tag ending "/v1.0.0"

    Examples:
      | branch |
      | main   |
      | go/cli |

  @ID-RELEASE-05 @slice-14 @wip
  Scenario Outline: a version that is not vX.Y.Z with an optional pre-release is a usage error
    When itos-template runs in the folder "tpl" with "release <version> --answer name=blue-fox --defaults"
    Then it exits with code 2
    And its error output says "<version>"
    And "tpl" has no tag ending "/<version>"

    Examples:
      | version |
      | 1.0.0   |
      | v1.0    |
      | latest  |

  @ID-RELEASE-06 @slice-14 @wip
  Scenario: a backport below the newest release is cut, and new's default stays the newest
    Given the template "acme" released as "v1.0.0", "v1.1.0" and "v1.2.0-rc.1"
    And a clone "tpl" of the template, every branch local
    When itos-template runs in the folder "tpl" with "release v1.0.1 --answer name=blue-fox --defaults"
    Then it exits with code 0
    And in "tpl" every branch the template lists carries the annotated tag "<branch>/v1.0.1" at its head, its message naming "v1.0.1"
    And after the tags of "tpl" are pushed to its origin, new renders the release "v1.1.0" by default

  @ID-RELEASE-07 @slice-14 @wip
  Scenario: a pre-release is cut, and new's default stays the newest stable release
    Given the template "acme" released as "v1.0.0", "v1.1.0" and "v1.2.0-rc.1"
    And a clone "tpl" of the template, every branch local
    When itos-template runs in the folder "tpl" with "release v1.3.0-rc.1 --answer name=blue-fox --defaults"
    Then it exits with code 0
    And in "tpl" every branch the template lists carries the annotated tag "<branch>/v1.3.0-rc.1" at its head, its message naming "v1.3.0-rc.1"
    And after the tags of "tpl" are pushed to its origin, new renders the release "v1.1.0" by default

  @ID-RELEASE-08 @slice-14 @wip
  Scenario Outline: a listed branch whose local head is <how> origin's is refused, naming it, before check runs
    Given in "tpl" the branch "stack/go" is <how> origin's
    When itos-template runs in the folder "tpl" with "release v1.0.0 --answer name=blue-fox --defaults"
    Then it exits with code 1
    And its error output says "stack/go"
    And its report checks no combination
    And "tpl" has no tag ending "/v1.0.0"

    Examples:
      | how         |
      | ahead of    |
      | behind      |
      | diverged from |

  @ID-RELEASE-09 @slice-14 @wip
  Scenario: a listed branch with no local head is tagged at origin's head
    Given a clone "ci" of the template, only its default branch local
    When itos-template runs in the folder "ci" with "release v1.0.0 --answer name=blue-fox --defaults"
    Then it exits with code 0
    And in "ci" the tag "stack/go/v1.0.0" is annotated and names the commit of origin's "stack/go"

  @ID-RELEASE-10 @slice-14 @wip
  Scenario: missing answers are refused with exit 2, each named, before anything is rendered or tagged
    When itos-template runs in the folder "tpl" with "release v1.0.0"
    Then it exits with code 2
    And its error output says "name"
    And its report checks no combination
    And "tpl" has no tag ending "/v1.0.0"

  @ID-RELEASE-11 @slice-14 @wip
  Scenario: release tags commits, never the working tree, whatever the tree holds
    Given in "tpl" an uncommitted change to "README.md" and an untracked file "notes.txt"
    When itos-template runs in the folder "tpl" with "release v1.0.0 --answer name=blue-fox --defaults"
    Then it exits with code 0
    And in "tpl" every branch the template lists carries the annotated tag "<branch>/v1.0.0" at its head, its message naming "v1.0.0"
    And in "tpl" the change to "README.md" and the file "notes.txt" are still there, uncommitted

  @ID-RELEASE-12 @slice-14 @wip
  Scenario: with --json a release is one object naming each tag, its branch and its commit, and the push
    When itos-template runs in the folder "tpl" with "release v1.0.0 --answer name=blue-fox --defaults --json"
    Then it exits with code 0
    And its release JSON is ok, version "v1.0.0", with a tag for each branch the template lists at its head
    And its release JSON push begins "git", "push", "--atomic", "origin"

  @ID-RELEASE-13 @slice-14 @wip
  Scenario: with --json a failing combination is ok false, names no tag, and keeps check's report
    Given the template "acme" whose feature "web" of the stack "go" has the check "git ls-files --error-unmatch missing.txt"
    And a clone "tpl" of the template, every branch local
    When itos-template runs in the folder "tpl" with "release v1.0.0 --answer name=blue-fox --defaults --json"
    Then it exits with code 1
    And its release JSON is not ok, with no tag, and its report says "go + cli + web" failed

  @ID-RELEASE-14 @slice-14 @wip
  Scenario Outline: release refuses <where>, naming <problem>
    Given <where>
    When itos-template runs in the folder "<dir>" with "release v1.0.0 --answer name=blue-fox --defaults"
    Then it exits with code <code>
    And its error output says "<problem>"

    Examples:
      | where                                        | dir   | code | problem |
      | a copy "plain" of the template with no origin | plain | 1    | origin  |
      | a folder "loose" outside any git repository   | loose | 3    | git     |
