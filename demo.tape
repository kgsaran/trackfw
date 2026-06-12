Output docs/demo.gif

Require trackfw

Set Shell "bash"
Set FontSize 13
Set FontFamily "JetBrains Mono"
Set Width 920
Set Height 600
Set Theme "Catppuccin Mocha"
Set Padding 24
Set Margin 8
Set MarginFill "#1e1e2e"
Set BorderRadius 8
Set PlaybackSpeed 1.0
Set TypingSpeed 40ms

# Setup — hidden, limpa diretório e adiciona binário ao PATH
Hide
Type "export PATH=/Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw/bin:$PATH"
Enter
Sleep 200ms
Type "cd /tmp/trackfw-demo && rm -rf * .claude .cursor .github docs"
Enter
Sleep 200ms
Type "mkdir -p docs/adr docs/req docs/roadmaps/backlog docs/roadmaps/wip docs/roadmaps/done"
Enter
Sleep 300ms
Show
Ctrl+L
Sleep 500ms

# ──────────────────────────────────────────────────────
#  CENA 1: trackfw adr new
# ──────────────────────────────────────────────────────
Type "trackfw adr new "
Sleep 200ms
Type `"Use PostgreSQL as primary database"`
Sleep 400ms
Enter
Sleep 2s

# Context
Type "We need persistent storage with JSONB and full-text search support."
Enter
Sleep 1.2s

# Decision
Type "Adopt PostgreSQL 16 as the primary database."
Enter
Sleep 1.2s

# Consequences
Type "Strong JSONB support and ecosystem. Requires managed hosting."
Enter
Sleep 1.2s

# Alternatives
Type "MySQL lacks JSONB. SQLite not suitable for production."
Enter
Sleep 2.5s

# ──────────────────────────────────────────────────────
#  CENA 2: trackfw req new — ADR discovery
# ──────────────────────────────────────────────────────
Type `trackfw req new "User login"`
Sleep 400ms
Enter
Sleep 2s

# Form 1 — Title (pre-filled) + Motivation
# Tab para avançar para o campo Motivation
Tab
Sleep 400ms
Type "Users need to authenticate to access protected resources."
Enter
Sleep 2s

# Form 2 — Criteria
Type "Users can log in with email and password"
Enter
Sleep 1.2s

# Linked ADR
Type "ADR-2026-06-12-use-postgresql-as-primary-database.md"
Tab
Sleep 300ms
# Linked Roadmap (vazio)
Enter
Sleep 1.5s

# Probe: How will users authenticate? → Down x3 = "Not decided yet"
Down
Down
Down
Sleep 600ms
Enter
Sleep 1.5s

# Probe: How will sessions be managed? → Enter = JWT (default, decided)
Enter
Sleep 2.5s

# ──────────────────────────────────────────────────────
#  CENA 3: trackfw validate
# ──────────────────────────────────────────────────────
Type "trackfw validate"
Sleep 400ms
Enter
Sleep 2.5s

# ──────────────────────────────────────────────────────
#  CENA 4: trackfw status
# ──────────────────────────────────────────────────────
Type "trackfw status"
Sleep 400ms
Enter
Sleep 3s
