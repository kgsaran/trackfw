# trackfw demo — regenerate with:
#   go build -o "$TMPDIR/trackfw-demo-bin/trackfw" ./cmd/trackfw
#   PATH="$TMPDIR/trackfw-demo-bin:$PATH" vhs demo.tape
#
# The sequence is the README Quick start, pinned non-interactively by
# internal/commands/quickstart_e2e_test.go (TestQuickStartEndToEnd). Here it runs in a real
# terminal, so `adr new` and `req new` open their wizards and the answers are typed below.
# Everything runs in a throwaway directory with HOME isolated: `trackfw init` never touches
# the real ~/.claude.

Output docs/demo.gif

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
Type `cd "$(mktemp -d)" && export HOME="$PWD/.home" && mkdir -p "$HOME" && git init -q -b main && export PS1='$ '`
Enter
Sleep 300ms
# `trackfw init` is an interactive wizard too; the demo runs it with defaults, off camera.
Type `trackfw init --ai-tools claude --forge github --identity-preset neutral < /dev/null > /dev/null 2>&1`
Enter
Sleep 3s
Ctrl+L
Show

Type "# Project set up with: trackfw init --ai-tools claude --forge github"
Enter
Sleep 600ms

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
# description is prefilled from the title
Enter
Sleep 500ms
Type "Users must log in before reaching their dashboard."
Enter
Sleep 500ms
Type "Login with email and password works"
Enter
Sleep 800ms
# ADR scope, then the domain questions: accept the defaults
Enter
Sleep 700ms
Enter
Sleep 700ms
Enter
Sleep 2s
Ctrl+L
Sleep 300ms

Type "# 3. Start the work and check the chain"
Enter
Type "trackfw roadmap move ROADMAP-$(date +%F)-user-authentication.md wip"
Enter
Sleep 1.5s
Type "trackfw validate"
Enter
Sleep 2.5s
Ctrl+L
Sleep 300ms

Type "# 4. Your AI agent tries to push straight to main. The hook says no:"
Enter
Type `echo '{"tool_name":"Bash","tool_input":{"command":"git push origin main"}}' | trackfw guard git-branch >/dev/null; echo "exit=$?"`
Enter
Sleep 4s

Type "trackfw status"
Enter
Sleep 4s
