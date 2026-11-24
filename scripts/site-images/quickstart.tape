# trackfw quick start demo — regenerate with scripts/site-images/generate.sh
# Output path is overridden at runtime by generate.sh (Output line replaced with
# a relative path; vhs is invoked with cwd=REPO_ROOT so the path resolves correctly).

Output site/public/img/quickstart.gif

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
Type `trackfw init --ai-tools claude --forge github --identity-preset neutral < /dev/null > /dev/null 2>&1`
Enter
Sleep 3s
Ctrl+L
Show

Type "# 1. Record a decision"
Enter
Type `trackfw adr new "User Authentication Strategy"`
Enter
Sleep 1.2s
Type "Users need to sign in to see their own data."
Enter
Sleep 500ms
Type "Email and password, hashed with bcrypt."
Enter
Sleep 500ms
Type "Simple to ship now; SSO can come later."
Enter
Sleep 500ms
Type "SSO only: rejected for the first release."
Enter
Sleep 1.5s
Ctrl+L
Sleep 300ms

Type "# 2. A requirement linked to that decision"
Enter
Type `trackfw req new "User Authentication" --adr ADR-$(date +%F)-user-authentication-strategy.md`
Enter
Sleep 1.5s
Enter
Sleep 500ms
Type "Users must log in before reaching their dashboard."
Enter
Sleep 800ms
Enter
Sleep 700ms
Enter
Sleep 700ms
Enter
Sleep 2s
Ctrl+L
Sleep 300ms

Type "# 3. Start the work"
Enter
Type "trackfw roadmap move ROADMAP-$(date +%F)-user-authentication.md wip"
Enter
Sleep 1.5s
Type "trackfw validate"
Enter
Sleep 2.5s
