# Medição: Guard global quando `HOME` ≠ `USERPROFILE` (Item 1, REQ-2026-09-01)

> Data: 2026-10-14 | Medição por: artemis-tf | VM: Windows-Lab (ARM64)
> REQ: `docs/req/REQ-2026-09-01-trackfw-escreve-e-audita-guard-global-em-caminho-que-o-agente-real-nunca-le-instalacao-fantasma-sob-home-diferente-de-userprofile.md`

## Contexto e Nota de Ambiente

Continuação da medição de 2026-10-12. Naquela sessão o Claude Code não estava instalado na VM,
impedindo confirmar o AC1. Esta medição completa o item com Claude Code 2.1.299 instalado.

Binário: `trackfw 9.5.0` compilado no worktree `v9.5.0` (`a1cc6e9`) recriado durante esta sessão
(o worktree de 2026-10-12 havia sido removido). Go `1.27.0 windows/arm64`.

**`brew install coreutils`:** instalado nesta sessão no Mac host para obter `gtimeout` (não
existia antes). Não foi desinstalado — registrado aqui para transparência.

**`C:\Users\Lab\.claude\`:** criado pelo Claude Code durante esta sessão (ao rodar `claude --version`,
`claude doctor`, `claude --print`, `claude --debug-file`). Conteúdo ao final: `debug/` (2 logs),
`projects/` (6 arquivos `.jsonl` — um por invocação de `--print` ou `--debug-file`), `sessions/`
— sem `settings.json`. Todos os novos arquivos estão em `USERPROFILE/.claude/`, não em XHOME.

**HOME no Windows Registry:** `reg query HKCU\Environment /v HOME` → NOT DEFINED. `HKLM` → NOT
DEFINED. O `HOME` da VM não está definido em nenhum registro Windows.

**HOME sem herança do ssh:** `env -u HOME bash -l -c 'echo $HOME'` → `HOME=` (vazio). O Git Bash
para Windows injeta `HOME=/c/Users/Lab` a partir de `USERPROFILE` no startup, mas quando `HOME`
não é herdado do processo pai, `bash -l` não o redefine — retorna vazio. Comportamento de sessão
SSH onde `HOME` vem do ambiente do sshd.

---

## Passo 1 — Instalação do Claude Code

```
Método: npm install -g @anthropic-ai/claude-code (Node.js 24.19.0 / npm 11.17.0)

Comandos:
  npm install -g @anthropic-ai/claude-code
    → postinstall não executado (sem --allow-scripts)
  npm install -g --allow-scripts=@anthropic-ai/claude-code @anthropic-ai/claude-code
    → changed 2 packages in 3s

Verificação:
  where claude
    C:\Users\Lab\AppData\Roaming\npm\claude
    C:\Users\Lab\AppData\Roaming\npm\claude.cmd
  claude --version
    2.1.299 (Claude Code)
  claude doctor:
    Platform: win32-arm64
    Path: …\node_modules\@anthropic-ai\claude-code\bin\claude.exe  (binário nativo 255 MB)
    Config install method: unknown
    Not signed in to claude.ai
```

O binário instalado é nativo Win32/ARM64 (255 MB), **não** um wrapper Node.js.

---

## Passo 2 — Onde o trackfw 9.5.0 escreve e audita (com `HOME ≠ USERPROFILE`)

### Mecanismo de resolução no Go

`trackfw` usa o pacote `internal/homedir`, cuja função `Dir()` retorna `$HOME` quando definida e
não-vazia, e `os.UserHomeDir()` como fallback. Em Windows, `os.UserHomeDir()` retorna `USERPROFILE`.

```go
// internal/homedir/homedir.go
func Dir() (string, error) {
    if h := os.Getenv("HOME"); h != "" {
        return h, nil
    }
    return os.UserHomeDir()
}
```

### Git Bash (não-login), `trackfw init` com `HOME=TMPHOME`

```
HOME=/c/Users/Lab/homeX_measure_9.5.0_NNN   USERPROFILE=C:\Users\Lab

trackfw 9.5.0 init --ai-tools claude --forge github:
  Destino (global): ~/.claude/agents/…  ~/.claude/skills/…
  ✓ claude agents and skills
  ✓ trackfw initialized

Verificação:
  TMPHOME/.claude/agents/  → ENCONTRADO (12 agentes)
  TMPHOME/.claude/skills/  → ENCONTRADO (18 diretórios)
  TMPHOME/.claude/settings.json → NÃO ENCONTRADO (init não cria settings.json)
  USERPROFILE/.claude/settings.json → NÃO ENCONTRADO
```

### Git Bash (não-login), `trackfw update harness --install-missing` (PATH=9.5.0)

```
HOME=/c/Users/Lab/homeX_fp_1594   USERPROFILE=C:\Users\Lab

trackfw update harness --install-missing:
  ✓ claude-credential-guard: updated (~/.claude/settings.json)
  ✓ claude-git-branch-guard: updated (~/.claude/settings.json)
  ✓ claude-agents: updated (~/.claude/agents)
  ✓ claude-skills: updated (~/.claude/skills)
  ✓ git-branch-guard-script: updated (~/.trackfw/scripts/…)
  updated=36 skipped=0 missing=0 failed=0

Caminhos reais criados:
  XHOME/.claude/settings.json → ENCONTRADO (guard instalado aqui)
  USERPROFILE/.claude/settings.json → NÃO ENCONTRADO (não tocado)
```

O `~` nos caminhos reportados pelo trackfw resolve para `XHOME`, não para `USERPROFILE`.

### Git Bash (não-login), `trackfw validate` (PATH=9.5.0, após install-missing)

```
HOME=/c/Users/Lab/homeX_fp_1594   USERPROFILE=C:\Users\Lab

trackfw validate:
  ⚠  adr_dir "docs/adr" does not exist
  1 warning(s)
```

**Zero violações.** O `validateGuardGlobalHookResolvable` (`validator_git_branch_guard.go:162`)
usa `homedir.Dir()` e encontrou o `settings.json` em XHOME — reportou guard como saudável.
Nenhum finding sobre `HOME ≠ USERPROFILE` (AC3/AC4 não implementados).

### cmd.exe, `trackfw 9.5.0` com `HOME=C:\Users\Lab\homeX_cmd950` (PATH=9.5.0)

```
Contexto cmd.exe:
  HOME=C:\Users\Lab (padrão) → node os.homedir() = C:\Users\Lab
  Após set HOME=C:\Users\Lab\homeX_cmd950:
    HOME=C:\Users\Lab\homeX_cmd950   USERPROFILE=C:\Users\Lab

trackfw 9.5.0 init:
  ✓ claude agents and skills  ✓ trackfw initialized
  XHOME\.claude\settings.json → NAO ENCONTRADO (init não cria)

trackfw 9.5.0 update harness --install-missing:
  updated=34 skipped=2 missing=0 failed=0
  XHOME\.claude\settings.json → ENCONTRADO

trackfw 9.5.0 validate:
  ✓ No violations found.   ← guard reportado saudável

node os.homedir() em cmd.exe (padrão e HOME diferente):
  → C:\Users\Lab (ambos os casos)
```

### PowerShell, `trackfw 9.5.0` com `HOME=C:\Users\Lab\homeX_ps95` (PATH=9.5.0)

```
HOME=C:\Users\Lab\homeX_ps95_5464   USERPROFILE=C:\Users\Lab

trackfw init: ✓ claude agents and skills
  XHOME\.claude\settings.json → NÃO ENCONTRADO

trackfw update harness --install-missing:
  updated=34 skipped=2 missing=0 failed=0
  XHOME\.claude\settings.json → ENCONTRADO

trackfw validate (PATH=9.5.0, XHOME):
  ⚠  adr_dir "docs/adr" does not exist
  1 warning(s)                      ← ZERO VIOLAÇÕES sobre guard

  (Medição anterior com PATH errado produziu 4 violations — Python em PATH sem subcomando guard;
   descartada. Medição definitiva: PATH=9.5.0, resultado: zero violações sobre guard.)

node os.homedir() (PowerShell, HOME diferente):
  → C:\Users\Lab
```

---

## Passo 3 — Onde o Claude Code lê o `settings.json` global

### 3a — Evidência direta (debug file, sem login)

Dois experimentos com `HOME = XHOME ≠ USERPROFILE`:

**Experimento A** (medição 3a isolada, Git Bash):
```
HOME=/c/Users/Lab/homeX_3a_test_1767   USERPROFILE=C:\Users\Lab

claude --debug-file "$XHOME/claude_debug.txt" --print "test"
  → "Not logged in · Please run /login"
  → XHOME/.claude/ NÃO CRIADO
  → USERPROFILE/.claude/debug/ sem novos arquivos
```

**Experimento B** (no contexto do falso positivo, Git Bash):
```
HOME=/c/Users/Lab/homeX_fp_1594   USERPROFILE=C:\Users\Lab
(XHOME/.claude/settings.json já instalado pelo trackfw)

claude --debug-file "$XHOME/dbg_fp.txt" --print "test"
  → "Not logged in · Please run /login"
```

Linhas relevantes do debug file (Experimento B):
```
[DEBUG] Broken symlink or missing file encountered for settings.json at path:
        C:\Users\Lab\.claude\settings.json
[DEBUG] Broken symlink or missing file encountered for settings.json at path:
        C:\Program Files\ClaudeCode\managed-settings.json
[DEBUG] Loading skills from: user=C:\Users\Lab\.claude\skills, project=[]
[DEBUG] Failed to stat directory C:\Users\Lab\.claude\agents
[DEBUG] Failed to stat directory C:\Users\Lab\.claude\commands
```

**Evidência direta:** com `HOME=XHOME` e guard instalado em `XHOME/.claude/settings.json`, o
Claude Code 2.1.299 reportou `missing file` para `C:\Users\Lab\.claude\settings.json` (USERPROFILE)
e buscou agents/skills em `C:\Users\Lab\.claude\` — nunca referenciou XHOME.

### 3a — Confirmação em PowerShell e cmd.exe

Mesma metodologia (`claude --debug-file <path> --print "test"` com `HOME=XHOME`):

**PowerShell** (`HOME=C:\Users\Lab\homeX_final2_ps`, `USERPROFILE=C:\Users\Lab`):
```
Debug file gerado em XHOME (conforme --debug-file).
Linhas relevantes:
  Broken symlink or missing file encountered for settings.json at path:
      C:\Users\Lab\.claude\settings.json          ← USERPROFILE
  Loading skills from: user=C:\Users\Lab\.claude\skills
  Failed to stat directory C:\Users\Lab\.claude\agents
```

**cmd.exe** (`HOME=C:\Users\Lab\homeX_cmd_final2`, `USERPROFILE=C:\Users\Lab`):
```
Debug file gerado em XHOME (conforme --debug-file).
Linhas relevantes:
  Broken symlink or missing file encountered for settings.json at path:
      C:\Users\Lab\.claude\settings.json          ← USERPROFILE
  Loading skills from: user=C:\Users\Lab\.claude\skills
  Failed to stat directory C:\Users\Lab\.claude\agents
```

Resultado idêntico ao Git Bash — os três terminais confirmam `USERPROFILE` por evidência direta (3a).

### 3b — Documentação oficial / `CLAUDE_CONFIG_DIR`

Não consultado. O método 3a deu evidência direta em todos os terminais; a tarefa manda parar no primeiro com resultado.

### 3c — Proxy Node.js (corroboração)

`node -e "console.log(require('os').homedir())"` retornou `C:\Users\Lab` em todos os terminais e
todos os valores de `HOME` testados:

| Terminal | HOME testado | `node os.homedir()` |
|---|---|---|
| Git Bash (não-login) | `/c/Users/Lab` (padrão) | `C:\Users\Lab` |
| Git Bash (não-login) | `/c/Users/Lab/homeX` (POSIX diferente) | `C:\Users\Lab` |
| Git Bash (não-login) | `C:\Users\Lab\homeX` (Windows diferente) | `C:\Users\Lab` |
| Git Bash (não-login) | `""` (vazio) | `C:\Users\Lab` |
| Git Bash (não-login) | (unset) | `C:\Users\Lab` |
| Git Bash (login) | `/c/Users/Lab` (mesmo que não-login nesta VM) | `C:\Users\Lab` |
| PowerShell | `C:\Users\Lab` (padrão) | `C:\Users\Lab` |
| PowerShell | `C:\Users\Lab\homeX_ps` (diferente) | `C:\Users\Lab` |
| cmd.exe | `C:\Users\Lab` (padrão) | `C:\Users\Lab` |
| cmd.exe | `C:\Users\Lab\homeX_cmd` (diferente) | `C:\Users\Lab` |

**Nota:** este é proxy — Claude Code 2.1.299 é nativo e não roda em Node.js. O resultado
corrobora o 3a mas não substitui.

---

## Passo 4 — Veredito do AC1: onde cada componente lê/escreve

### Mecanismo de resolução por componente

| Componente | Código (arquivo:linha) | Resolve para |
|---|---|---|
| `trackfw init`, `update harness` (escritor) | `internal/homedir/homedir.go`: `$HOME` first, `os.UserHomeDir()` fallback | `HOME` (se definido e não-vazio) |
| `validateGuardGlobalHookResolvable` (auditor global) | `validator_git_branch_guard.go:162` — `homedir.Dir()` | `HOME` (consistente com escritor) |
| `validateClaudeGuardHookMatcherGlobalWarnings` (só warnings) | `validator_credential_guard.go:1175` — `os.UserHomeDir()` | `USERPROFILE` |
| Claude Code 2.1.299 | Binário nativo — debug file: `C:\Users\Lab\.claude\*` imune a `HOME` | `USERPROFILE` (imune a `HOME`) |

O escritor e o auditor principal do trackfw são **consistentes entre si** (ambos `homedir.Dir()`).
A divergência é entre o **par trackfw e o Claude Code**: trackfw opera em `HOME`, Claude Code em
`USERPROFILE`.

### Tabela por terminal

| Terminal | `HOME` padrão na VM | trackfw 9.5.0 escreve settings.json em | `validateGuardGlobalHookResolvable` audita em | Claude Code lê em (evidência) | Diverge? |
|---|---|---|---|---|---|
| Git Bash (não-login) | `/c/Users/Lab` (POSIX = `C:\Users\Lab`) | `C:\Users\Lab\.claude\` | `C:\Users\Lab\.claude\` | `C:\Users\Lab\.claude\` (3a debug) | **Não** (default VM) |
| Git Bash (HOME≠USERPROFILE, issue #216) | `/c/Users/Lab/homeX` | `C:\Users\Lab\homeX\.claude\` | `C:\Users\Lab\homeX\.claude\` | `C:\Users\Lab\.claude\` (3a debug) | **SIM** |
| Git Bash (login) | `/c/Users/Lab` (mesmo que não-login nesta VM) | `C:\Users\Lab\.claude\` | `C:\Users\Lab\.claude\` | `C:\Users\Lab\.claude\` (3c proxy) | **Não** |
| PowerShell | `C:\Users\Lab` | `C:\Users\Lab\.claude\` | `C:\Users\Lab\.claude\` | `C:\Users\Lab\.claude\` (3a debug) | **Não** |
| PowerShell (HOME≠USERPROFILE) | `C:\Users\Lab\homeX` | `C:\Users\Lab\homeX\.claude\` | `C:\Users\Lab\homeX\.claude\` | `C:\Users\Lab\.claude\` (3a debug) | **SIM** |
| cmd.exe | `C:\Users\Lab` | `C:\Users\Lab\.claude\` | `C:\Users\Lab\.claude\` | `C:\Users\Lab\.claude\` (3a debug) | **Não** (default VM) |
| cmd.exe (HOME≠USERPROFILE) | `C:\Users\Lab\homeX_cmd950` | `C:\Users\Lab\homeX_cmd950\.claude\` | `C:\Users\Lab\homeX_cmd950\.claude\` | `C:\Users\Lab\.claude\` (3a debug) | **SIM** |

Evidência 3a (debug file) confirmada nos três terminais. O mecanismo é de sistema operacional:
o binário nativo Win32/ARM64 usa a API Windows para resolver o diretório home, ignorando `HOME`.

### Falso positivo de saúde — observado

Medição combinada (Git Bash, PATH=9.5.0, HOME=XHOME, diretório de projeto vazio):

```
1. trackfw update harness --install-missing:  updated=36 missing=0 failed=0
   XHOME/.claude/settings.json → CRIADO (guard instalado por trackfw)
   USERPROFILE/.claude/settings.json → NÃO EXISTE (não tocado)

2. trackfw validate:
   ⚠  adr_dir "docs/adr" does not exist
   1 warning(s)                     ← ZERO VIOLAÇÕES sobre guard

3. claude --debug-file (mesma sessão, mesmo HOME=XHOME):
   [DEBUG] Broken symlink or missing file encountered for settings.json at path:
           C:\Users\Lab\.claude\settings.json   ← Claude Code lê USERPROFILE
   (guard em XHOME/.claude/settings.json — nunca referenciado)
```

Confirmado também em PowerShell (HOME=C:\Users\Lab\homeX_ps95_5464, PATH=9.5.0):
```
trackfw validate:
  ⚠  adr_dir "docs/adr" does not exist
  1 warning(s)                      ← ZERO VIOLAÇÕES sobre guard

claude --debug-file (mesma sessão, mesmo HOME=XHOME):
  Broken symlink or missing file encountered for settings.json at path:
      C:\Users\Lab\.claude\settings.json   ← Claude Code lê USERPROFILE
```

Confirmado também em cmd.exe (HOME=C:\Users\Lab\homeX_cmd_final2, PATH=9.5.0):
```
trackfw validate:
  ⚠  adr_dir "docs/adr" does not exist
  1 warning(s)                      ← ZERO VIOLAÇÕES sobre guard

claude --debug-file (mesma sessão, mesmo HOME=XHOME):
  Broken symlink or missing file encountered for settings.json at path:
      C:\Users\Lab\.claude\settings.json   ← Claude Code lê USERPROFILE
```

**Veredito:** quando `HOME ≠ USERPROFILE` fisicamente, o trackfw instala e valida o guard
em `HOME/.claude/settings.json`, reportando saúde. O Claude Code nunca lê esse arquivo — ele
buscará `USERPROFILE/.claude/settings.json`, que não existe. O guard está instalado do ponto
de vista do trackfw e ausente do ponto de vista do consumidor real. Isso é o falso positivo de
saúde descrito na REQ.

**Nota sobre o estado padrão da VM:** todos os terminais têm `HOME = USERPROFILE = C:\Users\Lab`
no padrão desta VM. O Git Bash usa a forma POSIX `/c/Users/Lab` do mesmo diretório. Não há
divergência física por padrão. O defeito ocorre quando o usuário configurou `HOME` para um
diretório diferente, como no issue #216.

---

## Passo 5 — Restauração

```
settings.json em USERPROFILE/.claude/ antes: NÃO EXISTIA
settings.json em USERPROFILE/.claude/ após: NÃO ENCONTRADO (correto, não tocado)

Verificação final de vazamento (find -newer .claude.json):
  Arquivos novos detectados em USERPROFILE/.claude/:
    debug/  — 2 arquivos (logs do --debug-file, esperado)
    projects/ — 6 arquivos .jsonl (sessões de --print/--debug-file, esperado)
  Todos estão em USERPROFILE/.claude/, não em nenhum XHOME.
  Nenhum arquivo novo detectado em .trackfw, .agents, .kiro, .codex, .aws, .config ou .cursor.
  Resultado: sem vazamento.

Processos na VM ao encerrar: claude.exe=0, trackfw.exe=0, node.exe=0.

Diretórios XHOME criados pelos testes: removidos dentro de cada script de medição.
Scripts de medição na VM: removidos (ls measure_*.sh → NOT_FOUND no final de cada etapa).

Worktree C:\Users\Lab\tf-9.5.0: mantido (criado nesta sessão, útil para medições futuras).

Mac host — brew install coreutils: instalado nesta sessão, não desinstalado.
```

---

## Tabela Final

| Terminal | `HOME` | trackfw 9.5.0 escreve settings em | trackfw audita (validate) em | Claude Code lê em (evidência) | Diverge? |
|---|---|---|---|---|---|
| Git Bash (não-login), padrão VM | `/c/Users/Lab` | `C:\Users\Lab\.claude\` | `C:\Users\Lab\.claude\` | `C:\Users\Lab\.claude\` (3a) | **Não** |
| Git Bash (não-login), HOME≠UP | `/c/Users/Lab/homeX` | `C:\Users\Lab\homeX\.claude\` | `C:\Users\Lab\homeX\.claude\` | `C:\Users\Lab\.claude\` (3a) | **SIM** |
| Git Bash (login), padrão VM | `/c/Users/Lab` | `C:\Users\Lab\.claude\` | `C:\Users\Lab\.claude\` | `C:\Users\Lab\.claude\` (3c) | **Não** |
| PowerShell, padrão VM | `C:\Users\Lab` | `C:\Users\Lab\.claude\` | `C:\Users\Lab\.claude\` | `C:\Users\Lab\.claude\` (3a) | **Não** |
| PowerShell, HOME≠USERPROFILE | `C:\Users\Lab\homeX` | `C:\Users\Lab\homeX\.claude\` | `C:\Users\Lab\homeX\.claude\` | `C:\Users\Lab\.claude\` (3a) | **SIM** |
| cmd.exe, padrão VM | `C:\Users\Lab` | `C:\Users\Lab\.claude\` | `C:\Users\Lab\.claude\` | `C:\Users\Lab\.claude\` (3a) | **Não** |
| cmd.exe, HOME≠USERPROFILE | `C:\Users\Lab\homeX_cmd950` | `C:\Users\Lab\homeX_cmd950\.claude\` | `C:\Users\Lab\homeX_cmd950\.claude\` | `C:\Users\Lab\.claude\` (3a) | **SIM** |

Legenda evidência: (3a) debug file direto — obtido nos três terminais (Git Bash, PowerShell, cmd.exe).

| Item | Resultado |
|---|---|
| Claude Code instalado | **2.1.299 (win32-arm64, npm-global)** |
| Evidência AC1 — método obtido | **3a** (debug file sem login) |
| Requer login para evidência mais direta | `claude config list` → `Not logged in · Please run /login` |
| Falso positivo de saúde | **Observado** (validate "ok", Claude Code reporta settings.json ausente em USERPROFILE) |
| validate emite finding de HOME≠USERPROFILE | **Não** (AC3/AC4 não implementados em 9.5.0) |
| Vazamento para USERPROFILE nos testes | **Não** (verificado por mtime) |
| HOME diverge fisicamente de USERPROFILE no estado padrão da VM | **Não** — mesma pasta, strings diferentes no Git Bash |

---

```
git status --short (depois de escrever este arquivo):
?? docs/portabilidade/2026-10-14-guard-global-home-userprofile-vm.md
```
