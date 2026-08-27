# Barreira ML-3A — trackfw audit-surface

> Agente: hades-tf | Data: 2026-08-27 | Roadmap: ROADMAP-2026-08-26-comando-que-audita-a-superficie-executavel-de-um-checkout-de-pr.md

**VEREDITO: REPROVADO em AC14 para duas formas de comando. Três achados de segurança. Nenhum bloqueia a entrega no repositório trackfw (as formas de comando usadas aqui são cobertas), mas os dois AC14 parciais devem ser registrados como resíduos antes do merge e corrigidos numa REQ própria.**

---

## Metodologia

Cada resposta identifica se a evidência é **medida** (saída de comando executada nesta sessão)
ou **raciocínio** (inferência da leitura do código). Nenhum julgamento foi emitido antes da
medição correspondente.

---

## Q1 — Enumeration completeness: a implementação cobre o que o modelo de ameaça enumerou?

**MEDIDO.**

### Cobertos

| Surface | Implementação | Evidência |
|---------|--------------|-----------|
| 8 runtimes (claude, codex, gemini, copilot, cursor, kiro, windsurf, amazonq) | `canonicalRuntimes` em `internal/auditsurface/auditsurface.go` | saída de `trackfw audit-surface HEAD` em HEAD real: 8 linhas `absent`/`hook` |
| Paths ausentes reportados como `absent` | AC13 implementado via fallback | `absent [copilot] .github/hooks/trackfw-attention.json` no relatório HEAD real |
| Arquivos de instrução (CLAUDE.md, AGENTS.md, GEMINI.md, .windsurfrules, .github/copilot-instructions.md, .amazonq/developer/guidelines.md, .cursor/rules/trackfw.mdc) | `instructionFilePaths` | relatório HEAD: 7 linhas `instruction [absent/present]` |
| Slash commands (.claude/commands/**/*.md) | `gitLsTree` | medido em HEAD com 1+ slash command |
| Lifecycle hooks npm (preinstall, postinstall, prepare) | `auditLifecycleHooks` em `npm/package.json` | 3 linhas `lifecycle [absent]` no relatório |
| .husky/pre-commit | `auditLifecycleHooks` | linha `lifecycle [absent] .husky/pre-commit` |
| AC12: sem checkout do worktree | `git show <ref>:<path>` | `git status` antes/depois idêntico (medido na auditoria ML-1A) |
| AC16: sem varredura de conteúdo, apenas paths exatos | declarado em comentário AC16 em `auditsurface.go`; `docs/cli-parity.md` e `internal/generators/agentfiles.go` não aparecem no relatório | medido em HEAD real: 0 ocorrências de FP |
| AC6: wiring legítimo sem hooks não acusado | `no_hooks [claude] .claude/settings.json` para settings.json com só `permissions` | MEDIDO: fixture `{"permissions": {"allow": [...]}}` → `no_hooks [claude]` ✅ |

### Gaps não cobertas (declaradas ou novas)

**Gap A — `.vscode/tasks.json`:** O modelo de ameaça (§1 Wave 0) disse explicitamente "deve estar
no inventário do Wave 1 mesmo ausente aqui, pelo mesmo motivo dos 5 runtimes de hook ausentes."
A implementação não varre `.vscode/tasks.json`. Classificação: contradição doc/impl.
A threat model diz "Wave 1"; o Wave 1 não implementou. Não bloqueia o repositório trackfw
(o arquivo não existe aqui), mas o inventário prometido não foi entregue.

**Gap B — `npm/package.json` hardcoded vs. `package.json` na raiz:** `auditLifecycleHooks`
chama `gitShow(ref, "npm/package.json", ...)`. Para qualquer repositório que não seja trackfw
e que tenha um `package.json` na raiz com `postinstall`, o braço de lifecycle é vacuosamente
vazio enquanto o relatório ainda imprime três linhas `lifecycle [absent]` tranquilizadoras.
Isso é exatamente o §2.6 do modelo de ameaça ("o implementador lê o inventário deste
repositório como o inventário completo"). Não bloqueia o trackfw, mas é uma limitação
estrutural do comando quando usado em outros projetos.

**Gap C — contradição doc `.envrc`/`pyproject.toml`/devcontainer:** declarados fora do escopo
na REQ. Nenhuma ação necessária aqui.

---

## Q2 — As três variantes de ataque A, B, C são explicitamente capturadas?

**A e C: MEDIDO. B: MEDIDO.**

### Variante A — apenas o conteúdo do script muda, wiring inalterado

**Para scripts com extensão `.sh` e sem argumentos: CAPTURADA.** Medido via digest do
`trackfw-git-branch-guard.sh` em dois commits reais da história do repositório:

```
615f8f9  sha256:bd144a3f85c1ab0f
7132fc5  sha256:f2e80b0fa9a48fcc
```

Gate FN-2 falsifica em ambas as direções com `AUDIT_SURFACE_SELFTEST_BREAK=A`.

**Para scripts fora do whitelist de `normalizeCommand`: NÃO CAPTURADA. Ver Achado F1.**

### Variante B — wiring reaponta para outro script

**CAPTURADA. Medido.** Fixture: REF1 wiring → `scripts/a.sh`, REF2 wiring → `scripts/b.sh`.

```
REF1: hook [claude] .claude/settings.json PreToolUse/Bash scripts/a.sh sha256:8cfa61fc...
REF2: hook [claude] .claude/settings.json PreToolUse/Bash scripts/b.sh sha256:314aa7bf...
```

Ambos `RawCommand` e `Digest` mudam. Detectado por qualquer comparação dos relatórios.
Não há gate FN dedicado para esta variante, mas os campos do tuple a expõem.

### Variante C — matcher alargado de "Bash" para "*"

**CAPTURADA. Medido** via gate FN-4 (medição da auditoria ML-2A): o campo `matcher` no
tuple muda, o relatório muda, o gate detecta.

---

## Q3 — Risco de falso-positivo além das duas fixtures?

**RACIOCÍNIO + MEDIDO para AC6.**

A proteção estrutural é o AC16: `auditsurface.go` nunca faz `grep` em conteúdo de arquivo.
Abre **apenas** os 8 paths exatos de wiring, os 7 paths de instrução, e
`.claude/commands/**/*.md`. Qualquer arquivo que mencione caminhos de hook como string
(docs, geradores, scripts do gate) é estruturalmente excluído.

**Medido:** em HEAD real, `docs/cli-parity.md` e `internal/generators/agentfiles.go` não
aparecem no relatório (FP-1 e FP-2 do gate confirmam nas duas direções).

**AC6 medido:** settings.json com apenas `{"permissions": {"allow": [...]}}` (sem chave
`hooks`) → relatório imprime `no_hooks [claude] .claude/settings.json`, nenhuma linha `hook`.

Não há risco adicional de falso-positivo identificado além das fixtures já cobertas.

---

## Q4 — `AUDIT_SURFACE_SELFTEST_BREAK` representa risco de CI ou de artefato?

**MEDIDO.**

**Risco de bypass: ZERO.** A variável, quando definida, força o gate a falhar — nunca a passar
com prova suprimida. Definir `AUDIT_SURFACE_SELFTEST_BREAK=A` em CI garante que o cenário
FN-2 **falha**, não que passa sem verificar.

**Valor não reconhecido (C, lowercase, etc.):** `SELFTEST_BREAK="${AUDIT_SURFACE_SELFTEST_BREAK:-}"`.
Um valor não reconhecido entra em nenhum dos dois ramos `if/elif`, nenhum binário sabotado é
construído, e o gate roda com o binário real. Medido: `AUDIT_SURFACE_SELFTEST_BREAK=C` →
gate passa com todos os 7 cenários usando o binário real. Comportamento correto (não suprime
nenhuma prova), mas silencioso.

**Limpeza:** o binário sabotado é construído em `$WORK` (mktemp) e apagado por
`trap 'chmod -R u+w "$WORK" 2>/dev/null; rm -rf "$WORK"' EXIT`. Nenhum artefato persiste.

**Presença em CI/Makefile:** grep em Makefile, `.github/` e `scripts/` confirma que a variável
aparece apenas em `check-audit-surface.sh` (comentário + código) e em
`check-gates-falsify.sh` (que a passa explicitamente como `A` e `B` para falsificar o próprio gate).
Não está definida em nenhum workflow de CI nem no Makefile.

**Risco residual:** um PR que altere `SELFTEST_BREAK="${AUDIT_SURFACE_SELFTEST_BREAK:-A}"` (default
hard-coded) desabilitaria permanentemente o FN-2 enquanto o gate parece funcionar. Mitigação:
code review da linha `check-audit-surface.sh:73`.

---

## Q5 — O comando produz saída enganosa em casos hostis ou degenerados?

**MEDIDO. Este é o achado central da barreira.**

### Q5-a: ref inválido

Comando retorna `Error: audit-surface: ref does not resolve` + exit 1. Não enganoso. ✅

### Q5-b: fora de repositório git

Retorna `Error: not inside a git repository` + exit 1. Não enganoso. ✅

### Q5-c: JSON malformado no wiring file

Relatório imprime `parse-error/*` com a mensagem do erro. A linha é inconfundível com um hook
legítimo. Não enganoso. ✅

### Q5-d: submodule na árvore

Medido: o submodule não interfere. Hooks do repo-pai são reportados normalmente. ✅

### Q5-e: script como symlink — ACHADO F2 (HIGH)

**MEDIDO. Falso-negativo confirmado.**

Fixture: `.claude/settings.json` aponta para `scripts/link.sh`; `link.sh` é um symlink para
`real.sh`. Git armazena um symlink como um blob cujo conteúdo é a string do path de destino.

`git show HEAD:scripts/link.sh` retorna a string literal `real.sh` (sem newline).

`auditsurface.go` recebe essa string, computa SHA256, e reporta:

```
hook [claude] .claude/settings.json PreToolUse/Bash scripts/link.sh sha256:d7b138374e...
```

`sha256("real.sh") = d7b138374e701b5c7eb55f6ffbcffc56ee27502d936409f39f84f1789843fc7d` — confirmado.

**O ataque:** REF1 e REF2 diferem apenas em `real.sh`. Symlink inalterado.

```
REF1 digest: sha256:d7b138374e...
REF2 digest: sha256:d7b138374e...   <- IDENTICOS
```

O relatório mostra um `sha256:` estável e presente. O mantenedor conclui "script inalterado."
O conteúdo que realmente executa mudou. Este é o pior modo de falha: presente-e-estável
engana mais que ausente.

**Severidade: HIGH.** Não bloqueia o repositório trackfw (hooks não usam symlinks aqui),
mas o relatório mentiria silenciosamente para qualquer repositório que os use.

**Handoff:** `internal/auditsurface/auditsurface.go`, função `gitShow`. Após obter o conteúdo via
`git show <ref>:<path>`, detectar se o resultado é uma linha curta sem newline que corresponde a um
path relativo válido (ou verificar o tipo do objeto via `git cat-file -t <ref>:<path>`). Se o blob é
do tipo `blob` e contém apenas uma linha sem espaços que tem extensão de script, pode ser symlink
— tratar com `git show <ref>:<symlink-target>` para obter o conteúdo real.

### Q5-f: SHA de outro repositório — ACHADO F3 (MEDIUM)

**MEDIDO. Comportamento enganoso confirmado.**

`validateRef` chama `git rev-parse --verify <ref>`. Para uma string hexadecimal de 40 caracteres,
git aceita o formato sem confirmar que o objeto existe no repositório atual:

```
git rev-parse --verify a85c1b7d49431e4a8d4346428f59c0ec4e131105
a85c1b7d49431e4a8d4346428f59c0ec4e131105   # stdout, exit 0

git rev-parse --verify a85c1b7d49431e4a8d4346428f59c0ec4e131105^{commit}
fatal: Needed a single revision             # exit 128 — objeto não existe
```

O comando prossegue, varre todos os paths, não encontra nenhum (todos `absent`), e retorna:

```
trackfw audit-surface: 0 hook tuple(s) at a85c1b7d49431e4a8d4346428f59c0ec4e131105

absent [claude] .claude/settings.json
...
```

Exit 0. O relatório tem aparência de "nenhuma superfície neste ref" quando o ref pertence a outro
repositório e a query foi vacuosamente vazia.

**Severidade: MEDIUM.** Confusão de refs entre repositórios é incomum no workflow de auditoria
de PR. O relatório não afirma segurança positiva (só `absent`), mas um mantenedor desatento
pode confundir "0 hook tuples" com "não há hooks" em vez de "ref inválido."

**Handoff:** `internal/commands/audit_surface.go`, função `validateRef` (linha ~95).
Substituir `git rev-parse --verify <ref>` por `git rev-parse --verify <ref>^{commit}` ou
`git cat-file -e <ref>^{commit}`. Ambos retornam exit não-zero para objetos ausentes.

---

## Achado Central — F1 (HIGH): normalizeCommand bloqueia AC14 para formas comuns de comando

**MEDIDO.** Este achado é a falha mais ampla encontrada.

`normalizeCommand` em `internal/auditsurface/auditsurface.go` retorna `""` para qualquer
comando que:
- Não tenha extensão `.sh`, `.py` ou `.js`, **ou**
- Contenha um espaço (i.e., tenha argumentos), **ou**
- Comece com um interpretador (e.g., `bash`, `python3`, `node`).

Quando `normalizeCommand` retorna `""`, `RunAuditSurface` salta `gitShow` e escreve
`Digest = "unresolvable"`. O digest permanece `"unresolvable"` indefinidamente,
independente de qualquer mudança no conteúdo do script.

**Três casos medidos:**

```
# Caso 1: extensão .bash
REF1: hook [claude] .claude/settings.json PreToolUse/Bash scripts/hook.bash unresolvable
REF2: hook [claude] .claude/settings.json PreToolUse/Bash scripts/hook.bash unresolvable
(real.sh mudou de "version 1" para "version 2 HOSTILE" — não detectado)

# Caso 2: comando com argumento
REF1: hook [claude] .claude/settings.json PreToolUse/Bash scripts/hook.sh --strict unresolvable
REF2: hook [claude] .claude/settings.json PreToolUse/Bash scripts/hook.sh --strict unresolvable
(hook.sh mudou — não detectado)

# Caso 3: prefixo de interpretador
REF1: hook [claude] .claude/settings.json PreToolUse/Bash bash scripts/hook.sh unresolvable
REF2: hook [claude] .claude/settings.json PreToolUse/Bash bash scripts/hook.sh unresolvable
(hook.sh mudou — não detectado)
```

O hook É reportado (o `RawCommand` aparece), mas o digest é imutável. Variante A não é
detectável para essas formas de comando.

**Por que isso importa:** `bash scripts/hook.sh` e `scripts/hook.sh --strict` são formas
ordinárias de comando em wiring files reais. O digest "unresolvable" não sinaliza "não
consegui verificar" de forma distinta de "conteúdo inalterado"; ambos aparecem como uma
string na mesma coluna. Um mantenedor que vê `unresolvable` pela primeira vez pode não
saber o que isso significa.

**Escopo do impacto no trackfw:** o hook real do repositório é
`$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh`, que `normalizeCommand` resolve
para `scripts/trackfw-git-branch-guard.sh` (extensão `.sh`, sem espaço). O digest é
calculado corretamente. **F1 não bloqueia o trackfw neste momento**, mas AC14 não é
satisfeita como escrita para formas fora do whitelist.

**Handoff:** `internal/auditsurface/auditsurface.go`, função `normalizeCommand`. Ampliar:
- Suporte a `.bash`, `.zsh`, `.rb`, `.pl`, `.fish` como extensões adicionais.
- Extrair o path do script de comandos com prefixo de interpretador
  (`bash scripts/hook.sh` → `scripts/hook.sh`).
- Extrair o path base de comandos com argumentos
  (`scripts/hook.sh --strict` → `scripts/hook.sh`).
- Para comandos genuinamente irresolúveis (e.g., builtins, pipes), reportar `unresolvable`
  com um marcador distinto (e.g., `unresolvable[builtin]`) para diferenciar de falha de
  normalização.

---

## Resumo dos achados

| ID | Severidade | Superfície | Forma medida | Bloqueia trackfw? |
|----|-----------|-----------|-------------|-------------------|
| F1 | HIGH | `normalizeCommand` whitelist | `.bash`, `cmd --arg`, `bash cmd` → digest permanentemente `unresolvable` | Não (hooks do repo usam `.sh` sem args) |
| F2 | HIGH | Symlink como script | `link.sh → real.sh`: digest é hash do target string, não do conteúdo executado | Não (hooks não são symlinks aqui) |
| F3 | MEDIUM | `validateRef` com SHA de outro repo | bare `--verify` aceita SHA de 40 hex sem confirmar existência → relatório vazio + exit 0 | Baixo (confusão de ref, não bypass de segurança) |

---

## Veredito por AC

| AC | Status | Evidência |
|----|--------|-----------|
| AC1–AC11 (REQ) | MEDIDO ✅ (com ressalva F1 em AC14) | gate 7/7 + audit de HEAD real |
| AC12: sem checkout | ✅ | `git status` antes/depois idêntico |
| AC13: ausência é informação | ✅ | 8 linhas `absent` para runtimes não configurados |
| AC14: tuple completo, qualquer componente | PARCIAL ⚠️ | Variante A detectada para `.sh`; não detectada para formas F1/F2 |
| AC15: instrução vs. execução | ✅ | rótulos `instruction` e `hook` distintos |
| AC16: sem grep de conteúdo | ✅ | FP-1/FP-2 + medição em HEAD real |
| AC5/AC6: nomear sem julgar | ✅ | AC6 medido: `no_hooks` para wiring sem hooks |

---

## Declaração de resíduos aceitos para o repositório trackfw

1. **F1 e F2** são resíduos aceitos para esta entrega no repositório trackfw porque as formas de
   comando usadas aqui são cobertas. Devem ser corrigidos numa REQ própria antes de promover
   `audit-surface` como ferramenta genérica.
2. **F3** é resíduo aceito com impacto baixo; a mitigação é trivial (`^{commit}`) e deve ser
   incluída na mesma REQ de F1/F2.
3. **`.vscode/tasks.json`** (Gap A): o inventário prometido pelo modelo de ameaça não foi
   entregue. Adicionar à REQ de F1/F2.
4. **`package.json` na raiz** (Gap B): limitação estrutural para repositórios externos. Adicionar
   à REQ de F1/F2.

---

## Reverificação — ML-3B (2026-08-27, hades-tf)

> Escopo: confirmar ou negar os três achados pós-ML-1B; medir formas extras de F1 não cobertas pelo
> gate; medir edge cases de F2 (symlink fora do repo, cadeia, circular, alvo ausente); verificar
> `package.json` por descoberta (falso-positivo); regressão nos 17 cenários.

**VEREDITO: APROVADO. Os três achados originais estão fechados. Três resíduos novos declarados.**

O bloqueio da entrega é levantado.

---

### Achados originais — status pós-ML-1B

#### F1 — `normalizeCommand` (original: 3 formas) — FECHADO

Medido via gate FN-F1a/b/c (17/17 passam). Os três casos medidos na barreira original:

```
.bash        → digest MUDA entre refs  (antes: "unresolvable" fixo)
cmd --arg    → digest MUDA entre refs  (antes: "unresolvable" fixo)
bash cmd     → digest MUDA entre refs  (antes: "unresolvable" fixo)
```

Adicionalmente, formas não cobertas pelo gate — medidas agora:

| Forma | Resultado | Conclusão |
|-------|-----------|-----------|
| `./scripts/hook.sh` | digest muda (sha256:eb4fe7 → sha256:39ab98) | FECHADO — `./` não bloqueia a extensão |
| `/usr/local/bin/hardcoded.sh` | `not-found` | CORRETO — path absoluto não existe no repo |
| `env LOG=1 scripts/hook.sh` | `unresolvable` (estável) | RESÍDUO NOVO R1 — ver abaixo |
| `scripts/sub dir/hook.sh` | `unresolvable` (estável) | RESÍDUO NOVO R2 — ver abaixo |

#### F2 — symlink como script (original: link→real) — FECHADO para forma básica

Gate FN-F2 passa. Relatório correto para o caso original:

```
REF1: symlink->real.sh|sha256:8bb7138…
REF2: symlink->real.sh|sha256:335a931…   <- MUDA quando real.sh muda
```

Edge cases medidos agora (não cobertos pelo gate):

| Caso | Resultado medido | Conclusão |
|------|-----------------|-----------|
| Alvo absoluto (`/etc/passwd`) | `symlink->/etc/passwd\|not-supported` | CORRETO — não enganoso |
| Alvo ausente (`nonexistent.sh`) | `symlink->nonexistent.sh\|not-found` | CORRETO — não enganoso |
| Cadeia (link→middle→real) | `symlink->middle.sh\|sha256:d7b138…` estável | RESÍDUO NOVO R3 — ver abaixo |
| Circular (link→link) | `symlink->link.sh\|sha256:75871769…` | RESÍDUO NOVO R4 — ver abaixo |

#### F3 — `validateRef` aceita SHA estrangeiro — FECHADO

Confirmado por leitura de código (`^{commit}` presente, linha 96 de `internal/commands/audit_surface.go`)
e por gate FN-F3 (3 cenários, todas as 3 runtimes):

```
$ trackfw audit-surface 0000000000000000000000000000000000000042
Error: audit-surface: ref "000000…42" does not resolve: fatal: Needed a single revision
```

---

### Resíduos novos declarados (descobertos na reverificação)

#### R1 — `env VAR=x script.sh` → `unresolvable` estável (MEDIUM)

`env` não está no mapa `interpreters` de `normalizeCommand`. O token `env` é tratado como "não é
interpretador" → `candidate = tokens[0] = "env"` → sem extensão de script → retorna `""` →
`unresolvable`. O relatório mostra `unresolvable` nas duas refs quando `scripts/hook.sh` muda.

Não é uma mentira (o marcador é `unresolvable`, não um sha256 falso), mas é um gap: a Variante A
é indetectável para comandos prefixados com `env`. Severidade MEDIUM — prática comum em wiring
files de projetos que precisam injetar env vars.

**Mitigation:** adicionar `env` ao tratamento especial — consumir os tokens `VAR=valor` até
encontrar o primeiro token sem `=` e usá-lo como candidato de script.

#### R2 — `scripts/sub dir/hook.sh` (espaço no path) → `unresolvable` estável (MEDIUM)

`normalizeCommand`: cmd tem espaço → entra no ramo de "interpreter prefix ou path com argumentos".
`tokens[0] = "scripts/sub"` não está no mapa de interpretadores → `candidate = "scripts/sub"` → sem
extensão → retorna `""` → `unresolvable`. Paths com espaço no nome são incomuns mas válidos em
wiring files (e.g., projetos que usam nomes de diretórios com espaço).

Não é uma mentira (marcador `unresolvable`), mas a Variante A é indetectável para esses paths.
Severidade MEDIUM.

**Mitigation:** antes de tentar interpretar espaços como separador, tentar o path inteiro
`git ls-tree ref --name-only -- "scripts/sub dir/hook.sh"`. Se existe, usá-lo diretamente.

#### R3 — Symlink em cadeia (link→middle→real) → digest estável para mudança no final (HIGH)

`getSymlinkTarget` detecta que `link.sh` é modo `120000` → lê o target `middle.sh` (relativo) →
resolve para `scripts/middle.sh` → chama `gitShow(ref, "scripts/middle.sh")`. `gitShow` faz
`git show ref:scripts/middle.sh` que retorna o blob de `middle.sh` — que é **também um symlink**,
cujo blob é a string `"real.sh"`. O código recebe `"real.sh"`, calcula `sha256("real.sh")` e reporta
esse hash.

**Resultado medido:**
```
REF1: symlink->middle.sh|sha256:d7b138374e701b5c7eb55f6ffbcffc56ee27502d936409f39f84f1789843fc7d
REF2: symlink->middle.sh|sha256:d7b138374e701b5c7eb55f6ffbcffc56ee27502d936409f39f84f1789843fc7d
```

`real.sh` mudou entre REF1 e REF2. Digest **idêntico**. Este é o mesmo modo de falha do F2
original, um nível mais fundo. Severidade HIGH — o `symlink->` está no marcador, mas o sha256 parece
real e é imutável.

**Mitigation:** após seguir um symlink e chamar `gitShow`, verificar se o resultado retornado
também é um blob de modo `120000` (novo `getSymlinkTarget` recursivo, com profundidade máxima).
Alternativa: `git cat-file -t ref:resolved` antes de hashear — se `"blob"` e o blob tem modo
`120000`, reportar `symlink->target|chain-not-supported`.

**Bloqueia o repositório trackfw?** Não. Hooks do repo não usam symlinks em cadeia.

#### R4 — Symlink circular (link→link) → sha256 do nome do target, não de conteúdo (MEDIUM)

`getSymlinkTarget` retorna `"link.sh"` (target string). Resolvido para `"scripts/link.sh"`.
`gitShow(ref, "scripts/link.sh")` retorna `"link.sh"` novamente (é o blob do symlink).
sha256(`"link.sh"`) é reportado como digest.

**Resultado medido:**
```
symlink->link.sh|sha256:75871769f67f85c6a0315b2ff20b89fdd320940108ac4e7a79bf5e78612182ec
```

O prefixo `symlink->` está presente (o leitor é alertado de que é um symlink). O sha256 reportado,
porém, é o hash do **nome do target**, não de conteúdo executável. Prático: um symlink circular não
executaria em runtime (ELOOP), então o vetor de ataque real é baixo. Severidade MEDIUM.

**Mitigation:** na resolução de symlink, detectar loop (path de destino igual ao path de origem)
e reportar `symlink->link.sh|circular-not-supported`.

---

### Pergunta 3 — `package.json` por descoberta: falso-positivo? — NEGATIVO

Medido nos três casos preocupantes:

| Caso | Resultado | Conclusão |
|------|-----------|-----------|
| `package.json` na raiz com `postinstall` | `lifecycle [present] package.json postinstall…` | Correto ✅ |
| `node_modules/evil/package.json` com `postinstall` | não aparece no relatório | Correto ✅ |
| `fixtures/package.json` com `postinstall` | não aparece no relatório | Correto ✅ |

A discovery tenta apenas dois candidatos (`package.json` e `npm/package.json`). `node_modules` e
subdiretórios arbitrários não são varridos. Risco de falso-positivo: NULO para os padrões testados.

---

### Pergunta 4 — Regressão nos 17 cenários originais — NEGATIVO

```
check-audit-surface: all 17 scenarios passed (FN-1..5, FP-1..2, FN-F1a/b/c, FN-F2, FN-F3)
```

---

### Resumo de resíduos aceitos para esta entrega

| ID | Severidade | Superfície | FN confirmado? | Bloqueia trackfw? |
|----|-----------|-----------|----------------|------------------|
| R1 | MEDIUM | `env VAR=x script.sh` → unresolvable | Sim (mas marcado) | Não |
| R2 | MEDIUM | path com espaço → unresolvable | Sim (mas marcado) | Não |
| R3 | HIGH | cadeia de symlinks → sha256 estável | Sim | Não |
| R4 | MEDIUM | symlink circular → sha256 do nome | Parcialmente | Não |

R1/R2/R3/R4 devem ser rastreados na REQ existente de F1/F2/F3 ou numa REQ própria antes de
promover `audit-surface` como ferramenta genérica.

---

## Referências

- Modelo de ameaça: `docs/seguranca/2026-08-26-modelo-de-ameaca-da-superficie-executavel-de-checkout.md`
- Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-26-comando-que-audita-a-superficie-executavel-de-um-checkout-de-pr.md`
- Implementação core: `internal/auditsurface/auditsurface.go`
- Comando cobra: `internal/commands/audit_surface.go`
- Gate: `scripts/check-audit-surface.sh`
