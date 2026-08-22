---
status: wip
date: 2026-08-21
req: "docs/req/REQ-2026-08-17-validate-nao-detecta-hook-de-guard-na-forma-relativa-antiga-que-falha-fora-da-raiz.md"
adr: ""
squad: "apolo-tf, hades-tf"
---

# Roadmap: `validate` detecta hook de guard na forma relativa antiga

> Created: 2026-08-21 | Status: wip

## Context

REQ: `docs/req/REQ-2026-08-17-validate-nao-detecta-hook-de-guard-na-forma-relativa-antiga-que-falha-fora-da-raiz.md`

Hook de guard escrito na **forma relativa antiga** funciona quando o comando roda na raiz do
repositório e **falha silenciosamente** fora dela. O `validate` não detecta — e o script está lá,
então nada acusa.

Última REQ de segurança antes da **7.2.0**.

## 🔴 O risco dominante é o falso-positivo, e ele decide o desenho

**Cursor e Copilot usam caminho relativo como forma correta**, por decisão registrada. Acusá-los
seria pior que a lacuna: quebra `validate` de quem está certo, e — pelo `ADR-2026-08-17` — guard que
atrapalha é guard que o usuário desliga.

A regra precisa distinguir **relativo que falha** de **relativo que é a forma certa daquele CLI**.
Essa distinção é o trabalho; o resto é mecânica.

## Riscos que valem para todos os MLs

1. **Não invadir a fronteira do `credential_guard_hook_resolvable`**, que já tem gate cross-CLI desde
   o ML-3A da REQ dos três contratos. Estender, não duplicar.
2. **Gate comparando as saídas reais** — teste por stack não fecha. Nove divergências reais nesta
   série.
3. **Invocação CI-exata:** `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make parity`.
4. Anotação `trackfw-contract` atualizada — o checker de cobertura é bloqueante.

---

## Wave 1 — Repro e regra

### ML-1A — Reproduzir a falha antes de escrever a regra
**Status:** ✅ Concluído · **Agente:** `apolo-tf`

**Parecer (2026-08-21):**

#### P1 — Qual é a "forma relativa antiga"?

Literal exato (credential-guard):
```
scripts/trackfw-credential-guard.sh
```
Literal exato (git-branch-guard):
```
scripts/trackfw-git-branch-guard.sh
```
Nomeadas como `GUARD_CMD_LEGACY` (Node `npm/src/generators/hooks.js:1173`) e equivalentes Go
(`agentfiles.go` — a migração sai de `"scripts/trackfw-credential-guard.sh"` para a forma com
prefixo, linhas 253–254 / 438–441 / 584–586) e Python (`pypi/trackfw/generators/hooks.py` —
`_migrate_hook_command(..., 'scripts/trackfw-credential-guard.sh', _GUARD_CMD_*)`).

#### P2 — Em quais CLIs a forma relativa é ERRADA, em quais é CORRETA?

Medido via ADR-2026-08-11 (tabela de decisão) + constantes dos 3 geradores:

| CLI | Forma atual (correta) | Relativa (`scripts/...`) |
|---|---|---|
| Claude Code | `$CLAUDE_PROJECT_DIR/scripts/...` | **ERRADA** — hooks rodam no cwd do agente |
| Gemini CLI | `$GEMINI_PROJECT_DIR/scripts/...` | **ERRADA** — por argumento de assimetria (ADR §Gemini) |
| Codex CLI | `"$(git rev-parse --show-toplevel)/scripts/..."` | **ERRADA** — cwd é o cwd da sessão |
| Cursor | `scripts/...` ← **É esta forma** | **CORRETA** — doc: "Run from the project root" |
| GitHub Copilot CLI | `scripts/...` + campo `"cwd":"."` | **CORRETA** — cwd pinado pelo campo estrutural |
| Kiro | `scripts/...` | **CORRETA** (indeterminado, default ADR: não alterar) |

Cursor e Copilot confirmados: relativo é a forma atual emitida pelos 3 geradores e não tem migração
pendente (constantes `GUARD_CMD_CURSOR`, `GUARD_CMD_COPILOT`, `GBG_CMD_CURSOR`, `GBG_CMD_COPILOT`
mantidas como `scripts/...`).

#### P3 — O hook na forma antiga falha fora da raiz?

**Sim. Reproduzido:**

Fixture: `.claude/settings.json` com `"command":"scripts/trackfw-credential-guard.sh"` (type:command),
script presente em `<root>/scripts/trackfw-credential-guard.sh`.

```
# Validação da raiz — resultado:
credential_guard_hook_resolvable: VAZIO  ← bug confirmado (nenhuma violação)

# Execução como Claude faria a partir de subdir:
$ cd fixture-a/subdir && /bin/sh scripts/trackfw-credential-guard.sh
/bin/sh: scripts/trackfw-credential-guard.sh: No such file or directory
exit 127
```

O validate conclui "ok" porque `resolveCredentialGuardHookPath()` trata `scripts/...` (sem `$`, sem
`"`, não absoluto) como relativo puro (case 4, linha 84) e resolve para `<root>/scripts/...` — que
existe. O script não é executado pelo validate; só o CLI de agente o executa em runtime, a partir do
cwd corrente, que pode ser qualquer subdiretório.

#### P4 — Qual sinal distingue as duas formas?

**Sinal limpo e decidível: o arquivo de host (qual CLI) combinado com a presença ou ausência de prefixo.**

- `scripts/trackfw-credential-guard.sh` em `.claude/settings.json` → forma antiga/errada
- `scripts/trackfw-credential-guard.sh` em `.cursor/hooks.json` → forma correta

A string de comando é idêntica. O que difere é o CLI. `credentialGuardHookFiles` já estrutura cada
entrada com o CLI correspondente. A tabela do ADR-2026-08-11 é inequívoca: para Claude, Gemini e
Codex, o prefixo/mecanismo é **obrigatório**; para Cursor/Copilot/Kiro, o relativo é o mecanismo
correto.

Não há ambiguidade: um mesmo valor de string só pode ser "forma antiga" num dos 6 contextos de host
file. O discriminante é `(host_file, forma_do_comando)`, não `forma_do_comando` sozinha.

#### Recomendação de desenho para ML-1B

Adicionar flag `requiresVarOrShellPrefix bool` em `credentialGuardHookFile`:

```go
{".claude/settings.json",                    "Claude Code",        true,  true },   // requiresCommandType, requiresVarOrShellPrefix
{".codex/hooks.json",                        "Codex CLI",          true,  true },
{".gemini/settings.json",                    "Gemini CLI",         true,  true },
{".cursor/hooks.json",                       "Cursor",             false, false},
{".github/hooks/trackfw-attention.json",     "GitHub Copilot CLI", true,  false},
{".kiro/hooks/trackfw-attention.json",       "Kiro",               true,  false},
```

Em `validateGuardHookResolvable`, após `resolveCredentialGuardHookPath` retornar `ok=true`, acrescentar:

```go
if hf.requiresVarOrShellPrefix && isRelativePure(m.raw) {
    msgs = append(msgs, fmt.Sprintf(
        "%s (%s) references %s with a bare relative path — "+
        "this command only resolves from the project root and will silently "+
        "fail when the agent's cwd is a subdirectory; run `trackfw update` to fix it",
        hf.path, hf.cli, scriptMarker,
    ))
    continue
}
```

onde `isRelativePure(raw)` = `!strings.HasPrefix(raw, "$") && !strings.HasPrefix(raw, "\"") && !filepath.IsAbs(raw)` — exatamente a negação dos 3 prefixos que `resolveCredentialGuardHookPath` já reconhece como "correto para Claude/Gemini/Codex".

**Trade-off:** depende do modelo de "qual CLI usa qual forma", que já vive em `credentialGuardHookFiles`
e no ADR — não introduz nova dependência; o padrão `requiresCommandType` já usa o mesmo mecanismo.
A alternativa (comparar com o que o gerador emitiria) é mais genérica mas duplica lógica do gerador
no validador e é mais frágil a drift. Opção 1 preferida.

**Falso-positivo (AC3):** por construção ausente. Cursor/Copilot/Kiro têm `requiresVarOrShellPrefix=false`
e nunca entram no branch de acusação, independente do valor da string.

**Critérios de aceite:**
- [x] As quatro respostas, com evidência medida
- [x] Nenhuma linha de regra escrita

### ML-1B — Implementar a regra
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` · **Dep.:** ML-1A
**Critérios de aceite:** ver AC1–AC4 da REQ. Em especial o **AC3** — Cursor e Copilot com relativo
**continuam limpos**.

---

## Wave 2 — Gate

### ML-2A — Gate de paridade + P4
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` · **Dep.:** ML-1B
**Estender o gate existente** do `credential_guard_hook_resolvable`, não criar paralelo.

**Critérios de aceite:**
- [ ] Forma antiga acusada nos 3 CLIs; forma correta silenciosa nos 6
- [ ] **Cursor e Copilot com relativo silenciosos** — o discriminante de falso-positivo
- [ ] Cenário P4 com baseline e detecção
- [ ] `cli-parity.md` nomeia o gate; checker de cobertura exit 0
- [ ] `make quality` verde · CI-exata verde

---

## Wave 3 — Barreira

### ML-3A — `hades-tf`
**Status:** ⬜ Pendente · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-21-revisao-da-deteccao-de-hook-relativo.md`

A regra decide se um guard está ativo. Avaliar se a detecção pode ser **contornada** por uma forma
que ela não reconhece, e se o falso-positivo em Cursor/Copilot foi de fato evitado. **Veredito
explícito.**

---

## Notas
- **Fora de escopo:** mudar a decisão de qual forma cada CLI usa. A regra **detecta**, não redefine.
- Commits e branch são exclusivos do `trackfw_architect`.
