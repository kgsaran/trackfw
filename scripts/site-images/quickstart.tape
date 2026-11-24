# trackfw quick start demo — regenerate with scripts/site-images/generate.sh
# Output path is overridden at runtime by generate.sh (Output lines replaced with
# relative paths; vhs is invoked with cwd=REPO_ROOT so paths resolve correctly).
# Commands run non-interactively via a shell function defined in the hidden setup.
# `init` output is suppressed inside the function (>/dev/null 2>&1) so the scroll
# buffer stays clean and all five README commands appear cleanly in the transcript.
# validate ends with the promised 0 violations, 2 warnings (unfilled ADR and REQ sections).

Output site/public/img/quickstart.gif
Output site/public/img/quickstart.txt

Require trackfw
Require git

Set Shell "bash"
Set FontSize 16
Set FontFamily "JetBrainsMono Nerd Font"
Set Width 1200
Set Height 720
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
Type `trackfw() { local c="$1"; shift; if [ "$c" = "init" ]; then command trackfw "$c" "$@" </dev/null >/dev/null 2>&1; else command trackfw "$c" "$@" </dev/null; fi; }`
Enter
Sleep 100ms
Ctrl+L
Show

Type `trackfw init --ai-tools claude --forge github`
Enter
Sleep 2s

Type "# 1. Record a decision"
Enter
Type `trackfw adr new "User Authentication Strategy"`
Enter
Sleep 2s
Ctrl+L
Sleep 300ms

Type "# 2. A requirement linked to that decision"
Enter
Type `trackfw req new "User Authentication" --adr ADR-$(date +%F)-user-authentication-strategy.md`
Enter
Sleep 2s
Ctrl+L
Sleep 300ms

Type "# 3. Start the work"
Enter
Type "trackfw roadmap move ROADMAP-$(date +%F)-user-authentication.md wip"
Enter
Sleep 1.5s
Type `trackfw validate                  # 0 violations, 2 warning(s) until you fill in the ADR and REQ sections`
Enter
Sleep 2.5s
Screenshot site/public/img/quickstart-poster.png
