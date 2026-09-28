# Wave 0 — Threat Model: `trackfw init` reexecutado destrói a configuração do consumidor

> Hades (Security) · 2026-09-28 · Branch: `fix/init-reexecutado-destroi-config-do-consumidor`
> REQ: `docs/req/REQ-2026-09-28-trackfw-init-reexecutado-destroi-a-configuracao-do-consumidor-e-o-produto-passa-a-reprovar-um-repositorio-conforme.md`
> ADR: `docs/adr/ADR-2026-09-28-trackfw-init-reexecutado-preserva-a-configuracao-autorada-pelo-consumidor.md`

---

## 1. Enumeration completeness

O grep com âncora de início de linha retornou **22 sítios reais**:

```bash
grep -nE '^[[:space:]]*(if err := )?os\.(WriteFile|OpenFile)' internal/generators/scaffold.go
```

Saída: 22 linhas. (O grep ingênuo por substring conta 25 — 3 são comentários dentro de strings
literais de template e de godoc. A âncora acima exclui ambos os falsos positivos.)

O arquivo `commands/discover.go` tem um segundo writer de `trackfw.yaml` (linha 147), gerado por
`discover.GenerateYAML()`. Esse caminho JÁ ESTÁ GUARDADO por existência:

```go
// commands/discover.go:138–141
if _, statErr := os.Stat(yamlPath); statErr == nil {
    fmt.Fprintln(out, "\n⚠ trackfw.yaml already exists — remove it first if you want to regenerate")
    return nil
}
```

`discover --init` se recusa a sobrescrever. Não é um segundo sítio de defeito.

**A lista de 22 sítios está fechada para scaffold.go. O discover path está guardado.**

---

## 2. Tabela dos 22 sítios

| # | Linha | Função | Arquivo-alvo | Classe | Razão |
|---|---|---|---|---|---|
| 1 | 266 | `GenerateGlobalSkill` | `~/.claude/skills/trackfw/SKILL.md` | **(b)** | Artefato gerado pelo produto; como a habilidade chega ao agente — sobrescrever é o mecanismo de atualização. |
| 2 | 789 | `generateClaudeCommandsInner` | `.claude/commands/trackfw/<files>` | **(b)** | Slash commands gerados pelo produto. Tem guarda `os.Stat` + `!force` — não sobrescreve se existente; classificado (b) porque o conteúdo é do produto. |
| 3 | 864 | `writeTrackfwConfig` | `trackfw.yaml` | **(a)** | Configuração autorada pelo consumidor. Overwrite incondicional sem leitura prévia. **O defeito principal desta REQ.** |
| 4 | 893 | `generateValidateScript` | `scripts/trackfw-validate.sh` | **(b)** | Script gerado pelo produto; sobrescrever é como fixes chegam (ex: CRLF do #353). |
| 5 | 1022 | `generateAttentionScripts` | `scripts/trackfw-attention-signal.sh` | **(b)** | Script gerado pelo produto. |
| 6 | 1041 | `generateAttentionScripts` | `scripts/trackfw-attention-cleanup.sh` | **(b)** | Script gerado pelo produto. |
| 7 | 1088 | `GenerateCredentialGuardScript` | `scripts/trackfw-credential-guard.sh` | **(b)** | Script de guarda gerado pelo produto; sobrescrever é atualização de segurança. |
| 8 | 1141 | `GenerateGlobalCredentialGuardScript` | `~/.trackfw/scripts/trackfw-credential-guard.sh` | **(b)** | Variante global do guard; mesmo racional do #7. |
| 9 | 1439 | `GenerateGitBranchGuardScript` | `scripts/trackfw-git-branch-guard.sh` | **(b)** | Script de guarda gerado pelo produto. |
| 10 | 1492 | `GenerateGlobalGitBranchGuardScript` | `~/.trackfw/scripts/trackfw-git-branch-guard.sh` | **(b)** | Variante global do guard; mesmo racional do #9. |
| 11 | 2413 | `generateGitHubActionsWorkflow` | `.github/workflows/trackfw-gate.yml` | **(b)** | Workflow CI gerado pelo produto. |
| 12 | 2430 | `generateGitLabCIWorkflow` | `.gitlab-ci-trackfw.yml` | **(b)** | Workflow CI gerado pelo produto. |
| 13 | 2483 | `generateCommitMsgHook` (ramo husky) | `.husky/commit-msg` | **(b)** | Script de hook gerado pelo produto. |
| 14 | 2498 | `generateCommitMsgHook` (ramo lefthook) | `lefthook.yml` | **(c)** | Lê o arquivo existente, verifica presença de `commit-msg:`, acrescenta apenas se ausente. **Ramo de merge COMPLETO.** |
| 15 | 2513 | `generateCommitMsgHook` (ramo lefthook) | `.lefthook/commit-msg/trackfw-req-check.sh` | **(b)** | Script de hook gerado pelo produto. |
| 16 | 2551 | `generateHuskyHook` | `.husky/pre-commit` | **(b)** | Script de hook gerado pelo produto. |
| 17 | 2592 | `generateVaultIndex` | `vault/notes/index.md` | **(c)** | Semeado pelo produto mas torna-se autoria do consumidor (notas). Tem guarda `os.Stat` — skip-if-exists. **Já idempotente; ramo correto.** |
| 18 | 2659 | `generateGitAttributes` (ramo create) | `.gitattributes` | **(c)** | Cria o arquivo quando ausente. Parte de padrão 3 ramos: create/append/no-op. Junto com #19, **COMPLETO**. |
| 19 | 2679 | `generateGitAttributes` (ramo append) | `.gitattributes` | **(c)** | Lê, verifica, acrescenta bloco se regra ausente. Zero diff no conteúdo pré-existente. **Ramo de merge COMPLETO.** |
| 20 | 2764 | `generateGitIgnore` (ramo create) | `.gitignore` | **(c)** | Cria quando ausente. Padrão idêntico ao .gitattributes. Junto com #21, **COMPLETO.** |
| 21 | 2784 | `generateGitIgnore` (ramo append) | `.gitignore` | **(c)** | Lê, verifica, acrescenta bloco. **Ramo de merge COMPLETO.** |
| 22 | 2806 | `generateLefthookHook` | `lefthook.yml` | **(a)** | Overwrite incondicional do arquivo inteiro. Aplica a régua: conteúdo destruído é configuração autorada pelo consumidor (outros hooks do projeto). **Segundo sítio (a). Ver §2.1.** |

**Contagem:** (a) = 2 · (b) = 14 · (c) = 6 · Total = 22. ✓

### 2.1 Segundo sítio (a): `generateLefthookHook` linha 2806

A chamada ocorre em `generateGitHooks(cfg)` (linha 165 do orquestrador de `init`), que precede
`generateCommitMsgHook(cfg)` (linha 169). A sequência sob `cfg.Hooks == "lefthook"` é:

1. **L165** → `generateGitHooks` → `generateLefthookHook` → `os.WriteFile("lefthook.yml", ...)` — sobrescreve o arquivo **inteiro** com 4 linhas de pre-commit.
2. **L169** → `generateCommitMsgHook` → lê o `lefthook.yml` que acabou de ser sobrescrito, não acha `commit-msg:`, acrescenta.

Na primeira execução: correto.
Na reexecução com consumidor que adicionou hooks próprios (lint, pre-push, etc.): o conteúdo do consumidor é destruído no passo 1 e não é recuperado no passo 2. **A auto-contradição é medida no código: o produto desfaz no passo anterior o que recostrói no seguinte.**

O comentário no próprio scaffold.go (linhas 133-136) já documentava que `writeTrackfwConfig`
"sobrescreve trackfw.yaml sem ler o valor anterior" — o mesmo padrão existe em `generateLefthookHook`
e pela mesma razão não foi corrigido: ninguém aplicou a mesma régua.

**Impacto da descoberta:** a AC "Todo sítio (a) corrigido por ponto único" não é satisfeita por
uma correção em `writeTrackfwConfig` sozinha. Um ML adicional para `generateLefthookHook` deve
ser acrescentado ao roadmap antes de Wave 1 iniciar.

**Severidade comparativa:** menor do que o sítio da linha 864. `lefthook.yml` não governa o
resultado de `trackfw validate`; a perda não corrompe medições subsequentes. É um defeito real
de preservação de conteúdo do usuário, não um defeito de integridade de governança.

---

## 3. População (a) nomeada

São exatamente **dois sítios**:

| Sítio | Linha | Defeito |
|---|---|---|
| `writeTrackfwConfig` | 864 | Overwrite de `trackfw.yaml`; causa o dano de governança documentado na REQ |
| `generateLefthookHook` | 2806 | Overwrite de `lefthook.yml`; destrói hooks do consumidor mas não afeta validate |

O AC da REQ e da ADR foi escrito presumindo que (a) = 1. A enumeração mostra (a) = 2.

**Consequência direta:** a ADR diz "ponto único é alcançável" — isso continua verdade para a
correção de cada sítio, mas o ponto único por defeito são **dois**, e o roadmap precisa de um
ML para cada.

---

## 4. Sítios (c) verificados

Seis sítios (c) no total. Status do ramo de merge em cada:

| Linha | Arquivo-alvo | Ramo de merge | Status |
|---|---|---|---|
| 2498 | `lefthook.yml` (bloco commit-msg) | Lê existing, `strings.Contains(string(existing), "commit-msg:")`, acrescenta apenas se ausente | **COMPLETO** |
| 2592 | `vault/notes/index.md` | `os.Stat` skip-if-exists (linhas 2570-2573) | **COMPLETO** (já idempotente) |
| 2659 | `.gitattributes` (create) | Ramo de `os.IsNotExist` — só chega aqui quando o arquivo não existe | **COMPLETO** |
| 2679 | `.gitattributes` (append) | Lê, chama `hasGitAttributesRule`, acrescenta bloco apenas se regra ausente | **COMPLETO** |
| 2764 | `.gitignore` (create) | Ramo de `os.IsNotExist` — só chega aqui quando o arquivo não existe | **COMPLETO** |
| 2784 | `.gitignore` (append) | Lê, chama `hasGitIgnoreRule`, acrescenta bloco apenas se regra ausente | **COMPLETO** |

**Todos os seis sítios (c) têm ramo de merge completo.** Nenhuma lacuna.

O código relevante dos ramos .gitattributes e .gitignore:

```go
// generateGitAttributes — linhas 2666–2682
existing, err := os.ReadFile(path)
if err != nil {
    if !os.IsNotExist(err) { return fmt.Errorf(...) }
    // ramo create (linha 2659): arquivo ausente
    if err := os.WriteFile(path, []byte(gitAttributesBlock), 0644); err != nil { ... }
    return nil
}
if hasGitAttributesRule(string(existing)) {
    return nil  // ramo no-op: regra já presente
}
out := string(existing)
if out != "" && !strings.HasSuffix(out, "\n") { out += "\n" }
out += gitAttributesBlock
// ramo append (linha 2679): acrescenta sem tocar conteúdo pré-existente
if err := os.WriteFile(path, []byte(out), 0644); err != nil { ... }
```

O guarda `!strings.HasSuffix(out, "\n")` nas linhas 2674-2676 e 2779-2781 previne que o bloco
acrescentado grude na última linha pré-existente. **Este guarda é precedente obrigatório** para
qualquer ramo de append — incluindo o merge textual de `writeTrackfwConfig` (ML-1A).

---

## 5. Refutação ou confirmação da premissa da ADR — merge textual basta?

**CONFIRMADO.** O merge textual por chave ausente é suficiente para todos os constructs YAML
que `trackfw.yaml` usa e para as customizações razoáveis de um consumidor.

### 5.1 Corpus examinado

O `trackfw.yaml` deste repositório (20 linhas, commit em `fix/init-reexecutado-destroi-config-do-consumidor`):

```yaml
# trackfw configuration — gerado por trackfw discover
# governance_mode: lenient permite validação não-bloqueante durante onboarding

governance_mode: lenient
lenient_until: "2027-12-31"

adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: flat
hooks: none
ci: github-actions
forge: github

# Versão do modelo por tier (ADR-2026-08-21). ...
# ...
```

O template de `writeTrackfwConfig` (linhas 817-840 + fragmentos condicionais) inclui: escalares
simples, bloco `rules:` aninhado em 2 linhas, e bloco `agent_conventions:` com scalar `|`
(indentado por `strings.ReplaceAll(s, "\n", "\n  ")`).

### 5.2 Spec de implementação do merge textual

Para que o merge seja correto e seguro, a detecção de chave existente deve respeitar estas
quatro propriedades. Cada uma tem evidência no próprio código do produto:

**P1 — Ancoragem em coluna 0.** A detecção deve casar apenas linhas que começam com `key:` sem
espaço à esquerda. Isso é suficiente e necessário porque:
- Chaves aninhadas têm indentação obrigatória (`  branch_has_wip_roadmap: error` começa com 2 espaços).
- O template gera `agent_conventions` com corpo indentado explicitamente (linha 847:
  `strings.ReplaceAll(cfg.AgentConventions, "\n", "\n  ")`), de modo que `rules: ...` no corpo
  **nunca** aparece em coluna 0. A ancoragem é suficiente **por construção do template**.

**P2 — Dois pontos requeridos no padrão.** `strings.HasPrefix(line, "wip_limit:")` não coincide com
`wip_by_squad:` — os prefixos são distintos. Mas a verificação DEVE incluir `:` para fechar a
possibilidade de prefixo acidental em chaves futuras. Padrão correto: `^key:` ou
`strings.HasPrefix(line, key+":"`) (não `strings.HasPrefix(line, key)`).

**P3 — Linhas de comentário excluídas.** Uma chave que aparece apenas em comentário
(`# wip_limit: 3`) está **ausente** do YAML e DEVE ser acrescentada. Se o predicado usar
`strings.Contains` sem excluir comentários, a chave nunca seria adicionada. A semântica correta:
ignorar linhas que começam com `#` (após TrimSpace) antes de verificar presença.

Comportamento decidido: chave comentada = chave ausente = acrescentar.

**P4 — Guarda de newline final antes de append.** `generateGitAttributes` (linhas 2674-2676) e
`generateGitIgnore` (linhas 2779-2781) já carregam este guarda:
```go
if out != "" && !strings.HasSuffix(out, "\n") {
    out += "\n"
}
```
O mesmo guarda é **obrigatório** em `writeTrackfwConfig`, citando esses precedentes como razão.
Sem ele, a primeira chave nova grudaria na última linha pré-existente, corrompendo silenciosamente.

### 5.3 Casos que NÃO são problema

| Construct YAML | Por quê não é problema |
|---|---|
| Escalares entre aspas (`lenient_until: "2027-12-31"`) | A detecção olha a chave, não o valor; aspas no valor são transparentes |
| Listas em bloco (`adr_dirs:\n  - docs/adr`) | Chave em coluna 0; o corpo indentado não é confundido com outra chave |
| Blocos aninhados (`rules:\n  branch_has_wip_roadmap: error`) | Chave em coluna 0; conteúdo indentado ignorado por P1 |
| Scalar literal (`agent_conventions: \|`) | Corpo indentado por template (linha 847); P1 basta |
| Chaves com prefixo compartilhado (`roadmap_dir` / `roadmap_namespacing`) | P2 (dois-pontos) as distingue: `roadmap_dir:` ≠ `roadmap_namespacing:` |
| Valor com dois-pontos (`lenient_until: "2027-12-31"`) | A detecção para no primeiro `:` ao verificar a chave, não o valor; sem ambiguidade para chaves top-level |

### 5.4 Casos fora de escopo declarado (residual)

| Construct | Por quê fora de escopo |
|---|---|
| Âncoras e aliases YAML (`&base`, `*base`) | O produto **nunca** os emite. `trackfw.yaml` é escrito por template de texto; âncoras nunca são geradas. |
| Chave de merge YAML (`<<: *base`) | Mesmo racional — produto não emite. |
| Documentos múltiplos (`---`) | Produto não emite. |

Esses casos são **residual declarado**: a decisão de não cobrir é correta e intencional, não
uma lacuna.

### 5.5 Veredicto

O merge textual por chave ausente cobre todos os constructs que o produto gera e as
customizações razoáveis do consumidor. A implementação (ML-1A) deve satisfazer P1-P4
acima. Não há construct no corpus que exija round-trip estrutural (yaml.Node ou map[string]any).

**A ADR está confirmada.** A escolha de merge textual é a correta.

---

## 6. Evidência de que este arquivo não modificou o produto

```
git diff trackfw.yaml
(saída vazia — confirmado antes de escrever este parecer)
```

---

## 7. Ações requeridas antes de Wave 1 iniciar

1. **Acrescentar ML ao roadmap para linha 2806** (`generateLefthookHook`): implementar ramo
   de merge similar ao da linha 2498 — lê `lefthook.yml`, verifica presença de
   `pre-commit: commands: trackfw-validate:`, acrescenta apenas se ausente. Severidade menor
   que o sítio principal (não afeta validate), mas é **(a)** pela régua e deve entrar nesta REQ.

2. **ML-1B deve listar sítios (b) isentos em dois grupos:**
   - "Trunca por design (sem guarda)": linhas 266, 893, 1022, 1041, 1088, 1141, 1439, 1492, 2413, 2430, 2483, 2513, 2551.
   - "Já guardado por skip-if-exists": linhas 789, 2592.
   O gate de dois lados deve excluir ambos os grupos, mas com razão diferente. Se o gate for
   escrito como "WriteFile sem ReadFile precedente = flag", as linhas 789 e 2592 são falsos
   positivos se o gate não reconhecer o padrão `os.Stat` + skip.

3. A AC da REQ "Todo sítio (a) corrigido por ponto único" deve ser lida como dois pontos
   únicos após esta enumeração — um para linha 864, um para linha 2806.
