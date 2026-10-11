@phase-1
Feature: update brings a made project to a newer template release by a 3-way merge
  update runs at a made or adopted project's git root and reads its record,
  .itos-template.yaml (decision 10). It renders the old render again from the
  record's branch commits, never from a tag, which may have moved since, nor
  from the project's first commit; then the target release, with the record's
  stack, features and answers; and merges the difference into the project
  with git's own merge, the old render the base (decision 3, q-49). Renames
  follow as git's merge follows them. Both renders take new's substitutions
  and leave out the template-only paths, as new's does, and the paths the
  record declines; update runs no check and no setup step.

  The target is the newest release (decision 36: the highest stable complete
  one), or the version --to names, a pre-release included. A target equal to
  the recorded release changes nothing and exits 0; an older one is refused,
  since update only goes forward. A --to no release has is a usage error.

  update writes the merged files and the new record to the working tree and
  the index, staged, and commits nothing (q-46): the person commits through
  the project's own gates, with whatever footers its rules ask. It prints a
  message naming the old and new releases for that commit. HEAD never moves.

  A conflict is left as git merge leaves one (q-47): markers in the file,
  unmerged index entries holding the old render, the project and the new
  render, everything else merged and staged, the record naming the new
  release, exit 1 naming each conflicted path. update keeps no state of its
  own, so there is no --abort and no --continue; since it starts only from a
  clean tree, git reset --hard undoes it, which the message names.

  Only a question the new release adds is answered (q-48), as new answers
  one: --answer, a terminal, --defaults, refused without one. An --answer for
  a question the record already answers is a usage error: changing an answer,
  the stack or the features is not update's.

  Writing requires a non-bare git repository with a committed HEAD, a record,
  and a clean index and working tree, non-ignored untracked files included.

  With --json, stdout is one object with schema: 1 and ok, from, to (each the
  release and its branch commits), changed (path and kind: added, modified,
  removed, renamed), conflicts (path) and problems. A conflicted update is
  ok: false with the problem "merge-conflict". The human message and fix
  fields are not a machine contract (docs/CLI.md).

  # slice-13 (update): q-46 through q-49, answered by the person 2026-10-10.
  # The fixture template changes release.txt in every release (releases_test.go),
  # which is what a clean update changes and what a project edit to it conflicts
  # with. A variant of the fixture adds what a scenario needs to its v1.1.0
  # only. A project "made by new" runs new as the person would, --ref, --stack,
  # --feature, --answer name and --defaults (so its module is the default), and
  # is committed as new commits it. The snapshot steps may share adopt.feature's
  # (slice-12).
  Background:
    Given the template "acme" released as "v1.0.0", "v1.1.0" and "v1.2.0-rc.1"
    And a project in "made" made by new from the release "v1.0.0" with stack "go", features "cli" and name "blue-fox"

  @ID-UPDATE-01 @slice-13
  Scenario: update merges the newest release in, keeping the project's changes, and commits nothing
    Given a committed project change to "made/README.md" containing "private-project-content"
    And a committed file "made/own.txt" containing "project-only"
    And a saved snapshot of the HEAD of "made"
    When itos-template runs in "made" with "update"
    Then it exits with code 0
    And the file "made/release.txt" contains "v1.1.0"
    And the file "made/README.md" contains "private-project-content"
    And the file "made/own.txt" contains "project-only"
    And the HEAD of "made" is unchanged
    And "made" has staged changes to "release.txt" and ".itos-template.yaml" only, and nothing unstaged
    And the record in "made" names the release "v1.1.0" and its exact branch commits
    And its standard output says "v1.0.0"
    And its standard output says "v1.1.0"

  @ID-UPDATE-02 @slice-13
  Scenario Outline: --to names the release update brings the project to, a pre-release too
    When itos-template runs in "made" with "update --to <ref>"
    Then it exits with code 0
    And the file "made/release.txt" contains "<ref>"
    And the record in "made" names the release "<ref>" and its exact branch commits

    Examples:
      | ref         |
      | v1.1.0      |
      | v1.2.0-rc.1 |

  @ID-UPDATE-03 @slice-13
  Scenario: update keeps the record's template, stack, features and answers
    When itos-template runs in "made" with "update"
    Then it exits with code 0
    And the record in "made" names the template as "{template}"
    And the record in "made" names the stack "go" and the features "cli"
    And the record in "made" has the answer "name" as "blue-fox" and "module" as "example.com/you/project"

  @ID-UPDATE-04 @slice-13
  Scenario: a project already at the target release is told so and nothing is written
    Given a saved snapshot of "made"
    When itos-template runs in "made" with "update --to v1.0.0"
    Then it exits with code 0
    And its standard output says "v1.0.0"
    And the snapshot of "made" is unchanged

  @ID-UPDATE-05 @slice-13
  Scenario: an older target is refused, since update only goes forward
    Given a project in "later" made by new from the release "v1.1.0" with stack "go", features "cli" and name "blue-fox"
    And a saved snapshot of "later"
    When itos-template runs in "later" with "update --to v1.0.0"
    Then it exits with code 1
    And its error output says "v1.1.0"
    And the snapshot of "later" is unchanged

  @ID-UPDATE-06 @slice-13
  Scenario: a --to no release has is refused as a usage error, naming it
    Given a saved snapshot of "made"
    When itos-template runs in "made" with "update --to v9.9.9"
    Then it exits with code 2
    And its error output says "v9.9.9"
    And the snapshot of "made" is unchanged

  # If update took the old render from the tag rather than the record's
  # commits, release.txt would read "moved" in the base and the project's
  # "v1.0.0" would conflict with the release's "v1.1.0".
  @ID-UPDATE-07 @slice-13
  Scenario: the old render comes from the record's commits, not from a tag moved since
    Given the template's tags of "v1.0.0" moved to commits whose release.txt holds "moved"
    When itos-template runs in "made" with "update"
    Then it exits with code 0
    And the file "made/release.txt" contains "v1.1.0"

  @ID-UPDATE-08 @slice-13
  Scenario: a project change to lines the release changes is left conflicted as git merge leaves it
    Given a committed project change to "made/release.txt" containing "project-release"
    And a saved snapshot of the HEAD of "made"
    When itos-template runs in "made" with "update"
    Then it exits with code 1
    And its error output says "release.txt"
    And its error output says "git reset --hard"
    And the file "made/release.txt" holds git's conflict markers around "project-release" and "v1.1.0"
    And the index of "made" holds "release.txt" unmerged, its base "v1.0.0", the project's "project-release" and the release's "v1.1.0"
    And the record in "made" names the release "v1.1.0" and its exact branch commits
    And "made" has the record staged
    And the HEAD of "made" is unchanged

  @ID-UPDATE-09 @slice-13
  Scenario: a conflicted update with --json is one object naming the conflict
    Given a committed project change to "made/release.txt" containing "project-release"
    When itos-template runs in "made" with "update --json"
    Then it exits with code 1
    And its JSON output names the problem "merge-conflict"
    And its update JSON conflicts are "release.txt"

  @ID-UPDATE-10 @slice-13
  Scenario: a file the release changes and the project deleted is left as a modify/delete conflict
    Given a committed project deletion of "made/release.txt"
    When itos-template runs in "made" with "update"
    Then it exits with code 1
    And its error output says "release.txt"
    And the index of "made" holds "release.txt" unmerged, its base "v1.0.0", no project side and the release's "v1.1.0"

  @ID-UPDATE-11 @slice-13
  Scenario: a file the release adds is rendered with the project's answers, its name too
    Given the template "acme" released as "v1.0.0", "v1.1.0" and "v1.2.0-rc.1", whose v1.1.0 adds "docs/acme-widget.md" containing "acme-widget AcmeWidget"
    And a project in "fresh" made by new from the release "v1.0.0" with stack "go", features "cli" and name "blue-fox"
    When itos-template runs in "fresh" with "update --to v1.1.0"
    Then it exits with code 0
    And the file "fresh/docs/blue-fox.md" contains "blue-fox BlueFox"
    And the path "fresh/docs/acme-widget.md" does not exist
    And "fresh" has "docs/blue-fox.md" staged as added

  @ID-UPDATE-12 @slice-13
  Scenario: a file the release removes is removed when the project left it unchanged
    Given the template "acme" released as "v1.0.0", "v1.1.0" and "v1.2.0-rc.1", whose v1.1.0 removes "logo.bin"
    And a project in "fresh" made by new from the release "v1.0.0" with stack "go", features "cli" and name "blue-fox"
    When itos-template runs in "fresh" with "update --to v1.1.0"
    Then it exits with code 0
    And the path "fresh/logo.bin" does not exist
    And "fresh" has "logo.bin" staged as removed

  @ID-UPDATE-13 @slice-13
  Scenario: a file the release renames keeps the project's change under its new name
    Given the template "acme" released as "v1.0.0", "v1.1.0" and "v1.2.0-rc.1", whose v1.1.0 renames "README.md" to "README.txt"
    And a project in "fresh" made by new from the release "v1.0.0" with stack "go", features "cli" and name "blue-fox"
    And a committed project change to "fresh/README.md" containing "private-project-content"
    When itos-template runs in "fresh" with "update --to v1.1.0"
    Then it exits with code 0
    And the file "fresh/README.txt" contains "private-project-content"
    And the path "fresh/README.md" does not exist

  @ID-UPDATE-14 @slice-13
  Scenario: a template-only path the release changes never reaches the project
    Given the template "acme" released as "v1.0.0", "v1.1.0" and "v1.2.0-rc.1", whose v1.1.0 changes ".github/workflows/template.yml"
    And a project in "fresh" made by new from the release "v1.0.0" with stack "go", features "cli" and name "blue-fox"
    When itos-template runs in "fresh" with "update --to v1.1.0"
    Then it exits with code 0
    And the path "fresh/.github/workflows/template.yml" does not exist

  @ID-UPDATE-15 @slice-13
  Scenario Outline: a question the release adds is answered as new answers one
    Given the template "acme" released as "v1.0.0", "v1.1.0" and "v1.2.0-rc.1", whose v1.1.0 adds the question "owner" for the literal "acme-owner" in "OWNERS", its default "nobody"
    And a project in "fresh" made by new from the release "v1.0.0" with stack "go", features "cli" and name "blue-fox"
    When itos-template runs in "fresh" with "update --to v1.1.0 <answer>"
    Then it exits with code 0
    And the file "fresh/OWNERS" contains "<owner>"
    And the record in "fresh" has the answer "owner" as "<owner>"

    Examples:
      | answer             | owner     |
      | --answer owner=ops | ops       |
      | --defaults         | nobody    |

  @ID-UPDATE-16 @slice-13
  Scenario: a question the release adds is refused unanswered without a terminal, nothing written
    Given the template "acme" released as "v1.0.0", "v1.1.0" and "v1.2.0-rc.1", whose v1.1.0 adds the question "owner" for the literal "acme-owner" in "OWNERS", its default "nobody"
    And a project in "fresh" made by new from the release "v1.0.0" with stack "go", features "cli" and name "blue-fox"
    And a saved snapshot of "fresh"
    When itos-template runs in "fresh" with "update --to v1.1.0"
    Then it exits with code 2
    And its error output says "owner"
    And the snapshot of "fresh" is unchanged

  @ID-UPDATE-17 @slice-13
  Scenario: an answer to a question the record already answers is refused, nothing written
    Given a saved snapshot of "made"
    When itos-template runs in "made" with "update --answer name=red-cat"
    Then it exits with code 2
    And its error output says "name"
    And the snapshot of "made" is unchanged

  @ID-UPDATE-18 @slice-13
  Scenario Outline: update refuses a dirty project, nothing written
    Given "made" has <state>
    And a saved snapshot of "made"
    When itos-template runs in "made" with "update"
    Then it exits with code 1
    And its error output says "clean"
    And the snapshot of "made" is unchanged

    Examples:
      | state                        |
      | an unstaged tracked change   |
      | a staged tracked change      |
      | a non-ignored untracked file |

  @ID-UPDATE-19 @slice-13
  Scenario: a project with no record is refused, pointed at adopt
    Given a committed project deletion of "made/.itos-template.yaml"
    When itos-template runs in "made" with "update"
    Then it exits with code 1
    And its error output says ".itos-template.yaml"
    And its error output says "adopt"

  @ID-UPDATE-20 @slice-13
  Scenario Outline: update refuses a folder that is not a project's git repository
    Given "<dir>" is <state>
    When itos-template runs in "<dir>" with "update"
    Then it exits with code 3
    And its error output says "<problem>"

    Examples:
      | dir   | state                               | problem |
      | plain | a folder outside any git repository | git     |
      | bare  | a bare git repository               | bare    |

  # slice-15: git merge-tree labels a conflict's markers with the arguments it
  # was given, HEAD and the new render's tree, an object id no person
  # recognises, and under diff3 the base's tree id too. update names the sides
  # as a person reads them: HEAD for the project, as git merge does, and
  # "template" and the release for the renders: "template v1.1.0" for the new
  # one, "template v1.0.0" for the base, or the template's root commit,
  # abbreviated, for a record naming no release. The person's
  # merge.conflictStyle is kept (merge, diff3, zdiff3), as merge-tree keeps it.
  # Tried 2026-10-10 on git 2.56: git merge-file -L over a conflict's three
  # index stages gives the same markers with the names chosen, still git's
  # own merge (decision 38); the stages stay as they were (ID-UPDATE-08).
  @ID-UPDATE-21 @slice-15
  Scenario: a conflict's markers name HEAD and the release, never a git object id
    Given a committed project change to "made/release.txt" containing "project-release"
    When itos-template runs in "made" with "update"
    Then it exits with code 1
    And the file "made/release.txt" has the line "<<<<<<< HEAD"
    And the file "made/release.txt" has the line ">>>>>>> template v1.1.0"
    And the file "made/release.txt" names no git object id
    And the index of "made" holds "release.txt" unmerged, its base "v1.0.0", the project's "project-release" and the release's "v1.1.0"

  @ID-UPDATE-22 @slice-15
  Scenario: under the person's diff3 conflict style the base's section names the recorded release
    Given a committed project change to "made/release.txt" containing "project-release"
    When itos-template runs in "made" with git's merge.conflictStyle set to diff3 with "update"
    Then it exits with code 1
    And the file "made/release.txt" has the line "||||||| template v1.0.0"
    And the file "made/release.txt" has the line ">>>>>>> template v1.1.0"
    And the file "made/release.txt" names no git object id

  # slice-18 (declined paths): GitHub issue #3, part 2. update leaves each path
  # the record declines out of the old and the new render alike, so the release
  # never changes, adds or removes it, and the project's file there, or its
  # absence, stays as the project has it. The record keeps the list. The record
  # is edited by a step here as a person would edit it, then committed.
  @ID-UPDATE-23 @slice-18
  Scenario: a declined file the release changes stays as the project has it, with no conflict
    Given a committed project change to "made/release.txt" containing "project-release"
    And "made" records "release.txt" as declined, committed
    When itos-template runs in "made" with "update"
    Then it exits with code 0
    And the file "made/release.txt" contains "project-release"
    And "made" has staged changes to ".itos-template.yaml" only, and nothing unstaged
    And the record in "made" names the release "v1.1.0" and its exact branch commits
    And the record in "made" declines "release.txt"

  @ID-UPDATE-24 @slice-18
  Scenario: a declined file the project deleted never comes back, and no modify/delete conflict is left
    Given a committed project deletion of "made/release.txt"
    And "made" records "release.txt" as declined, committed
    When itos-template runs in "made" with "update"
    Then it exits with code 0
    And the path "made/release.txt" does not exist
    And "made" has staged changes to ".itos-template.yaml" only, and nothing unstaged

  @ID-UPDATE-25 @slice-18
  Scenario: a declined folder keeps out a file the release adds under it
    Given the template "acme" released as "v1.0.0", "v1.1.0" and "v1.2.0-rc.1", whose v1.1.0 adds "docs/acme-widget.md" containing "acme-widget AcmeWidget"
    And a project in "fresh" made by new from the release "v1.0.0" with stack "go", features "cli" and name "blue-fox"
    And "fresh" records "docs" as declined, committed
    When itos-template runs in "fresh" with "update --to v1.1.0"
    Then it exits with code 0
    And the path "fresh/docs/blue-fox.md" does not exist
    And "fresh" has staged changes to "release.txt" and ".itos-template.yaml" only, and nothing unstaged
