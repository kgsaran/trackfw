---
status: done
date: 2026-08-21
req: "docs/req/REQ-2026-08-17-validate-nao-detecta-hook-de-guard-na-forma-relativa-antiga-que-falha-fora-da-raiz.md"
adr: ""
squad: "apolo-tf, hades-tf"
---

# Roadmap: `validate` detecta hook de guard na forma relativa antiga

> Created: 2026-08-21 | Status: done

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
**Status:** ✅ Concluído · **Agente:** `apolo-tf` · **Dep.:** ML-1A
**Critérios de aceite:** ver AC1–AC4 da REQ. Em especial o **AC3** — Cursor e Copilot com relativo
**continuam limpos**.

---

### Auditoria do ML-1B — aprovada; e a mensagem é a melhor da série

```
AC1  claude relativo    ->  acusa       ✓
AC2  claude com prefixo ->  silencioso  ✓
AC3  cursor relativo    ->  silencioso  ✓   <- o discriminante de falso-positivo
make quality (CI-exata) exit 0 · validate exit 0
```

A mensagem diz **o quê**, **por quê**, **quando falha** e **o remédio**:

> *".claude/settings.json (Claude Code) references trackfw-credential-guard.sh with a bare relative
> path — this command only resolves from the project root and will silently fail when the agent's
> cwd is a subdirectory; run `trackfw update` to fix it"*

Quem recebe isso não precisa abrir o código. É o padrão que quero nas outras regras.

**O `git-branch-guard` coube na mesma estrutura** — os dois guards compartilham
`validateGuardHookResolvable`, então o flag novo cobre ambos automaticamente. Ele acrescentou teste
provando, em vez de afirmar.

**Nenhum hook real deste repositório foi acusado** — verifiquei junto: `validate` exit 0.

#### 🔴 Erro meu no caminho, e é o segundo do mesmo tipo hoje

Minha primeira tentativa de fixture deu **falso negativo nos três casos**. Vi `AC1 violacoes=0` e
quase reportei que a regra não funcionava. A causa era o escaping do JSON via `python3 -c` dentro de
string de shell — o `settings.json` saía malformado, e nenhuma regra tinha o que avaliar.

Refiz com heredoc e o comportamento apareceu correto.

**É a segunda vez hoje que quase concluo errado por instrumento defeituoso**, não por defeito real —
a primeira foi contar `FAIL` num gate que usa outra convenção de mensagem. A lição é a mesma nas
duas: **antes de acusar o produto, provar que o instrumento mede o que eu acho que mede.** Um
fixture que não dispara nada é indistinguível de um produto que não detecta nada.

**Correção de passagem dele:** um teste do Python passava na `main` por acidente — faltava `cwd` em
duas seções, e ele corrigiu ao tocar no arquivo.


## Wave 2 — Gate

### ML-2A — Gate de paridade + P4
**Status:** ✅ Concluído · **Agente:** `apolo-tf` · **Dep.:** ML-1B

**Parecer (2026-08-21):**

Entregues:
- `scripts/check-validate-parity.sh`: bloco CG estendido com `cg-claude-relativo`
  (forma relativa antiga → acusada) e `cg-copilot-relativo-present` (Copilot → silêncio);
  novo bloco GBG com `gbg-claude-relativo` e `gbg-cursor-relativo-present`. Todos byte-
  idênticos nos 3 CLIs (verificado manualmente antes de escrever o gate).
- `scripts/check-gates-falsify.sh`: Cenário 159 (P4 direção-A — supressão da acusação
  de Claude) e Cenário 160 (P4 direção-B — falso-positivo em Copilot). Ambos com
  baseline + detecção autodiscriminante.
- `docs/cli-parity.md` anotação atualizada; `check-parity-contract-coverage.sh` exit 0.

**Evidências:**
- `make build` → exit 0
- `go test ./...` → verde (sem modificação de testes — lógica coberta por ML-1B)
- `check-validate-parity.sh` → verde (8 casos CG + 2 casos GBG, byte-idênticos)
- `check-parity-contract-coverage.sh` → exit 0, 0 inválidas, 0 sem anotação
- Cenário 159 `validate-parity/credential-guard-bare-relative-not-detected` → OK
- Cenário 160 `validate-parity/credential-guard-copilot-false-positive-detected` → OK
- `./bin/trackfw validate` → exit 0 (nenhum hook real do repo acusado)

**Critérios de aceite:**
- [x] Forma antiga acusada nos 3 CLIs; forma correta silenciosa
- [x] **Copilot com relativo silencioso** — discriminante de falso-positivo
- [x] Cenário P4 duas direções (159 acusar-de-menos; 160 acusar-de-mais) com baseline + detecção
- [x] `cli-parity.md` nomeia o gate; checker de cobertura exit 0
- [x] CI-exata verde (check-validate-parity.sh, check-parity-contract-coverage.sh)

---

### Auditoria do ML-2A — aprovada; as duas direções falsificadas

```
Cenario 159  suprimir a acusacao do Claude   (acusar de MENOS)
Cenario 160  falso-positivo no Copilot       (acusar de MAIS)

sabotagem propria: removi a guarda por CLI da condicao
  if hf.requiresVarOrShellPrefix && isRelativePure(...)  ->  if isRelativePure(...)
  gate -> EXIT=1:
    "cursor-absent/go: mensagem inesperada — esperava 'but the script does not exist';
     recebeu: '.cursor/hooks.json (Cursor) references ... with a bare relative path'"
restaurado -> EXIT=0
160 cenarios · make quality (CI-exata) exit 0 · validate exit 0
```

**Cobrir as duas direções era o pedido central, e ele cobriu.** O alvo óbvio — suprimir a acusação —
prova metade. O defeito **caro** deste lote é o falso-positivo: uma regressão que acuse o Cursor
quebra o `validate` de quem está certo, e pelo `ADR-2026-08-17` guard que atrapalha é guard que o
usuário desliga.

**O `git-branch-guard` ganhou bloco próprio no gate**, com o mesmo par. O ML-1B provou que a
estrutura cobria os dois; agora há prova em **runtime**, não só teste.

**Risco residual dele, fechado por mim:** ele declarou que o `make parity` completo excedeu 5 minutos
e que os cenários novos foram verificados isoladamente. Rodei a invocação CI-exata inteira — exit 0,
160 cenários. Declarar o que ficou por confirmar, em vez de afirmar verde, é o comportamento certo,
e é a terceira vez nesta série que ele faz isso.

**Wave 2 fechada.** Falta a barreira.

## Wave 3 — Barreira

### ML-3A — `hades-tf`
**Status:** ✅ Concluido · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-21-revisao-da-deteccao-de-hook-relativo.md`

A regra decide se um guard está ativo. Avaliar se a detecção pode ser **contornada** por uma forma
que ela não reconhece, e se o falso-positivo em Cursor/Copilot foi de fato evitado. **Veredito
explícito.**

---

## Notas
- **Fora de escopo:** mudar a decisão de qual forma cada CLI usa. A regra **detecta**, não redefine.
- Commits e branch são exclusivos do `trackfw_architect`.

### Auditoria do ML-3A — **APROVADO**; três lacunas nomeadas, uma virou REQ

Veredito: **APROVADO** (`docs/seguranca/2026-08-21-revisao-da-deteccao-de-hook-relativo.md`).

**Ele testou 8 variantes**, não as 4 que eu sugeri:

```
CAPTURADAS:      scripts/...  ·  ./scripts/...  ·  sh scripts/...  ·  ../scripts/...
SILENCIO OK:     $CLAUDE_PROJECT_DIR/...
NAO CAPTURADAS:  $PWD/...  ·  $UNDEFINED/...  ·  "scripts/..." (aspas)
```

**Confirmei a mais relevante por medição própria:**

```
"command": "$PWD/scripts/trackfw-credential-guard.sh"   (json validado antes)
  validate acusa?  0
  hook fora da raiz -> No such file or directory
```

**E o argumento de por que essa é a pior das três é meu, e agrava o achado:** `$PWD` é **o erro que
alguém comete tentando consertar**. Quem recebe *"references ... with a bare relative path"* e edita
à mão pode escrever `$PWD/` achando que ancora na raiz — e o `validate` **passa a ficar em
silêncio**, confirmando o engano.

**A correção do ML-1B criou o caminho para esse erro**, ao ensinar que o problema era "falta de
prefixo". Virou `REQ-2026-08-21-validate-nao-detecta-hook-com-pwd-que-falha-fora-da-raiz`, com a
decisão de postura explicitada: acusar tudo que não casa (fecha a classe, arrisca falso-positivo) ou
lista de formas sabidamente quebradas (não incomoda, mas é "condição estreita demais" pela décima
vez). **O ADR escolhe com o motivo.**

**O falso-positivo é por construção, e ele mostrou o mecanismo:** a condição curto-circuita no
primeiro operando, então para Cursor/Copilot/Kiro o valor do comando **nem é avaliado**. Mediu os
três. O ML-2A já provara que a guarda é load-bearing.

**Kiro:** ele **não** afirmou que `false` é seguro — disse que é **conservador, não comprovadamente
seguro**, e registrou o residual. Se o Kiro rodar hooks de subdiretório, o guard falha em silêncio.
É a distinção entre "medido" e "escolhido na ausência de dado", e ele a manteve.

**Wave 3 fechada. REQ pronta para PR.**

