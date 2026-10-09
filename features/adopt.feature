@phase-1
Feature: adopt links an existing project to a template baseline
  adopt <template> operates at the existing project's git root. It requires
  --ref: the reproducible release selection defined by template-releases,
  never a guessed matching version or an implicit latest release. --stack,
  repeatable --feature and --answer, and --defaults follow new's conventions.
  It renders that selection in temporary storage, with the same substitutions
  and template-only exclusions as new, without running checks or setup.

  The baseline is that render, not the project's first commit. The preview
  names the source, requested ref, selection, answers and resolved branch
  commits. Paths are relative to the project root, with / separators, sorted
  by path. A difference is project-only, modified, or missing from the
  project. Binary and tracked git file-mode differences are identified too.
  The default preview prints no file contents. --diff adds text diffs from
  baseline to project, never binary payloads; it applies nothing.

  --dry-run exits 0 after a successful preview, differences included, and
  writes no project files. It may inspect a dirty working tree, including
  non-ignored untracked files, and identifies that the preview is dirty.
  Otherwise differences refuse adoption with exit 1 unless --force accepts
  them. --force does not copy, delete or merge any project file. In particular,
  missing baseline files are project deletions for a later update, not files
  adoption installs. Adopting the latest ref imports no changes by itself.

  Writing requires a non-bare git repository with a committed HEAD, a clean
  index and working tree including non-ignored untracked files, and no existing
  .itos-template.yaml. --force bypasses none of these checks. .git and ignored
  project-only files are not differences. The selected template's files are
  still compared even when the project's ignore rules match their paths.

  Success creates only .itos-template.yaml, in the same format new records
  for that ref (decision 10), leaving existing files, HEAD and staged entries
  unchanged. The record is not staged or committed: the user commits it through
  the project's gates before update. Failure leaves no partial record.

  With --json, stdout is one object with schema: 1 and ok, written, baseline,
  differences and problems. baseline uses the same render record data as new;
  differences have path, kind (project-only, modified, missing), binary and
  mode_changed. --diff adds a diff string for text changes. Default JSON
  includes no file contents. Refused differences remain in the failure report.
  The human message and fix fields are not a machine contract (docs/CLI.md).
  Switches have --no- pairs and flag environment variables follow docs/CLI.md.

  # slice-12 (adopt): q-29 through q-33, answered by the person.
  # Initial migration is adopt an explicitly selected older baseline, commit
  # the record, then update. Ref semantics and cross-branch release selection
  # belong to template-releases; update's merge and conflict recovery belong
  # to update-merges. This slice writes no merge state and creates no commit.
  # The fixture release pins main, stack/go and go/cli. The existing project
  # has the rendered files but no record, and at least one ordinary commit.
  Background:
    Given the template "acme" with the release "v1.0.0"
    And an existing project in "made" matching that release with stack "go", features "cli", name "blue-fox" and module "example.com/blue/fox"

  @ID-ADOPT-01 @slice-12 @wip
  Scenario: matching adoption records the selected baseline without changing existing files or committing
    Given a saved adoption snapshot of "made"
    When itos-template runs in "made" with "adopt {template} --ref v1.0.0 --stack go --feature cli --answer name=blue-fox --answer module=example.com/blue/fox"
    Then it exits with code 0
    And the record in "made" names the template as "{template}"
    And the record in "made" names the stack "go" and the features "cli"
    And the record in "made" has the answer "name" as "blue-fox" and "module" as "example.com/blue/fox"
    And the record in "made" names the release "v1.0.0" and its exact branch commits
    And the adoption snapshot of "made" is unchanged except for the new unstaged record

  @ID-ADOPT-02 @slice-12 @wip
  Scenario: differences refuse adoption unless explicitly accepted and the default preview reveals no contents
    Given a committed project change to "made/README.md" containing "private-project-content"
    And a saved adoption snapshot of "made"
    When itos-template runs in "made" with "adopt {template} --ref v1.0.0 --stack go --feature cli --answer name=blue-fox --answer module=example.com/blue/fox"
    Then it exits with code 1
    And its standard output identifies "README.md" as "modified"
    And its standard output does not say "private-project-content"
    And its error output says "--force"
    And the adoption snapshot of "made" is unchanged

  @ID-ADOPT-03 @slice-12 @wip
  Scenario: force accepts modified missing and project-only files without installing or removing anything
    Given a committed project change to "made/README.md" containing "private-project-content"
    And a committed deletion of "made/cli.txt"
    And a committed project-only file "made/own.txt" containing "keep-this-file"
    And a saved adoption snapshot of "made"
    When itos-template runs in "made" with "adopt {template} --ref v1.0.0 --stack go --feature cli --answer name=blue-fox --answer module=example.com/blue/fox --force"
    Then it exits with code 0
    And its standard output identifies "README.md" as "modified"
    And its standard output identifies "cli.txt" as "missing"
    And its standard output identifies "own.txt" as "project-only"
    And the adoption snapshot of "made" is unchanged except for the new unstaged record
    And the path "made/cli.txt" does not exist

  @ID-ADOPT-04 @slice-12 @wip
  Scenario: dry-run shows differences successfully and writes nothing even with force
    Given a committed project change to "made/README.md" containing "private-project-content"
    And a saved adoption snapshot of "made"
    When itos-template runs in "made" with "adopt {template} --ref v1.0.0 --stack go --feature cli --answer name=blue-fox --answer module=example.com/blue/fox --dry-run --force"
    Then it exits with code 0
    And its standard output identifies "README.md" as "modified"
    And its standard output names the baseline "v1.0.0" and its exact branch commits
    And its standard output does not say "private-project-content"
    And the adoption snapshot of "made" is unchanged

  @ID-ADOPT-05 @slice-12 @wip
  Scenario: diff explicitly shows text changes without applying them
    Given a committed project change to "made/README.md" containing "private-project-content"
    And a saved adoption snapshot of "made"
    When itos-template runs in "made" with "adopt {template} --ref v1.0.0 --stack go --feature cli --answer name=blue-fox --answer module=example.com/blue/fox --dry-run --diff"
    Then it exits with code 0
    And its standard output shows a text diff from the baseline to "README.md" containing "private-project-content"
    And the adoption snapshot of "made" is unchanged

  @ID-ADOPT-06 @slice-12 @wip
  Scenario: the preview identifies a binary difference without printing its payload
    Given a committed binary change to "made/logo.bin" containing "private-binary-content" after a NUL
    And a saved adoption snapshot of "made"
    When itos-template runs in "made" with "adopt {template} --ref v1.0.0 --stack go --feature cli --answer name=blue-fox --answer module=example.com/blue/fox --dry-run --diff"
    Then it exits with code 0
    And its standard output identifies "logo.bin" as a binary modification
    And its standard output does not say "private-binary-content"
    And the adoption snapshot of "made" is unchanged

  # Git records executable modes on every platform, even when the checkout's
  # filesystem does not enforce them. The fixture changes the committed mode.
  @ID-ADOPT-07 @slice-12 @wip
  Scenario: the preview identifies a tracked git mode change with unchanged contents
    Given "made/cli.txt" has a committed executable mode unlike the baseline
    And a saved adoption snapshot of "made"
    When itos-template runs in "made" with "adopt {template} --ref v1.0.0 --stack go --feature cli --answer name=blue-fox --answer module=example.com/blue/fox --dry-run"
    Then it exits with code 0
    And its standard output identifies a mode change for "cli.txt"
    And the adoption snapshot of "made" is unchanged

  @ID-ADOPT-08 @slice-12 @wip
  Scenario: adoption requires an explicit ref even when the project matches the latest release
    Given a saved adoption snapshot of "made"
    When itos-template runs in "made" with "adopt {template} --stack go --feature cli --answer name=blue-fox --answer module=example.com/blue/fox --force"
    Then it exits with code 2
    And its error output says "--ref"
    And the adoption snapshot of "made" is unchanged

  @ID-ADOPT-09 @slice-12 @wip
  Scenario: a requested ref that does not exist is refused without writing a record
    Given a saved adoption snapshot of "made"
    When itos-template runs in "made" with "adopt {template} --ref v-missing --stack go --feature cli --answer name=blue-fox --answer module=example.com/blue/fox --force"
    Then it exits with code 2
    And its error output says "v-missing"
    And the adoption snapshot of "made" is unchanged

  @ID-ADOPT-10 @slice-12 @wip
  Scenario Outline: force never bypasses a dirty repository
    Given "made" has <state>
    And a saved adoption snapshot of "made"
    When itos-template runs in "made" with "adopt {template} --ref v1.0.0 --stack go --feature cli --answer name=blue-fox --answer module=example.com/blue/fox --force"
    Then it exits with code 1
    And its error output says "clean"
    And the adoption snapshot of "made" is unchanged

    Examples:
      | state                        |
      | an unstaged tracked change   |
      | a staged tracked change      |
      | a non-ignored untracked file |

  @ID-ADOPT-11 @slice-12 @wip
  Scenario: dry-run may preview a dirty working tree and identifies that fact without changing it
    Given "made" has an unstaged tracked change
    And a non-ignored untracked file "made/own.txt" containing "private-project-content"
    And a saved adoption snapshot of "made"
    When itos-template runs in "made" with "adopt {template} --ref v1.0.0 --stack go --feature cli --answer name=blue-fox --answer module=example.com/blue/fox --dry-run"
    Then it exits with code 0
    And its standard output says "dirty"
    And its standard output identifies "own.txt" as "project-only"
    And its standard output does not say "private-project-content"
    And the adoption snapshot of "made" is unchanged

  @ID-ADOPT-12 @slice-12 @wip
  Scenario: force never replaces an existing record
    Given a committed file "made/.itos-template.yaml" containing "keep-this-record"
    And a saved adoption snapshot of "made"
    When itos-template runs in "made" with "adopt {template} --ref v1.0.0 --stack go --feature cli --answer name=blue-fox --answer module=example.com/blue/fox --force"
    Then it exits with code 1
    And its error output says ".itos-template.yaml"
    And the adoption snapshot of "made" is unchanged

  @ID-ADOPT-13 @slice-12 @wip
  Scenario Outline: adoption refuses a project without the required git history or working tree
    Given "made" is <state>
    And a saved adoption snapshot of "made"
    When itos-template runs in "made" with "adopt {template} --ref v1.0.0 --stack go --feature cli --answer name=blue-fox --answer module=example.com/blue/fox --force"
    Then it exits with code <code>
    And its error output says "<problem>"
    And the adoption snapshot of "made" is unchanged

    Examples:
      | state                               | code | problem |
      | a folder outside any git repository | 3    | git     |
      | a bare git repository               | 3    | bare    |
      | a git repository without a commit   | 1    | commit  |

  @ID-ADOPT-14 @slice-12 @wip
  Scenario: missing answers are refused without a terminal rather than guessed from project files
    Given a saved adoption snapshot of "made"
    When itos-template runs in "made" with "adopt {template} --ref v1.0.0 --stack go --feature cli --force"
    Then it exits with code 2
    And its error output says "name"
    And its error output says "module"
    And the adoption snapshot of "made" is unchanged

  @ID-ADOPT-15 @slice-12 @wip
  Scenario: defaults use the selected manifest's answer default exactly as new does
    Given a saved adoption snapshot of "made"
    When itos-template runs in "made" with "adopt {template} --ref v1.0.0 --stack go --feature cli --answer name=blue-fox --defaults --force"
    Then it exits with code 0
    And the record in "made" has the answer "module" as "example.com/you/project"
    And the adoption snapshot of "made" is unchanged except for the new unstaged record

  @ID-ADOPT-16 @slice-12 @wip
  Scenario: JSON preview is one machine-readable object and writes no record
    Given a committed project change to "made/README.md" containing "private-project-content"
    And a saved adoption snapshot of "made"
    When itos-template runs in "made" with "adopt {template} --ref v1.0.0 --stack go --feature cli --answer name=blue-fox --answer module=example.com/blue/fox --dry-run --json"
    Then it exits with code 0
    And its standard output is one adoption JSON object with schema 1, ok true and written false
    And its adoption JSON baseline names "v1.0.0" and its exact branch commits
    And its adoption JSON differences include "README.md" with kind "modified", binary false and mode_changed false
    And its standard output does not say "private-project-content"
    And the adoption snapshot of "made" is unchanged

  @ID-ADOPT-17 @slice-12 @wip
  Scenario: JSON refusal preserves the difference report and writes nothing
    Given a committed project change to "made/README.md" containing "private-project-content"
    And a saved adoption snapshot of "made"
    When itos-template runs in "made" with "adopt {template} --ref v1.0.0 --stack go --feature cli --answer name=blue-fox --answer module=example.com/blue/fox --json"
    Then it exits with code 1
    And its standard output is one adoption JSON object with schema 1, ok false and written false
    And its adoption JSON differences include "README.md" with kind "modified", binary false and mode_changed false
    And its adoption JSON problems include a rule ID and a fix
    And the adoption snapshot of "made" is unchanged

  @ID-ADOPT-18 @slice-12 @wip
  Scenario: selecting an older release does not silently use the template's current branch heads
    Given the template's current branches differ from the release "v1.0.0"
    And a saved adoption snapshot of "made"
    When itos-template runs in "made" with "adopt {template} --ref v1.0.0 --stack go --feature cli --answer name=blue-fox --answer module=example.com/blue/fox"
    Then it exits with code 0
    And the record in "made" names the release "v1.0.0" and its exact branch commits
    And the adoption snapshot of "made" is unchanged except for the new unstaged record

  @ID-ADOPT-19 @slice-12 @wip
  Scenario: adoption renders a baseline without running its checks
    Given the template has a release "v1.1.0" with the same rendered files and checks that always fail if run
    And a saved adoption snapshot of "made"
    When itos-template runs in "made" with "adopt {template} --ref v1.1.0 --stack go --feature cli --answer name=blue-fox --answer module=example.com/blue/fox"
    Then it exits with code 0
    And the adoption snapshot of "made" is unchanged except for the new unstaged record

  @ID-ADOPT-20 @slice-12 @wip
  Scenario: ignored project-only files neither dirty the project nor become adoption differences
    Given "made" has a committed ignore rule for "local-only.txt"
    And an ignored untracked file "made/local-only.txt" containing "private-project-content"
    And a saved adoption snapshot of "made"
    When itos-template runs in "made" with "adopt {template} --ref v1.0.0 --stack go --feature cli --answer name=blue-fox --answer module=example.com/blue/fox --dry-run"
    Then it exits with code 0
    And its standard output does not list "local-only.txt"
    And its standard output does not say "private-project-content"
    And the adoption snapshot of "made" is unchanged
