---
date: 2026-08-22
author: "Hades (Security Reviewer)"
roadmap: "ROADMAP-2026-08-22-validate-detecta-hook-com-pwd-que-falha-fora-da-raiz.md"
ml: "ML-3A"
revision: "3 (reverificacao de ML-4A — tres ressalvas confirmadas fechadas; APROVADO COM RESSALVAS)"
---

# Revisao de Seguranca — Classificacao por Ancoragem (ML-3A)

> Revisao do classificador `classifyHookAnchorage` introduzido nas Waves 1–2 do
> `ROADMAP-2026-08-22-validate-detecta-hook-com-pwd-que-falha-fora-da-raiz.md`.
> Continuacao direta da barreira de 2026-08-21 (achados D.1, D.2, D.3).
>
> **Nota de revisao:** a primeira rascunho deste parecer (sessao anterior) mediou `~/` e
> `sh -c "$PWD/..."` usando paths errados para os CLIs Node.js e Python, recebendo SILENT falso.
> Esta revisao re-mediu com os tres stacks corretos. Os resultados mudaram o veredicto.

## Veredicto

**REPROVADO**

A entrega fecha o risco principal (D.1 — `$PWD/...` detectado, D.3 — `"$PWD/..."` entre aspas
detectado). Contudo, a entrega introduziu um **falso-positivo confirmado em todos os tres CLIs**:
`~/scripts/...` e `~/.trackfw/scripts/...` sao acusados como "bare relative path" com uma mensagem
factualmente errada. O sinal `~` expande para `$HOME` (caminho absoluto, ancorado) — nao e
cwd-dependent — e a mensagem "this command only resolves from the project root" e falsa para essa
forma. O caminho `~/.trackfw/scripts/trackfw-credential-guard.sh` e especificamente o local que o
proprio trackfw cria para o harness global.

O falso-positivo e o risco dominante identificado no ADR e na REQ. Accusar uma forma legitima faz
o usuario rodar `trackfw update` e sobrescrever um hook intencionalmente escrito. A entrega nao pode
ser aprovada sem que este caso seja tratado.

Achados bloqueantes e nao-bloqueantes estao listados na tabela abaixo por prioridade.

---

## Instrumento e validade das medicoes

Medicoes feitas com:
- Binario `./bin/trackfw` compilado na sessao (nao o do PATH)
- `node /Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw/npm/bin/trackfw`
- `PYTHONPATH=/Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw/pypi python3 -m trackfw`
- Fixtures gerados via `python3 -c "import json; ... print(json.dumps(d))"` — round-trip validado
  antes de confiar no resultado (alerta do roadmap sobre JSON malformado foi incorporado)
- Cada forma foi testada nos tres stacks; "SILENT" reportado apenas quando todos os tres
  produziram silencio (qualquer ACUSADO em um dos tres e anotado como tal)

A medicao anterior (sessao interrompida) usou paths errados para Node.js
(`npm/src/index.js` em vez de `npm/bin/trackfw`) e Python (`pypi/trackfw/cli.py` sem
PYTHONPATH em vez de `python3 -m trackfw` com PYTHONPATH), recebendo SILENT falso. Esta sessao
corrigiu os paths e re-mediu todas as formas criticas.

---

## P1 — A classe 2 pode ser contornada?

### Tabela de resultados medidos

Todas as formas abaixo testadas em `.claude/settings.json` (Claude Code,
`requiresVarOrShellPrefix=true`) com o marker `trackfw-credential-guard.sh`.
Os tres CLIs foram testados; a coluna "Parity" marca se todos os tres concordam.

| Forma testada | Go | Node | Python | Parity | Classe |
|---|---|---|---|---|---|
| `$PWD/scripts/trackfw-credential-guard.sh` | ACUSADO ($PWD msg) | ACUSADO | ACUSADO | OK | 2 |
| `"$PWD/scripts/..."` (aspas externas) | ACUSADO ($PWD msg) | ACUSADO | ACUSADO | OK | 2 (apos strip) |
| `$PWD/../scripts/...` | ACUSADO ($PWD msg) | ACUSADO | ACUSADO | OK | 2 |
| `${PWD}/scripts/...` | **SILENT** | **SILENT** | **SILENT** | OK (3 CLIs concordam) | 3 (gap) |
| ` $PWD/scripts/...` (espaco a esquerda) | ACUSADO (bare relative) | ACUSADO | ACUSADO | OK | 2 |
| `sh -c "$PWD/scripts/..."` | ACUSADO (**msg errada**) | ACUSADO (**msg errada**) | ACUSADO (**msg errada**) | OK (msg errada em todos) | 2 |
| `env FOO=x $PWD/scripts/...` | ACUSADO (**msg errada**) | ACUSADO (**msg errada**) | ACUSADO (**msg errada**) | OK (msg errada em todos) | 2 |

### Gap confirmado: `${PWD}/...`

`${PWD}/scripts/...` e SILENCIOSO nos tres CLIs. Causa:

```go
if strings.HasPrefix(rawStripped, "$PWD/") || ...
```

`${PWD}/` nao casa com o prefixo literal `$PWD/`. Nao comeca com `./` nem `../`. Comeca com `$`
logo a ultima clausula da classe 2 (`!strings.HasPrefix(rawStripped, "$")`) avalia para falso →
nao satisfaz classe 2 → cai em classe 3 (SILENCIOSA).

Em bash, `${PWD}/scripts/...` expande identicamente a `$PWD/scripts/...`. O ADR justifica
classe 3 para "variaveis de ambiente cujo valor nao podemos determinar em tempo de validacao".
`PWD` e mandado pelo POSIX e sempre definido pelo shell — seu valor e sempre "o diretorio de
trabalho corrente". A razao do ADR para a classe 3 nao se aplica a `PWD`: ele nao e uma variavel
privada do usuario; e uma variavel de ambiente de sistema com semantica conhecida e identica a
`$PWD`. O classificador tem uma **lista de prefixos literais** onde deveria ter um predicado
semantico. Este e um gap de implementacao, nao um residual legitimamente coberto pelo ADR.

Dito isso, a forma `${PWD}/` e extremamente incomum. O risco de exploite deliberado e baixo. O
gap nao e bloqueante por si so — o que o torna relevante e a combinacao com o achado A abaixo.

### Achado secundario: `sh -c "$PWD/..."` — deteccao correta, mensagem errada

`sh -c "$PWD/scripts/..."` e detectado corretamente (e cwd-dependent) mas recebe a mensagem
"bare relative path — this command only resolves from the project root", nao a mensagem "$PWD path".
A causa: `cwdDependentReason(rawStripped)` verifica `strings.HasPrefix(rawStripped, "$PWD")`, mas
`rawStripped = "sh -c \"$PWD/scripts/...\""` comeca com `sh`, nao com `$PWD`. A mensagem "bare
relative path" nao menciona `$PWD` e nao descreve o problema real.

**Risco de reinducao:** um usuario que ve "bare relative path" para `sh -c "$PWD/..."` pode
concluir que o problema e o wrapper `sh -c` e remove-lo, mantendo `$PWD/` — que seria acusado
com a mensagem correta. Nao e uma reinducao para uma forma silenciosa. Severidade: MODERADA
(mensagem errada, nao false-negative, nao bloqueante por si so).

---

## P2 — A classe 1 ficou livre de falso-positivo?

**NAO.** A entrega introduziu um falso-positivo confirmado: `~/scripts/...` e acusado nos tres
CLIs com mensagem factualmente errada.

### O que foi medido

| Forma | Go | Node | Python | Esperado |
|---|---|---|---|---|
| `/opt/scripts/trackfw-credential-guard.sh` | SILENT | SILENT | SILENT | classe 1 — OK |
| `$CLAUDE_PROJECT_DIR/scripts/...` | SILENT | SILENT | SILENT | classe 1 — OK |
| `$GEMINI_PROJECT_DIR/scripts/...` | SILENT | SILENT | SILENT | classe 1 — OK |
| `"$(git rev-parse --show-toplevel)/scripts/..."` | SILENT | SILENT | SILENT | classe 1 — OK |
| `~/scripts/trackfw-credential-guard.sh` | **ACUSADO** | **ACUSADO** | **ACUSADO** | classe 1 — **FALSO POSITIVO** |
| `~/.trackfw/scripts/trackfw-credential-guard.sh` | **ACUSADO** | **ACUSADO** | **ACUSADO** | classe 1 — **FALSO POSITIVO** |

### Analise do falso-positivo em `~/`

`classifyHookAnchorage` avalia `~/scripts/...`:

1. `strings.HasPrefix("~/...", "$CLAUDE_PROJECT_DIR/")` → falso
2. `strings.HasPrefix("~/...", "$GEMINI_PROJECT_DIR/")` → falso
3. `strings.HasPrefix("~/...", "$(git rev-parse --show-toplevel)/")` → falso
4. `filepath.IsAbs("~/...")` → **falso** — `~` nao e `/`; `filepath.IsAbs` nao expande tilde
5. `strings.HasPrefix("~/...", "$PWD/")` → falso
6. `strings.HasPrefix("~/...", "./")` → falso
7. `strings.HasPrefix("~/...", "../")` → falso
8. `!strings.HasPrefix("~/...", "$") && !filepath.IsAbs("~/...")` → **verdadeiro** → **classe 2**

O classificador trata `~/` como caminho relativo puro porque `filepath.IsAbs` nao expande tilde.
Mas `~` expande para `$HOME` em qualquer shell POSIX — e portanto e semanticamente ancorado: nao
depende do cwd. A mensagem "this command only resolves from the project root and will silently fail
when the agent's cwd is a subdirectory" e **factualmente falsa** para `~/scripts/...`.

**Por que `~/.trackfw/scripts/...` e critico:** esse e o path que o trackfw usa para o harness
global. Um usuario que configurou `~/.trackfw/scripts/trackfw-credential-guard.sh` como hook
global ve esse path acusado, roda `trackfw update`, e tem seu hook sobrescrito por
`$CLAUDE_PROJECT_DIR/scripts/...` — um path que so funciona no projeto raiz onde `trackfw update`
foi executado, quebrando a intencao do hook global. Este cenario satisfaz o gatilho do risco
dominante da REQ: falso-positivo causa acao corretiva que enfraquece o guard.

**Correcao esperada:** adicionar `strings.HasPrefix(rawStripped, "~/")` a classe 1 em
`classifyHookAnchorage`, nos tres stacks. Tilde e semanticamente equivalente a um caminho absoluto
da perspectiva de ancoragem.

---

## P3 — A classe 3 e ponto cego aceito ou porta nova?

A classe 3 e menor do que antes desta entrega: `$PWD/...` saiu dela. O conjunto atual de classe 3
e "qualquer `$VAR/...` que nao seja `$CLAUDE_PROJECT_DIR/`, `$GEMINI_PROJECT_DIR/`,
`$(git rev-parse --show-toplevel)/`, nem `$PWD/`".

Contudo, como discutido em P1: `${PWD}/...` esta na classe 3 sem justificativa ADR valida. `PWD`
e POSIX-mandado. A implementacao tem uma lista de prefixos literais onde deveria ter um predicado
semantico. Nao e uma nova porta de entrada critica (a forma e incomum), mas a framing do ADR
("residual aceito") nao se aplica a `${PWD}`.

---

## P4 — `$UNDEFINED/...` — consistencia com o ADR

`$UNDEFINED/scripts/trackfw-credential-guard.sh` → SILENCIOSO (medido). Classe 3.

Para `$UNDEFINED`, a justificativa do ADR e valida: o valor da variavel nao e conhecido em tempo
de validacao. Se `UNDEFINED` nao estiver definida, expande para `/scripts/...` (absoluto, falha);
se estiver definida pelo usuario, pode expandir para qualquer coisa. O classificador nao tem acesso
ao ambiente de runtime — e como o ADR registra, executar o hook para testar seria arbitrary code
execution. Silencio e correto para esta forma.

Sem inconsistencia com o ADR.

---

## P5 — A mensagem ensina o caminho certo?

**Para a forma primaria `$PWD/...`:** SIM. A mensagem "$PWD expands to the current working
directory, not the project root; run `trackfw update` to fix it" e precisa, desfaz a confusao
anterior, e aponta diretamente para o remedio. Nao induz um terceiro erro.

**Para `sh -c "$PWD/..."` e `env FOO=x $PWD/...`:** NAO. Recebem a mensagem "bare relative path",
que nao menciona `$PWD` e nao descreve o que esta errado. Um usuario que ve essa mensagem para
`sh -c "$PWD/..."` pode remover o wrapper em vez de corrigir o `$PWD`.

**Para `~/scripts/...`:** NAO. A mensagem "bare relative path — this command only resolves from the
project root" e factualmente errada. `~/.trackfw/scripts/...` nao falha "when the agent's cwd is a
subdirectory" — falha apenas se o arquivo nao existir ou nao for executavel.

---

## Achado pre-existente (nao introduzido por esta entrega)

Forma `"$CLAUDE_PROJECT_DIR/scripts/..."` com aspas externas → classe 1 (correta, `stripOuterQuotesForClassify`
funciona), mas `resolveCredentialGuardHookPath` recebe `m.raw` com as aspas. O prefixo
`"$CLAUDE_PROJECT_DIR/..."` (com aspa inicial) nao casa com `claudePrefix = "$CLAUDE_PROJECT_DIR/"`,
e a clausula de relativo-puro exclui strings que comecem com `"`. Resultado: `ok=false`, os checks
de existencia e executabilidade sao pulados. Um hook `"$CLAUDE_PROJECT_DIR/..."` apontando para um
script deletado e SILENCIOSO.

Este problema existia antes desta entrega e nao e introduzido por ela. Registro aqui porque e
uma superficie de pre-existente relevante para a barreira.

---

## Resumo dos achados por prioridade

| Prioridade | Achado | Disposicao |
|---|---|---|
| **BLOQUEANTE** | `~/...` e `~/.trackfw/...` acusados como "bare relative" nos 3 CLIs — falso-positivo com mensagem errada | Exige adicao de `strings.HasPrefix(rawStripped, "~/")` a classe 1 nos 3 stacks |
| Moderada | `sh -c "$PWD/..."` detectado corretamente mas recebe mensagem "bare relative" — nao menciona $PWD | Exige correto branching em `cwdDependentReason` (verificar se rawStripped contém `$PWD`) |
| Baixa | `${PWD}/...` → classe 3 silenciosa — nao legitimamente coberto pelo ADR, mas forma incomum | Debito nomeado; pode ser REQ futura |
| Pre-existente | Quoted `"$CLAUDE_PROJECT_DIR/..."` escapa checks de existencia/executabilidade | Presente antes desta entrega; registrado como observacao |
| Informativa | `$UNDEFINED/...` SILENCIOSO e consistente com ADR | Confirmado; nenhuma acao |
| Informativa | Formas `$CLAUDE_PROJECT_DIR`, `$GEMINI_PROJECT_DIR`, absoluto, Codex (com aspas) → SILENCIOSO | Confirmado; nenhuma acao |

---

## O que esta barreira NAO cobre

- `gitBranchGuardScriptMarker`: a logica `classifyHookAnchorage` e compartilhada com
  `git_branch_guard_hook_resolvable`. O falso-positivo de `~/` aplica-se igualmente ao git branch
  guard. A correcao deve ser aplicada de forma unificada.

- Parity de mensagem entre os 3 stacks para formas de mensagem-errada: as tres implementacoes
  produzem o mesmo resultado errado para `sh -c "$PWD/..."`, logo a paridade e consistente —
  mas consistentemente errada. A correcao de mensagem tambem deve ser aplicada nos 3 stacks.

---

## Instrucao para o arquiteto (original — revisao 2)

**Esta barreira reprova a entrega.** Duas correcoes sao necessarias antes de APROVADO:

1. **Bloqueante:** adicionar `strings.HasPrefix(rawStripped, "~/")` como condicao de classe 1 em
   `classifyHookAnchorage` — nos tres stacks (`internal/validator/validator_credential_guard.go`,
   `npm/src/validator/index.js`, `pypi/trackfw/validator.py`). Tilde e semanticamente absoluto.

2. **Moderado (recomendado junto):** corrigir `cwdDependentReason` para detectar `$PWD` dentro de
   um comando wrapper (`sh -c`, `env`). Uma abordagem: `strings.Contains(rawStripped, "$PWD")` em
   vez de apenas `strings.HasPrefix`. Verificar se isto nao introduz novo falso-positivo em formas
   onde `$PWD` aparece como argumento (ex: `git -C $PWD status`) antes de aplicar.

O gap de `${PWD}` nao e bloqueante e pode ser endereçado em REQ futura.

---

## Reverificacao ML-4A — Sessao 2026-08-22 (Hades, revisao 3)

### Veredito

**APROVADO COM RESSALVAS**

As tres ressalvas bloqueantes do parecer anterior foram medidas nos tres runtimes com fixtures
gerenciadas via `json.dump` Python (round-trip validado). A correcao fecha os tres achados.
O debito de mensagem de `"~/..."` e declarado e aceito para a release 7.2.0.

### Instrumento desta reverificacao

- Binario `./bin/trackfw` recompilado (`make build` — exit 0)
- `node /…/npm/bin/trackfw` (entrypoint correto)
- `PYTHONPATH=/…/pypi python3 -m trackfw` (entrypoint correto)
- Fixtures gerados com `python3 -c "import json, sys; ... print(json.dumps(...))"` — round-trip
  validado antes de confiar no resultado
- `cd "$tmpdir" && <cli> validate` — o validator usa `os.Getwd()`, nao aceita `--root`

Nota: o uso de `--root` nos testes da sessao anterior causava silencio falso universal. Corrigido
aqui executando os CLIs a partir do diretorio de fixture.

### Q1 — As tres ressalvas estao fechadas? (medido nos 3 runtimes)

| Forma | Go | Node | Python | Esperado | Status |
|---|---|---|---|---|---|
| `~/.trackfw/scripts/trackfw-credential-guard.sh` (sem aspas) | SILENT | SILENT | SILENT | classe 1 | FECHADA |
| `~/scripts/trackfw-credential-guard.sh` (sem aspas) | SILENT | SILENT | SILENT | classe 1 | FECHADA |
| `"~/.trackfw/scripts/trackfw-credential-guard.sh"` (com aspas) | ACUSADO | ACUSADO | ACUSADO | classe 2 | FECHADA |
| `${PWD}/scripts/trackfw-credential-guard.sh` | ACUSADO (msg PWD) | ACUSADO (msg PWD) | ACUSADO (msg PWD) | classe 2 | FECHADA |
| `sh -c "$PWD/scripts/trackfw-credential-guard.sh"` | ACUSADO (msg PWD) | ACUSADO (msg PWD) | ACUSADO (msg PWD) | classe 2, msg PWD | FECHADA |

Mensagem de `${PWD}` e `sh -c "$PWD/..."`: "$PWD expands to the current working directory, not the
project root; run `trackfw update` to fix it" — correta e especifica.

### Q2 — A correcao do til abriu porta nova?

Casos borda medidos nos 3 runtimes:

| Forma | Resultado | Analise |
|---|---|---|
| `~` sozinho | SILENT | Nao contem o marker; regra nao ativada. Irrelevante para ancoragem. |
| `~usuario/scripts/trackfw-credential-guard.sh` | ACUSADO ("bare relative path") | `~user` nao comeca com `~/`; cai em classe 2. Mensagem errada (nao e cwd-dependent). Edge case extremamente improvavel em contexto real. Nao bloqueante — observacao registrada. |
| `~/../scripts/trackfw-credential-guard.sh` | SILENT | Comeca com `~/` → classe 1. `~/../` resolve para um absoluto. Correto. |
| `$HOME/scripts/trackfw-credential-guard.sh` | SILENT (classe 3) | `$HOME` e tratado como variavel generica (classe 3). Semanticamente equivalente a `~/` mas o validador nao le o ambiente. Consistente com o ADR. |

**Observacao nova (nao bloqueante):** `~usuario/scripts/...` (tilde com nome de usuario diferente
do corrente) e acusado com mensagem "bare relative path" factualmente errada. O path expande para
o home de outro usuario — nao depende do cwd. Em contexto de hooks de agentes de IA, essa forma
e extremamente improvavel. Registrado como debito de baixa prioridade, menor que o gap de
`${PWD}` original.

### Q3 — Regressao nos casos aprovados anteriormente?

| Forma | Resultado | Avaliacao |
|---|---|---|
| `/opt/x/scripts/trackfw-credential-guard.sh` (absoluto) | SILENT | OK — classe 1 preservada |
| `"$(git rev-parse --show-toplevel)/scripts/..."` (Codex) | Acusa "script does not exist" | CORRETO — a forma e classe 1 (ancorada), o resolver encontra o path absoluto e o script nao existe no fixture de teste. Nao e regressao de classificacao; e o check de existencia funcionando. |
| `$OUTRA/scripts/trackfw-credential-guard.sh` | SILENT | OK — classe 3 preservada |
| `$CLAUDE_PROJECT_DIR/scripts/...` | Acusa "script does not exist" | CORRETO — mesma logica do Codex acima. |
| `$PWD/scripts/trackfw-credential-guard.sh` | ACUSADO (msg PWD) | OK — classe 2 preservada, msg correta |

Nenhuma regressao de classificacao detectada. Os avisos de "script does not exist" para
$CLAUDE_PROJECT_DIR e Codex sao esperados (o fixture nao tem o script em disco) e corretos.

### Q4 — Debito: mensagem de `"~/..."` e aceitavel para 7.2.0?

**Veredito: DEBITO ACEITAVEL para 7.2.0.**

Fundamento:
- A deteccao e correta: `"~/..."` e classe 2 (o til dentro de aspas duplas nao expande — o path
  nunca resolve).
- A mensagem "bare relative path — this command only resolves from the project root" e
  **direcionalmente correta** (a forma esta errada) mas **literalmente imprecisa** (`"~/..."` nao
  e cwd-dependent — o til simplesmente nao expande dentro de aspas duplas).
- O remedio sugerido (`run trackfw update`) e **correto**: o update substitui por
  `$CLAUDE_PROJECT_DIR/...` ou caminho absoluto real.
- Quem escreve `"~/..."` (tilde entre aspas duplas) em um hook de agente e uma ocorrencia
  vanishingly rare. A probabilidade de dano por mensagem enganosa e muito baixa.
- Fui eu quem restringiu o handoff a duas mensagens (apos aprovar o escopo do ML-4A). O
  debito e meu e esta documentado aqui para REQ futura.

O debito nao e bloqueante para 7.2.0. Deve ser endereçado em REQ subsequente junto com
`~usuario/...` (mesmo corretivo: terceira mensagem para tilde-dentro-de-aspas, ou instrucao
mais precisa).

### Suite de testes e paridade

- `go test ./...` → verde (todos os pacotes)
- `node --test npm/tests/validator.test.js` → 98 passed, 0 failed
- `python3 -m pytest pypi/tests/test_validator.py -q` → 121 passed
- `bash scripts/check-validate-parity.sh` → 18 casos CG passaram, byte-identical nos 3 CLIs
  (incluindo `claude-tilde`, `claude-tilde-quoted`, `claude-pwd-braced`, `claude-sh-c-pwd`)

### Instrucao ao arquiteto (revisao 3)

A barreira esta **levantada**. A entrega ML-4A esta **APROVADA COM RESSALVAS** documentadas:

1. **Aprovado:** `~/` sem aspas → classe 1 (SILENT) nos 3 CLIs.
2. **Aprovado:** `"~/..."` com aspas → classe 2 (ACUSADO). Mensagem imprecisa registrada como debito aceitavel.
3. **Aprovado:** `${PWD}/...` → classe 2, mensagem correta do PWD, nos 3 CLIs.
4. **Aprovado:** `sh -c "$PWD/..."` → classe 2, mensagem correta do PWD, nos 3 CLIs.
5. **Residual nomeado:** `~usuario/...` — acusado com mensagem errada. Edge case improvavel. REQ futura.
6. **Residual nomeado (pre-existente):** `${PWD}` como gap de prefixo literal vs predicado semantico — fechado pelo ML-4A (`${PWD}/` agora em classe 2).
7. **Residual nomeado (pre-existente, nao introduzido):** `"$CLAUDE_PROJECT_DIR/..."` com aspas escapa checks de existencia/executabilidade.

Prosseguir para o proximo ML do roadmap.
