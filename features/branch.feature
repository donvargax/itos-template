@phase-1
Feature: branch sync merges a template branch's upkeep down from the branch it descends from, and branch list says where every branch stands
  A template's optional parts are branches (decision 2): the root, a stack
  off it, a feature off its stack or off the feature it needs
  (docs/manifest.md's "Branches"). Each takes the root's changes by a merge,
  never a rebase, and that upkeep is branch sync's. branch is the noun group
  of the two (q-60): alone it runs neither, naming them.

  branch sync runs in a clone of a template, on the branch checked out
  (q-57). The root is origin's default branch, as release reads it, and the
  branches follow from the manifest on the root's local branch. Each branch
  descends from others: a stack from the root, a feature from its stack, and
  a feature that needs others from each feature it needs, in the manifest's
  order, instead of its stack. branch sync merges into the branch checked
  out the first of those whose commits it lacks, with git's own merge, and
  commits nothing: the merge is left in progress and staged, as git merge
  --no-commit leaves one, for the person to commit through the template's
  own gates with whatever message and footers its rules ask (decision 37).
  The commit is then a merge of both. It prints the branch it merged. HEAD
  never moves. One branch a run: the person commits, then runs branch sync
  again for the next branch it descends from, or switches to the next
  branch down.

  A conflict is left as git merge leaves one: markers in the file, unmerged
  index entries, everything else merged and staged, exit 1 naming each
  conflicted path and git merge --abort, which undoes it. No path is special
  (q-59): the manifest, the template-only paths and each branch's own itos
  data merge as git merges them, a conflict the person resolves.

  branch sync goes down the tree a branch at a time, so a branch it descends
  from that itself lacks commits of a branch above it is refused, naming it,
  to be synced first. A branch that lacks nothing exits 0 saying it is up to
  date, and so does the root, which descends from none.

  Merging requires a git repository (else exit 3); a branch checked out that
  the manifest names (else exit 2: a detached HEAD, or a branch of none,
  the error naming the branches branch sync can merge into); an origin
  naming its default branch (else exit 2, as release refuses one); an
  origin, each branch it descends from local, a clean index and working
  tree, non-ignored untracked files included, and no merge in progress (else
  exit 1, nothing written).

  With --json, stdout is one object with schema: 1 and ok, branch, merged
  (the branch merged, or null when nothing was), staged and conflicts, each
  an object with path, and problems. A conflicted merge is ok: false with the problem
  "merge-conflict". The human message and fix fields are not a machine
  contract (docs/CLI.md).

  branch list (q-58) writes nothing and runs on any checkout, a detached or
  unclean one too: it lists every branch the manifest names, in its order,
  with each branch it descends from whose commits it lacks, and a branch
  missing locally as missing. It exits 0 when no branch lacks any, 1 when
  one does, so a template's CI can hold it. With --json, stdout is one
  object with schema: 1, ok and branches, each with branch, from (the
  branches it descends from), lacks (those whose commits it lacks) and
  missing.

  # A fresh clone of the acme fixture is in step: features/testdata/acme
  # builds each branch off the one it descends from. A scenario gives a
  # branch a commit to make another lack it.

  @ID-SYNC-01 @slice-19
  Scenario: a stack takes the root's new commit as a staged merge, HEAD unchanged
    Given the template "acme"
    And a clone "tpl" of the template, every branch local
    And in "tpl" the branch "main" gains a commit setting "shared.txt" to "shared"
    And in "tpl" the branch "stack/go" is checked out
    And a saved snapshot of the HEAD of "tpl"
    When itos-template runs in the folder "tpl" with "branch sync"
    Then it exits with code 0
    And in "tpl" a merge of "main" is in progress
    And "tpl" has staged changes to "shared.txt" only, and nothing unstaged
    And the HEAD of "tpl" is unchanged
    And its standard output says "main"

  @ID-SYNC-02 @slice-19
  Scenario: a feature takes its stack's new commit
    Given the template "acme"
    And a clone "tpl" of the template, every branch local
    And in "tpl" the branch "stack/go" gains a commit setting "go-shared.txt" to "go"
    And in "tpl" the branch "go/cli" is checked out
    When itos-template runs in the folder "tpl" with "branch sync"
    Then it exits with code 0
    And in "tpl" a merge of "stack/go" is in progress
    And "tpl" has staged changes to "go-shared.txt" only, and nothing unstaged

  # web needs cli in the fixture's manifest, so go/web descends from go/cli,
  # which carries the stack, not from stack/go.
  @ID-SYNC-03 @slice-19
  Scenario: a feature that needs another takes that feature's new commit, not the stack's
    Given the template "acme"
    And a clone "tpl" of the template, every branch local
    And in "tpl" the branch "go/cli" gains a commit setting "cli-shared.txt" to "cli"
    And in "tpl" the branch "go/web" is checked out
    When itos-template runs in the folder "tpl" with "branch sync"
    Then it exits with code 0
    And in "tpl" a merge of "go/cli" is in progress
    And "tpl" has staged changes to "cli-shared.txt" only, and nothing unstaged

  @ID-SYNC-04 @slice-19
  Scenario Outline: a branch that lacks nothing, or the root, is up to date and nothing changes
    Given the template "acme"
    And a clone "tpl" of the template, every branch local
    And in "tpl" the branch "<branch>" is checked out
    And a saved snapshot of "tpl"
    When itos-template runs in the folder "tpl" with "branch sync"
    Then it exits with code 0
    And its standard output says "up to date"
    And in "tpl" no merge is in progress
    And the snapshot of "tpl" is unchanged

    Examples:
      | branch   |
      | main     |
      | stack/go |
      | go/web   |

  @ID-SYNC-05 @slice-19
  Scenario: a branch's change to lines the root changes is left conflicted as git merge leaves it
    Given the template "acme"
    And a clone "tpl" of the template, every branch local
    And in "tpl" the branch "main" gains a commit setting "README.md" to "from main"
    And in "tpl" the branch "stack/go" gains a commit setting "README.md" to "from stack/go"
    And in "tpl" the branch "stack/go" is checked out
    And a saved snapshot of the HEAD of "tpl"
    When itos-template runs in the folder "tpl" with "branch sync"
    Then it exits with code 1
    And its error output says "README.md"
    And its error output says "git merge --abort"
    And the file "tpl/README.md" holds git's conflict markers around "from stack/go" and "from main"
    And in "tpl" a merge of "main" is in progress, "README.md" unmerged
    And the HEAD of "tpl" is unchanged

  # branch sync goes down the tree a branch at a time: go/cli would take main's
  # commit only through stack/go, which lacks it.
  @ID-SYNC-06 @slice-19
  Scenario: a branch whose own parent lacks a commit above it is refused, naming the parent to sync first
    Given the template "acme"
    And a clone "tpl" of the template, every branch local
    And in "tpl" the branch "main" gains a commit setting "shared.txt" to "shared"
    And in "tpl" the branch "go/cli" is checked out
    And a saved snapshot of "tpl"
    When itos-template runs in the folder "tpl" with "branch sync"
    Then it exits with code 1
    And its error output says "stack/go"
    And in "tpl" no merge is in progress
    And the snapshot of "tpl" is unchanged

  @ID-SYNC-07 @slice-19
  Scenario Outline: an unclean checkout is refused and nothing changes
    Given the template "acme"
    And a clone "tpl" of the template, every branch local
    And in "tpl" the branch "main" gains a commit setting "shared.txt" to "shared"
    And in "tpl" the branch "stack/go" is checked out
    And "tpl" has <state>
    And a saved snapshot of "tpl"
    When itos-template runs in the folder "tpl" with "branch sync"
    Then it exits with code 1
    And in "tpl" no merge is in progress
    And the snapshot of "tpl" is unchanged

    Examples:
      | state                        |
      | an unstaged tracked change   |
      | a staged tracked change      |
      | a non-ignored untracked file |

  @ID-SYNC-08 @slice-19
  Scenario: a merge already in progress is refused, so a second run never stacks on the first
    Given the template "acme"
    And a clone "tpl" of the template, every branch local
    And in "tpl" the branch "main" gains a commit setting "shared.txt" to "shared"
    And in "tpl" the branch "stack/go" is checked out
    And itos-template ran in the folder "tpl" with "branch sync"
    When itos-template runs in the folder "tpl" with "branch sync"
    Then it exits with code 1
    And its error output says "merge"
    And in "tpl" a merge of "main" is in progress

  @ID-SYNC-09 @slice-19
  Scenario: a detached HEAD is a usage error naming the branches branch sync can merge into
    Given the template "acme"
    And a clone "tpl" of the template, every branch local
    And the clone "tpl" has its HEAD detached
    When itos-template runs in the folder "tpl" with "branch sync"
    Then it exits with code 2
    And its error output says "stack/go"

  @ID-SYNC-10 @slice-19
  Scenario: a branch the manifest does not name is a usage error naming the branches branch sync can merge into
    Given the template "acme"
    And a clone "tpl" of the template, every branch local
    And in "tpl" a new branch "topic" is checked out
    When itos-template runs in the folder "tpl" with "branch sync"
    Then it exits with code 2
    And its error output says "topic"
    And its error output says "go/cli"

  @ID-SYNC-11 @slice-19
  Scenario: a branch it descends from that is not local is refused, naming it
    Given the template "acme"
    And a clone "tpl" of the template, only its default branch local
    And in "tpl" the branch "go/cli" is checked out
    When itos-template runs in the folder "tpl" with "branch sync"
    Then it exits with code 1
    And its error output says "stack/go"
    And in "tpl" no merge is in progress

  @ID-SYNC-12 @slice-19
  Scenario: a clone whose origin names no default branch is refused, as release refuses it
    Given the template "acme"
    And a clone "tpl" of the template, every branch local
    And in "tpl" the branch "stack/go" is checked out
    And the clone "tpl" has no record of origin's HEAD
    And the clone "tpl" has an origin whose HEAD names no branch
    When itos-template runs in the folder "tpl" with "branch sync"
    Then it exits with code 2
    And its error output says "no default branch"
    And in "tpl" no merge is in progress

  @ID-SYNC-13 @slice-19
  Scenario: a folder outside any git repository is refused with exit 3
    Given "elsewhere" is a folder outside any git repository
    When itos-template runs in the folder "elsewhere" with "branch sync"
    Then it exits with code 3

  @ID-SYNC-14 @slice-19
  Scenario: a merge with --json is one object naming the branch merged and the paths staged
    Given the template "acme"
    And a clone "tpl" of the template, every branch local
    And in "tpl" the branch "main" gains a commit setting "shared.txt" to "shared"
    And in "tpl" the branch "stack/go" is checked out
    When itos-template runs in the folder "tpl" with "branch sync --json"
    Then it exits with code 0
    And its branch sync JSON is ok, branch "stack/go", merged "main", staged "shared.txt" and no conflict

  @ID-SYNC-15 @slice-19
  Scenario: a conflicted merge with --json is one object naming the conflict
    Given the template "acme"
    And a clone "tpl" of the template, every branch local
    And in "tpl" the branch "main" gains a commit setting "README.md" to "from main"
    And in "tpl" the branch "stack/go" gains a commit setting "README.md" to "from stack/go"
    And in "tpl" the branch "stack/go" is checked out
    When itos-template runs in the folder "tpl" with "branch sync --json"
    Then it exits with code 1
    And its branch sync JSON is not ok, the problem "merge-conflict", conflicts "README.md"

  @ID-SYNC-16 @slice-20
  Scenario: branch list on a template in step lists every branch, none lacking, and exits 0
    Given the template "acme"
    And a clone "tpl" of the template, every branch local
    When itos-template runs in the folder "tpl" with "branch list --json"
    Then it exits with code 0
    And its branch list JSON lists "main", "stack/go", "go/cli", "go/web", "stack/python" and "python/cli", none lacking a branch

  # main's commit is lacked by the stacks only: the branches below them lack
  # nothing of the branch they descend from until a stack takes it.
  @ID-SYNC-17 @slice-20
  Scenario: branch list names each branch that lacks a branch it descends from, writes nothing and exits 1
    Given the template "acme"
    And a clone "tpl" of the template, every branch local
    And in "tpl" the branch "main" gains a commit setting "shared.txt" to "shared"
    And a saved snapshot of "tpl"
    When itos-template runs in the folder "tpl" with "branch list --json"
    Then it exits with code 1
    And its branch list JSON says "stack/go" lacks "main"
    And its branch list JSON says "stack/python" lacks "main"
    And its branch list JSON says "go/cli" lacks nothing
    And the snapshot of "tpl" is unchanged

  @ID-SYNC-18 @slice-20
  Scenario: branch list reports from a detached, unclean checkout and names a branch missing locally
    Given the template "acme"
    And a clone "tpl" of the template, only its default branch local
    And the clone "tpl" has its HEAD detached
    And "tpl" has an unstaged tracked change
    When itos-template runs in the folder "tpl" with "branch list"
    Then it exits with code 1
    And its standard output says "stack/go"
    And its standard output says "missing"

  # slice-21 (q-60): sync's --status selected a read-only report instead of a
  # merge, which docs/CLI.md rule 18 forbids, so both actions moved into the
  # noun group branch (rules 3 and 4), each a verb: sync became branch sync,
  # sync --status branch list, and every scenario above was renamed to match,
  # its ID kept and nothing else it checks changed. The group alone runs no
  # action (rule 7): it names its subcommands, exit 2 (rule 13).
  @ID-SYNC-19 @slice-21
  Scenario: branch with no subcommand is a usage error naming sync and list
    When itos-template runs with "branch"
    Then it exits with code 2
    And its error output says "sync"
    And its error output says "list"

  # An old name exits 2 naming the new one, and keeps no code to run it (rule
  # 41).
  @ID-SYNC-20 @slice-21
  Scenario Outline: the old sync and its --status exit 2 naming the command that replaced them
    When itos-template runs with "<old>"
    Then it exits with code 2
    And its error output says "<new>"

    Examples:
      | old                  | new         |
      | sync                 | branch sync |
      | sync --status        | branch list |
