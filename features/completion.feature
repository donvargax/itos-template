@phase-1
Feature: Shell completion
  itos-template completes on bash, zsh, fish and PowerShell (decision 25):
  itos-template completion <shell> prints a short script for that shell,
  which asks the binary itself what fits, through a hidden command,
  __complete, taking the words typed so far, the last the one being
  completed (empty for a new word). __complete answers from kong's model of
  the command line, so the scripts never go stale: one candidate a line, then
  a last line saying what the shell does beyond them, ":files" to complete
  file names (a template's path, a folder) or ":none". A flag's values are
  completed only where kong knows them: no flag has a fixed set yet (a
  stack's names come from a template's manifest, the idea
  completion-values). The scenarios ask __complete, as a script would, on
  the three systems; no shell runs in them.

  # slice-11 (shell-completion)
  # Main-red attempt (2026-10-08): commits 143dace..e7f4d93 were reverted after CI run
  # 37861711541 failed because completion.feature has no representative scenario in
  # features/smoke.yaml. Before retrying, add a completion smoke scenario and prove the code
  # without the two rejected mutation exceptions. At recovery start all seven scenarios were
  # @wip; ID-COMPL-07 is retained as a boundary case.
  # Red evidence before implementation: 01-03 and 07 exited 2 with "unexpected argument
  # __complete"; 04 exited 2 with "unexpected argument completion"; 05 did not name tcsh in its
  # error; 06's help omitted completion. The 04 outline exercised all four shell names.

  @ID-COMPL-01 @slice-11
  Scenario: a new word at the top completes to the commands, the hidden one left out
    When itos-template runs with "__complete ''"
    Then it exits with code 0
    And its standard output lists "new"
    And its standard output lists "check"
    And its standard output lists "completion"
    And its standard output does not list "__complete"

  @ID-COMPL-02 @slice-11
  Scenario: a flag of a command completes from its name's start
    When itos-template runs with "__complete new acme made --st"
    Then it exits with code 0
    And its standard output lists "--stack"
    And its standard output does not list "--feature"
    And the last line of its standard output is ":none"

  @ID-COMPL-03 @slice-11
  Scenario: a command's argument is left to the shell's completion of file names
    When itos-template runs with "__complete new ''"
    Then it exits with code 0
    And the last line of its standard output is ":files"

  @ID-COMPL-04 @slice-11
  Scenario Outline: completion prints a script for <shell> that asks itos-template
    When itos-template runs with "completion <shell>"
    Then it exits with code 0
    And its standard output says "__complete"

    Examples:
      | shell      |
      | bash       |
      | zsh        |
      | fish       |
      | powershell |

  @ID-COMPL-05 @slice-11
  Scenario: completion refuses a shell it has no script for with exit 2, naming it
    When itos-template runs with "completion tcsh"
    Then it exits with code 2
    And its error output says "tcsh"

  @ID-COMPL-06 @slice-11
  Scenario: the help names completion and not __complete
    When itos-template runs with "--help"
    Then it exits with code 0
    And its standard output says "completion"
    And its standard output does not say "__complete"

  @ID-COMPL-07 @slice-11
  Scenario: completion handles a shell with no current word
    When itos-template runs with "__complete"
    Then it exits with code 0
    And its standard output lists "new"
    And its standard output does not list "__complete"
