package cli

// GenerateBashCompletion returns a bash completion script for gopher-code.
func GenerateBashCompletion() string {
	return `_gopher_code() {
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    opts="--model --query --cwd --print --continue --resume --system-prompt --system-prompt-file --append-system-prompt --max-turns --dangerously-skip-permissions --output-format --thinking --effort --verbose --version --no-session-persistence --allowed-tools --disallowed-tools --session-id --name --prefill --debug --debug-file --bare --max-budget-usd --provider --api-url --input-format --json-schema --init --fallback-model --permission-mode --add-dir --worktree --betas --include-hook-events --help"
    COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
}
complete -F _gopher_code gopher-code`
}

// GenerateZshCompletion returns a zsh completion script for gopher-code.
func GenerateZshCompletion() string {
	return `#compdef gopher-code
_gopher_code() {
    _arguments \
        '--model[Model to use]:model:' \
        '--query[One-shot query]:query:' \
        '--cwd[Working directory]:dir:_directories' \
        {-p,--print}'[Print response and exit]' \
        {-c,--continue}'[Continue most recent conversation]' \
        {-r,--resume}'[Resume by session ID]:id:' \
        '--system-prompt[Override system prompt]:prompt:' \
        '--max-turns[Maximum turns]:turns:' \
        '--output-format[Output format]:format:(text json stream-json)' \
        '--thinking[Thinking mode]:mode:(enabled disabled)' \
        '--effort[Effort level]:level:(low medium high max)' \
        '--provider[Provider]:provider:(anthropic bedrock vertex openai)' \
        '--permission-mode[Permission mode]:mode:(auto interactive deny)' \
        '--version[Show version]' \
        '--verbose[Verbose output]' \
        '--help[Show help]'
}
_gopher_code "$@"`
}

// GenerateFishCompletion returns a fish completion script for gopher-code.
func GenerateFishCompletion() string {
	return `complete -c gopher-code -l model -d "Model to use"
complete -c gopher-code -l query -d "One-shot query"
complete -c gopher-code -l cwd -d "Working directory" -rF
complete -c gopher-code -s p -l print -d "Print response and exit"
complete -c gopher-code -s c -l continue -d "Continue most recent conversation"
complete -c gopher-code -s r -l resume -d "Resume by session ID"
complete -c gopher-code -l output-format -d "Output format" -rfa "text json stream-json"
complete -c gopher-code -l thinking -d "Thinking mode" -rfa "enabled disabled"
complete -c gopher-code -l effort -d "Effort level" -rfa "low medium high max"
complete -c gopher-code -l provider -d "Provider" -rfa "anthropic bedrock vertex openai"
complete -c gopher-code -l version -d "Show version"
complete -c gopher-code -l verbose -d "Verbose output"`
}
