package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/alecthomas/kong"
)

// Completion is the public command that prints a shell's completion stub.
type Completion struct {
	Shell string `arg:"" enum:"bash,zsh,fish,powershell" help:"The shell: bash, zsh, fish or powershell."`
}

// Run prints the requested shell stub, or reports an unsupported shell as a
// usage error.
func (c Completion) Run(ui *UI) int {
	script, ok := Scripts()[strings.ToLower(c.Shell)]
	if !ok {
		return ui.Usage(fmt.Errorf("unsupported completion shell %q; choose bash, zsh, fish or powershell", c.Shell), false)
	}
	_, _ = io.WriteString(ui.Stdout, script)
	return 0
}

// Scripts are short shell integrations that ask this executable for each
// completion request.
func Scripts() map[string]string {
	return map[string]string{
		"bash": `# Add this to your shell startup file.
_itos_template_complete() {
  local IFS=$'\n' reply
  reply=$(itos-template __complete "${COMP_WORDS[@]:1}") || return
  COMPREPLY=()
  local line
  while IFS= read -r line; do
    case "$line" in
      :files) compopt -o default; return ;;
      :none) return ;;
      *) COMPREPLY+=("$line") ;;
    esac
  done <<< "$reply"
}
complete -F _itos_template_complete itos-template
`,
		"zsh": `# Add this to your shell startup file.
_itos_template_complete() {
  local -a reply
  reply=("${(@f)$(itos-template __complete ${words[2,-1]})}")
  if [[ ${reply[-1]} == :files ]]; then
    _files
  else
    reply[-1]=()
    compadd -Q -- "${reply[@]}"
  fi
}
compdef _itos_template_complete itos-template
`,
		"fish": `# Save as ~/.config/fish/completions/itos-template.fish.
function __itos_template_complete
  set -l reply (itos-template __complete (commandline -opc) (commandline -ct))
  if test "$reply[-1]" = :files
    __fish_complete_path (commandline -ct)
    return
  end
  printf '%s\n' $reply[1..-2]
end
complete -c itos-template -f -a '(__itos_template_complete)'
`,
		"powershell": `# Add this to your PowerShell profile.
Register-ArgumentCompleter -Native -CommandName itos-template -ScriptBlock {
  param($wordToComplete, $commandAst, $cursorPosition)
  $words = @($commandAst.CommandElements | Select-Object -Skip 1 | ForEach-Object { $_.Extent.Text })
  if ($words.Count -gt 0 -and $words[-1] -eq $wordToComplete) {
    if ($words.Count -eq 1) { $words = @() } else { $words = $words[0..($words.Count - 2)] }
  }
  $words += $wordToComplete
  $reply = @(& itos-template __complete @words)
  if ($reply[-1] -eq ':files') {
    Get-ChildItem -Name "$wordToComplete*" | ForEach-Object { [System.Management.Automation.CompletionResult]::new($_, $_, 'ProviderItem', $_) }
  } elseif ($reply[-1] -ne ':none') {
    $reply | Select-Object -SkipLast 1 | ForEach-Object { [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_) }
  }
}
`,
	}
}

// Complete returns one candidate per line followed by the shell's fallback
// instruction. words contains the arguments after __complete; its last word
// is the partial word, including an empty word for a new token.
func Complete(root *kong.Application, words []string) []string {
	partial := ""
	previous := words
	if len(words) > 0 {
		partial = words[len(words)-1]
		previous = words[:len(words)-1]
	}
	node := root.Node
	position := 0
	optionsEnded := false
	needValue := false
	var valueFlag *kong.Flag
	for i := 0; i < len(previous); i++ {
		word := previous[i]
		if optionsEnded {
			position++
			continue
		}
		if word == "--" {
			optionsEnded = true
			continue
		}
		if strings.HasPrefix(word, "-") && word != "-" {
			flag, valueGiven := findFlag(node, word)
			if flag != nil && !flag.IsBool() && !flag.IsCumulative() && !valueGiven {
				needValue = true
				valueFlag = flag
				if i+1 < len(previous) {
					i++
					needValue = false
					valueFlag = nil
				}
			}
			continue
		}
		if child := childNamed(node, word); child != nil {
			node = child
			position = 0
			needValue = false
			continue
		}
		position++
		needValue = false
	}
	if optionsEnded {
		if position < len(node.Positional) {
			return []string{":files"}
		}
		return []string{":none"}
	}
	if name, value, hasValue := strings.Cut(partial, "="); strings.HasPrefix(partial, "-") && hasValue {
		flag, _ := findFlag(node, name)
		if flag != nil && flag.Enum != "" {
			return flagValues(flag, value)
		}
	}
	if needValue {
		if valueFlag != nil && valueFlag.Enum != "" {
			return flagValues(valueFlag, partial)
		}
		return []string{":none"}
	}
	if strings.HasPrefix(partial, "-") {
		return flagCandidates(root.Node, node, partial)
	}
	if len(node.Children) > 0 && position == 0 {
		var candidates []string
		for _, child := range node.Children {
			if !child.Hidden && strings.HasPrefix(child.Name, partial) {
				candidates = append(candidates, child.Name)
			}
		}
		return finish(candidates, ":none")
	}
	if position < len(node.Positional) {
		if argument := node.Positional[position]; argument.Enum != "" {
			values := argument.EnumSlice()
			var candidates []string
			for _, value := range values {
				if strings.HasPrefix(value, partial) {
					candidates = append(candidates, value)
				}
			}
			return finish(candidates, ":none")
		}
		return []string{":files"}
	}
	return []string{":none"}
}

func flagCandidates(root, node *kong.Node, prefix string) []string {
	var candidates []string
	for current := node; current != nil; current = current.Parent {
		for _, flag := range current.Flags {
			if flag.Hidden {
				continue
			}
			name := "--" + flag.Name
			if strings.HasPrefix(name, prefix) {
				candidates = append(candidates, name)
			}
			if flag.Short != 0 {
				short := "-" + string(flag.Short)
				if strings.HasPrefix(short, prefix) {
					candidates = append(candidates, short)
				}
			}
			if flag.Tag != nil && flag.Tag.Negatable != "" {
				name = "--no-" + flag.Name
				if strings.HasPrefix(name, prefix) {
					candidates = append(candidates, name)
				}
			}
		}
	}
	return finish(candidates, ":none")
}

func findFlag(node *kong.Node, name string) (*kong.Flag, bool) {
	for current := node; current != nil; current = current.Parent {
		for _, flag := range current.Flags {
			long := "--" + flag.Name
			if name == long || strings.HasPrefix(name, long+"=") {
				return flag, name != long
			}
			if flag.Short != 0 && name == "-"+string(flag.Short) {
				return flag, false
			}
		}
	}
	return nil, false
}

func flagValues(flag *kong.Flag, prefix string) []string {
	var candidates []string
	for _, value := range flag.EnumSlice() {
		if strings.HasPrefix(value, prefix) {
			candidates = append(candidates, value)
		}
	}
	return finish(candidates, ":none")
}

func childNamed(node *kong.Node, name string) *kong.Node {
	for _, child := range node.Children {
		if child.Name == name || contains(child.Aliases, name) {
			return child
		}
	}
	return nil
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func finish(candidates []string, fallback string) []string {
	sort.Strings(candidates)
	candidates = append(candidates, fallback)
	return candidates
}
