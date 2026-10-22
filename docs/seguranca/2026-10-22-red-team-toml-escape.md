# Red Team — ML-4E: Wave 4 (TOML escape do Codex)

**ML-4E | REQ-2026-09-26 | Agente: hades-tf | 2026-10-22**

Worktree: `/private/tmp/trackfw-toml-escape` (branch `fix/harness-arquiteto-escape-toml-codex`).
Cópia de sonda: scratchpad `rt4e/` (rsync --exclude=.git). HOME isolado em todos os runs.

---

## Veredito

**REPROVADO** — dois defeitos identificados. Um bloqueante, um secundário.

### Defeito 1 — BLOQUEANTE (severidade média): U+FFFE e U+FFFF chegam ao frontmatter YAML e causam falha no target claude

`containsControlCharIdent` (`identity.go:170`) e `containsControlChar` (`render.go:753`) não cobrem
U+FFFE nem U+FFFF. Esses não-caracteres Unicode passam pelo `identity.Load`, chegam ao campo
`description` do frontmatter YAML do destino `claude` (Rota B), e o yaml.v3 rejeita o conteúdo
com `"yaml: control characters are not allowed"`. O arquivo do agente não é escrito.

**Impacto medido:** Load passa. Render retorna nil (sem erro). O agente-arquivo para o destino
`claude` não é criado (yaml.v3 rejeita durante a escrita). O destino `codex` (TOML) não é
afetado — `tomlBasicString` já escapa esses valores como `￾`/`￿` (IsPrint=false).

**Nota sobre "double-quoted também falha":** adicionando U+FFFE a `containsControlChar` sem mais
nada, `needsYAMLQuoting` retornaria true e `yamlEscapeDoubleQuoted` seria chamado. Mas
`yamlEscapeDoubleQuoted` cai no `WriteRune` (rune > 0x9F) e emite os bytes UTF-8 crus. O yaml.v3
TAMBÉM rejeita a forma double-quoted com raw U+FFFE: `yaml: control characters are not allowed`.
A especificação YAML 1.2 proíbe U+FFFE e U+FFFF no stream inteiro, mesmo em escalares
double-quoted com escape. **Não existe representação YAML válida para esses code points.**
Portanto, o fix correto é REJEITAR, não escapar. `yamlEscapeDoubleQuoted` não precisa ser alterado.

**Fix requerido (dois sítios, mesma causa):**

```
identity.go:  containsControlCharIdent — acrescentar r == 0xFFFE || r == 0xFFFF
render.go:    containsControlChar       — acrescentar r == 0xFFFE || r == 0xFFFF
```

O sítio primário é `containsControlCharIdent` — rejeição em Load impede que os valores alcancem
qualquer renderizador. O sítio `render.go` é defesa-em-profundidade: garante que se uma descrição
com U+FFFE/U+FFFF estiver no markdown do agente (sem passar por Load), o check em
`rewriteFrontmatterModelLine` também rejeita. Os dois devem ser consertados na mesma entrega.

### Defeito 2 — SECUNDÁRIO (severidade baixa, pré-existente): bare comma em display_name causa rejeição yaml.v3

A enumeração completa (`TestRT3_MeasureYAMLSafeScalar_AllNonchars`, 1.1 M runes) encontrou
três runes rejeitados por yaml.v3: U+002C (`,`), U+FFFE, U+FFFF.

Para a vírgula: `needsYAMLQuoting(",")` = false. `yamlSafeScalar(",")` = `","`. YAML resultante:
`description: ,`. yaml.v3 interpreta como erro de fluxo.

**Este defeito é pré-existente à Wave 4** (não foi introduzido por nenhum ML desta REQ). Um
`display_name` de valor `,` sozinho é patologicamente incomum. Severidade baixa. Registrado como
achado de enumeração; não bloqueia o merge da Wave 4, mas deve ser aberto como defeito separado.

---

## 1. Vetores T-1 a T-4 do threat model — estão fechados?

### T-1 — `strconv.Quote` substituído por `tomlBasicString` (Codex)

**FECHADO.**

As 4 chamadas de `strconv.Quote` no ramo `custom-agent-toml` foram substituídas por
`tomlBasicString`. Validação com Python 3.12 `tomllib` (oráculo TOML 1.0):

| Input | Saída | Python 3.12 |
|---|---|---|
| BEL U+0007 | `\u0007` | ACEITO — era `\a` |
| VT U+000B | `\u000b` | ACEITO — era `\v` |
| DEL U+007F | `\u007f` | ACEITO — era `\x7f` |
| NEL U+0085 | `\u0085` | ACEITO |
| C1 U+009F | `\u009f` | ACEITO |
| U+2028 | ` ` | ACEITO |
| U+FFFD | raw UTF-8 (IsPrint=true) | ACEITO |
| U+FFFE | `￾` (IsPrint=false) | ACEITO — TOML encoder correto |
| U+FFFF | `￿` (IsPrint=false) | ACEITO — TOML encoder correto |
| U+10FFFF | `\U0010ffff` | ACEITO |
| NUL | `\u0000` | ACEITO |
| Surrogate bytes (0xED 0xA0 0x80) | error | N/A — rejeitado antes |
| JSON `\ud800` | Load passa com U+FFFD | ACEITO |

Todos os casos C0, DEL, C1, U+2028/U+2029 produzem TOML 1.0 válido e round-trip correto.

**Codex 0.163.0:** Teste direto de carregamento de agente não realizado (requer auth). Wave-0
confirmou TOML 1.1 (crates `toml-0.9.11+spec-1.1.0`) via metadados do binário. Python 3.12
(TOML 1.0, mais restritivo) aceita toda a saída do encoder — bound conservador.

### T-2 — DEL (U+007F) em `display_name` via identidade

**FECHADO por ML-4C.** `containsControlCharIdent` cobre `r == 0x7F`. Load rejeita.

### T-3 — BEL (U+0007) em slug via identidade

**FECHADO por ML-4C.** `identity.Load` valida slugs contra `slugPattern`. BEL não casa.

C1 em display_name (ex.: NEL U+0085) também recusado.

**Nota sobre o alvo FP `a_b` do Wave-0:** O Wave-0 citou `a_b` como alvo FP ("deve PASSAR").
Mas o `slugPattern` (`^[a-z0-9]+(-[a-z0-9]+)*$`) nunca permitiu underscores. ML-4C apenas
moveu a validação para acontecer também em `Load`. O exemplo no Wave-0 era inapropriado. O
alvo FP correto (`my-agent`) continua passando. Presets Ártemis, Métis e Dédalo passam.

**Blast radius documentado:** Um slug inválido em `identity.json` faz `Load` falhar para
TODOS os agentes do arquivo — comportamento correto pela política "callers must reject".

### T-4 — HTML injetado em TOML via `ApplyThirdPartyReferences`

**FECHADO por ML-4D.** `BuildPlans` guarda com `!IsStructuredAgentRepresentation`. Para
`codex` (`custom-agent-toml`), o bloco HTML é omitido silenciosamente. Medido.

---

## 2. Impacto medido do Defeito 1

**`TestRT3_FFFE_FFFF_Impact_CLI` — FAIL (2 subtests), medido na cópia rt4e:**

Para U+FFFE em `display_name="Zeus￾TF"`:

```
identity.Load PASSED — display_name="Zeus￾TF"
yaml.v3 REJECTED frontmatter (DEFECT CONFIRMED): yaml: control characters are not allowed
content[:300]:
---
name: zeus-tf
description: Zeus<raw-U+FFFE>TF — Senior backend specialist for APIs...
model: sonnet
memory: project
tools: Read, Edit, Write, Bash, Grep, Glob, AskUserQuestion
---
```

Para U+FFFF: mesmo comportamento.

O arquivo do agente não seria criado. O CLI informa o erro ao usuário. Falha ruidosa, não
silent write. Não há corrupção silenciosa.

**`TestRT3_AfterFix_Simulation`** — confirmou que mesmo com double-quoting, yaml.v3 rejeita:

```
yamlEscapeDoubleQuoted(U+FFFE) = raw-UTF-8-U+FFFE (WriteRune path)
yaml.v3 REJECTED quoted U+FFFE: yaml: control characters are not allowed
yaml.v3 REJECTED quoted U+FFFF: yaml: control characters are not allowed
```

Conclusão: não existe representação YAML válida. Fix correto é rejeição em Load.

---

## 3. Enumeração completa: yamlSafeScalar x yaml.v3

`TestRT3_MeasureYAMLSafeScalar_AllNonchars` — enumerou 1.1 M runes (skipping C0/DEL/C1/LS/PS
já cobertos, surrogates). Runes rejeitados por yaml.v3:

| Rune | Valor | Status |
|---|---|---|
| U+002C | `,` | Pré-existente — bare comma não coberta por `needsYAMLQuoting` |
| U+FFFE | non-character | **Defeito 1** desta REQ |
| U+FFFF | non-character | **Defeito 1** desta REQ |

Nenhum outro rune falhou além desses três.

---

## 4. TOML: `tomlBasicString` produz TOML 1.0 inválido em alguma entrada?

**NÃO encontrado.** Ver tabela em §1 (T-1). U+FFFE e U+FFFF são corretamente escapados.
`TestRT3_TOML_FFFE_FFFF` — PASS.

---

## 5. Referências de terceiro: algum caminho ainda anexa bloco a artefato estruturado?

**NÃO encontrado.**

`ApplyThirdPartyReferences` tem único caller: `BuildPlans` em plan.go:112.

**`--apply-to` com targets misturados (ex.: `claude,codex`):** O loop de pré-validação
(L318-397 de integrations_thirdparty.go) itera TODOS os `resolvedTargets` para cada `agentID`
antes de qualquer escrita. Skill file (L488), provenance (L510-L520) e registry (L530) são
escritos DEPOIS do loop — nenhuma escrita acontece na recusa. Coberto pelo teste
`TestThirdPartyInstall_ApplyToRejectsStructuredRepresentationBeforeWrite`.

Nota: teste via CLI para targets misturados (`claude,codex` simultâneos) não foi executado.
Code path verificado por leitura. Caminho é determinístico pelo mesmo loop.

**Estado parcial `UpsertThirdPartyReference` + `manager.Update`:** Resíduo aceito pelo
arquiteto na auditoria de ML-4D. Estado transitório resolvido pelo próximo `agents update`.
Não é defeito de segurança.

---

## 6. Suíte de testes — contagem verificada

Pacotes testados com `HOME=/tmp/isolated-home-rt4e` na cópia `rt4e/`:

| Pacote | Resultado |
|---|---|
| `internal/integrations` | 115 PASS, 0 FAIL (pré-RT3) |
| `internal/identity` | todos PASS |
| `internal/commands` | todos PASS |

---

## 7. Exigências para o prometeu-tf

### 7.1 Fix — Defeito 1 (dois sítios, mesma causa)

**Sítio A — identity.go, `containsControlCharIdent` (linha 170-177):**

Condição resultante:
```go
if r < 0x20 || r == 0x7F || (r >= 0x80 && r <= 0x9F) || r == 0xFFFE || r == 0xFFFF || r == ' ' || r == ' ' {
```

**Sítio B — render.go, `containsControlChar` (linha 753-760):**
```go
if r < 0x20 || r == 0x7F || (r >= 0x80 && r <= 0x9F) || r == 0xFFFE || r == 0xFFFF || r == ' ' || r == ' ' {
```

**`yamlEscapeDoubleQuoted` NÃO precisa de alteração.** U+FFFE e U+FFFF não possuem
representação YAML válida. O fix correto é rejeição em Load; a função de escape nunca deve
receber esses valores.

### 7.2 Testes exigidos (mordida dupla obrigatória)

- `TestIdentity_ContainsControlCharIdent_FFFE_FFFF`:
  - Load com `display_name="Zeus￾TF"` → deve REJEITAR. Sem fix: deve PASSAR (mordida).
  - Idem para U+FFFF.
  - Verificar que presets (Ártemis, Métis, Dédalo) continuam passando após o fix.

- `TestRender_ContainsControlChar_FFFE_FFFF` (defesa-em-profundidade):
  - Construir identity.Config diretamente (bypass Load), chamar Render para `claude` com
    display_name contendo U+FFFE, parsear frontmatter com yaml.v3 → deve retornar erro.
  - Não usar apenas `containsControlChar("￾") == true` como único teste — esse proxy
    passaria mesmo com `yamlEscapeDoubleQuoted` ainda emitindo raw bytes.

### 7.3 Defeito 2 (vírgula) — não bloqueia Wave 4

Pré-existente. Abrir como nova REQ com:
- Causa: `needsYAMLQuoting` não cobre U+002C standalone.
- Fix: adicionar `,` ao switch de runes que forçam quoting.

---

## 8. Notas para o vault (texto a ser gravado pelo arquiteto)

### Nota: U+FFFE e U+FFFF não têm representação YAML válida — rejeitar, não escapar

**Causa raiz:** `containsControlCharIdent` e `containsControlChar` não cobriam U+FFFE/U+FFFF.
Esses dois não-caracteres Unicode são proibidos pela especificação YAML 1.2 no stream inteiro —
nem `￾` como escape sequence em double-quoted scalar é aceito (yaml.v3 aplica c-printable
range antes de usar o escape).

**Medição:** `TestRT3_AfterFix_Simulation` confirmou que double-quoting não resolve.
`TestRT3_MeasureYAMLSafeScalar_AllNonchars` (1.1 M runes, 2.08s) confirmou que são as únicas
duas entradas não-C0/DEL/C1/LS/PS rejeitadas (além de bare comma, causa distinta).

**Regra:** Para qualquer novo campo de identidade que chegue ao YAML, adicionar U+FFFE e U+FFFF
ao conjunto de caracteres a rejeitar. Nunca tentar escapá-los para YAML.

**Achado colateral:** bare comma (U+002C) como único valor de display_name também causa
rejeição yaml.v3 — pré-existente, mecanismo distinto (`needsYAMLQuoting` não cobre `,`).

### Nota metodológica: enumerar o espaço de entrada antes de declarar fechado

O loop de 1.1 M runes rodou em 2.08s e encontrou o defeito do bare comma que a análise
pontual havia perdido. Para qualquer função de serialização de segurança, enumerar o espaço
completo de entrada antes de declarar o conjunto de defeitos fechado.

---

## Apêndice — Working context (incluso no parecer por restrição de escopo desta sessão)

**INÍCIO (2026-10-22):** hades-tf, ML-4E Red Team da Wave 4 (REQ-2026-09-26). Leitura do threat
model, roadmap e código. Cópia de sonda em scratchpad rt4e/. Probes em Go + validação Python 3.12.

**FIM (2026-10-22):** hades-tf, ML-4E Red Team da Wave 4. Artefato:
`docs/seguranca/2026-10-22-red-team-toml-escape.md`. Veredito: REPROVADO.

Defeito primário (BLOQUEANTE): U+FFFE e U+FFFF passam `containsControlCharIdent` e
`containsControlChar`, chegam ao frontmatter YAML como raw bytes, yaml.v3 rejeita. Não existe
escape YAML válido — fix correto é rejeição em Load. Dois sítios: identity.go e render.go.

Defeito secundário (pré-existente, baixa severidade): bare comma em display_name causa rejeição
yaml.v3 (`needsYAMLQuoting` não cobre bare comma). Nova REQ necessária.

T-1 a T-4 confirmados fechados. Slug FP (`a_b`) era premissa incorreta do Wave-0 —
underscores nunca foram válidos no slugPattern.

---

*Hades, Security Reviewer — ML-4E Red Team — 2026-10-22*
