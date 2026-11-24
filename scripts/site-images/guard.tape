# trackfw guard demo — shows the guard blocking a raw git push
# Output path is overridden at runtime by generate.sh (Output line replaced with
# a relative path; vhs is invoked with cwd=REPO_ROOT so the path resolves correctly).

Output site/public/img/guard.gif
Output site/public/img/guard.txt

Require trackfw
Require git

Set Shell "bash"
Set FontSize 20
Set FontFamily "JetBrainsMono Nerd Font"
Set Width 820
Set Height 320
Set Theme "Catppuccin Mocha"
Set Padding 24
Set Margin 8
Set MarginFill "#1e1e2e"
Set BorderRadius 8
Set TypingSpeed 30ms

Hide
Type `cd "$(mktemp -d)" && export HOME="$PWD/.home" && mkdir -p "$HOME" && export USERPROFILE="$HOME" && export CODEX_HOME="$HOME/.codex" && export XDG_CONFIG_HOME="$HOME/.config" && git init -q -b main && export PS1='$ ' && export TZ=UTC && export GIT_AUTHOR_NAME="Demo User" && export GIT_AUTHOR_EMAIL="demo@example.invalid" && export GIT_COMMITTER_NAME="Demo User" && export GIT_COMMITTER_EMAIL="demo@example.invalid" && unset GH_TOKEN && unset GITHUB_TOKEN`
Enter
Sleep 300ms
Type `trackfw init --ai-tools claude --forge github --identity-preset neutral < /dev/null > /dev/null 2>&1`
Enter
Sleep 3s
Ctrl+L
Show

Type "# AI agent tries to push straight to main. The hook says no:"
Enter
Type `printf '{"tool_name":"Bash","tool_input":{"command":"git push origin main"}}' | trackfw guard git-branch; echo "exit=$?"`
Enter
Sleep 4s
Screenshot site/public/img/guard-poster.png
