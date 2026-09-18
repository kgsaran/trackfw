---
status: done
date: 2026-08-20
req: "docs/req/REQ-2026-08-20-tres-contratos-afirmados-no-cli-parity-sem-gate-cross-cli.md"
adr: ""
squad: "apolo-tf, hades-tf"
---

# Roadmap: gates para os três contratos de maior risco

> Created: 2026-08-20 | Status: done

## Context

REQ: `docs/req/REQ-2026-08-20-tres-contratos-afirmados-no-cli-parity-sem-gate-cross-cli.md`

Primeiro consumo da lista da triagem (42 `gap` + 51 `partial`). Três alvos, escolhidos por risco e
**confirmados por medição** antes de abrir a REQ.

## 🔴 Riscos que valem para todos os MLs

1. **Não afrouxar o gate para caber.** Windsurf e Amazon Q têm formato diferente dos outros seis; se
   o comparador estrutural não serve, **o comparador muda, não o critério**.
2. **Divergência real entre CLIs é achado, não conserto silencioso.** Aconteceu **cinco vezes** na
   semana passada. Registrar e abrir microlote próprio.
3. **Invocação CI-exata:** `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make parity`. Rodar o script direto
   não é a mesma coisa — três rodadas de CI se perderam por isso.
4. **Ao fechar cada um, a anotação da seção vira `gate=`.** O checker de cobertura é bloqueante desde
   o ML-3A da REQ anterior; anotação desatualizada reprova.

---

## Wave 1 — Windsurf e Amazon Q (o mais grave: alegação **falsa**)

### ML-1A — Avaliar o comparador antes de estender
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Arquivos:** nenhum de produto — **lote de investigação**, entrega um parecer curto no roadmap.

Os dois gates comparam **estrutura JSON** de 6 CLIs que compartilham forma. Windsurf usa arquivo
único `.windsurf/hooks.json` com `hooks.pre_run_command`; Amazon Q usa agente customizado em
`.amazonq/cli-agents/*.json`. **Provavelmente foi por isso que ficaram de fora.**

**Pergunta a responder com medição, não palpite:** o comparador atual estende para os dois formatos,
ou eles exigem comparador próprio? Se exigem, qual o desenho — e o que se perde em cada opção?

**Critérios de aceite:**
- [x] Resposta com evidência: forma real dos dois arquivos gerados pelos 3 CLIs, lado a lado
- [x] Recomendação explícita, com o trade-off
- [x] **Nenhuma linha de gate escrita** — decidir o desenho antes de codificar é o ponto do lote

#### Parecer (apolo-tf, 2026-08-20)

**Método:** fixture descartável em `$TMPDIR` (fora do repo e de `$HOME`), com `HOME` isolado por
runtime. Marcador de detecção colocado (`.windsurfrules` vazio + dir `.amazonq/`) e os três
binários reais invocados uma vez cada — `bin/trackfw discover --init` (Go), `node npm/bin/trackfw
discover --init`, `PYTHONPATH=pypi python3 -m trackfw discover --init`. Saída completa capturada;
os dois arquivos (`.windsurf/hooks.json`, `.amazonq/cli-agents/q_cli_default.json`) lidos dos três
diretórios de trabalho.

**1) Forma real, lado a lado.**

`Windsurf` — os 3 CLIs escrevem **exatamente** o mesmo JSON, byte a byte (após reformatação):
```json
{
  "hooks": {
    "pre_run_command": [
      { "command": "bash scripts/trackfw-git-branch-guard.sh", "show_output": true }
    ]
  }
}
```

`Amazon Q` — divergência real (ver item 4). Go escreve só os campos citados no próprio doc comment
de `InjectAmazonQHooks` (`name`, `description`, `tools`, `hooks`, `toolsSettings`). Node e Python
escrevem os mesmos campos **mais** `prompt`, `mcpServers`, `toolAliases`, `allowedTools`,
`resources`, `useLegacyMcpJson` — os campos que o doc comment do Go descreve como "deliberadamente
NÃO escritos aqui". O bloco `hooks.preToolUse` e `toolsSettings.execute_bash.deniedCommands` (o
`^git (commit|push|checkout -b)`) são **idênticos** nos 3.

**2) O comparador atual estende, ou exige um próprio?**
**Estende, sem alteração.** `compare_json` em `check-agent-hooks-parity.sh` já é um diff estrutural
JSON genérico e recursivo — não assume nada sobre a forma de nenhum CLI específico, só que existe
**um arquivo, em um caminho fixo, por CLI**. Windsurf (`.windsurf/hooks.json`) e Amazon Q
(`.amazonq/cli-agents/q_cli_default.json`) cumprem exatamente essa premissa: caminho fixo, um
arquivo, JSON parseável. Basta acrescentar duas entradas em `CLIS`, `marker_for()` e
`hookfile_for()`, seguindo a convenção já usada (`file:.windsurfrules` no mesmo estilo de
`file:CLAUDE.md`; `dir:.amazonq` no mesmo estilo de `dir:.cursor`/`dir:.kiro`). A hipótese inicial
do ML (formatos exigiriam comparador dedicado) **não se confirmou** — o formato de arquivo único
com caminho fixo é o que o comparador já assume; a divergência de Amazon Q vira `FAIL` automático
no diff genérico, que é o comportamento correto.

**3) Se exigisse comparador próprio: desenho e trade-off.** N/A — não exige. Registrado apenas por
completude do critério: a alternativa descartada seria um comparador dedicado por CLI (ex.: um para
"arquivo único com objeto de eventos", outro para "arquivo de agente nomeado"), que teria o mesmo
poder de detecção do genérico atual só que com mais código a manter — sem ganho, já que ambos os
formatos são "um arquivo JSON em um caminho fixo".

**4) Divergência real entre os 3 CLIs — achado, não conserto.**
**Sim, no Amazon Q.** Node.js e Python escrevem 6 campos extras (`prompt`, `mcpServers`,
`toolAliases`, `allowedTools`, `resources`, `useLegacyMcpJson`) que o Go **deliberadamente omite**
(doc comment de `InjectAmazonQHooks`, `internal/generators/agentfiles.go`, motivo: risco de campo
não esperado pelo schema real do Amazon Q, nunca confirmado contra a doc oficial nesta sessão
anterior). Registrado como achado — **não corrigido neste lote**; é o gatilho natural do ML-1B
(o `compare_json` vai reportar essa drift assim que a cobertura existir) e, se a correção de
comportamento for necessária, é microlote/REQ própria, não silenciosa. Windsurf: nenhuma
divergência encontrada.

**5) `deniedCommands` é parte do contrato — o gate precisa compará-lo?**
**Sim, e o desenho genérico já cobre isso de graça.** A tabela do `cli-parity.md` (linha "Amazon Q
Developer | hook `preToolUse` + `deniedCommands` regex... | deny global") declara os dois
mecanismos como o contrato — não só o hook. Como `compare_json` faz diff recursivo do JSON inteiro
(não um subcaminho escolhido a dedo), `toolsSettings.execute_bash.deniedCommands` já é comparado
automaticamente assim que o arquivo inteiro entra no diff — não precisa de nenhum caminho especial
no comparador.

**Achado adicional (guarda de vacuidade #2, não é o comparador estrutural):** o segundo guard de
`check-agent-hooks-parity.sh` (`grep -q "trackfw-credential-guard.sh"`) não se aplica a Windsurf/
Amazon Q — nenhum dos dois tem wiring de credential-guard em nenhum dos 3 CLIs (confirmado por
`grep` em `agentfiles.go`/`hooks.js`/`hooks.py`: só `git-branch-guard` é injetado para esses dois).
O ML-1B precisa trocar essa string por `trackfw-git-branch-guard.sh` **especificamente para essas
duas entradas** (as outras 6 continuam checando `credential-guard.sh`, que é o que elas de fato
wireiam nesse arquivo).

**Recomendação para o ML-1B:** estender as 3 tabelas (`CLIS`, `marker_for`, `hookfile_for`) com
`windsurf`/`amazonq` nesse mesmo script, ajustar a string do guard de vacuidade #2 para essas duas
entradas, e deixar `compare_json` intocado — ele já vai reportar a divergência do item 4 como
`FAIL`, que deve ser corrigida (comportamento, fora deste lote) ou explicitamente aceita/registrada
antes de fechar o ML-1B como verde. Nenhum comparador novo, nenhum script novo.

### ML-1B — Implementar a cobertura decidida
**Status:** ✅ Concluído · **Agente:** `apolo-tf` · **Dependência:** ML-1A
**Critérios de aceite:**
- [x] Windsurf e Amazon Q cobertos, nos 3 CLIs, comparando saídas/artefatos reais
- [x] Cenário P4 com baseline e detecção
- [x] A anotação da seção deixa de afirmar cobertura inexistente
- [x] `make quality` verde

#### Execução (apolo-tf, 2026-08-20)

**A alegação do texto NÃO se confirmou ao ser verificada — e o motivo era mais grave do que
"faltava cobrir".** A seção "Git branch guard por runtime" já apontava (via a própria anotação
`trackfw-contract` deixada pelo ML-1A) que Windsurf/Amazon Q não tinham gate cross-CLI. Verificando
antes de codar, achei uma segunda alegação falsa, mais específica, na seção "Caminhos confirmados —
Windsurf e Amazon Q": ela dizia que a correção de caminho/schema (ML-3A pós-auditoria) tinha
"byte-identidade confirmada via `check-agent-hooks-parity.sh` **e** `check-harness-hooks-parity.sh`".
**A segunda metade é impossível, não só não-verificada:** os dois arquivos corrigidos
(`.windsurf/hooks.json`, `.amazonq/cli-agents/q_cli_default.json`) são de **project-scope**;
`check-harness-hooks-parity.sh` só compara arquivos de **global-scope** em `~/.<tool>/...`, que não
existem para Windsurf/Amazon Q — confirmado lendo `internal/generators/update.go`
(`harnessCatalogTargetOrder`/`buildHarnessTargetIDs`) e o espelho em `npm/src/commands/
update-harness.js`/`pypi/trackfw/commands/update_harness.py`: nenhum dos 3 CLIs tem um target
`windsurf-credential-guard`/`windsurf-git-branch-guard`/`amazonq-credential-guard`/`amazonq-git-
branch-guard`. Windsurf não tem mecanismo de hook global nativo (decisão registrada no próprio
comentário do código, "stays out per the ADR"); Amazon Q simplesmente nunca recebeu esse par.
`check-harness-hooks-parity.sh` **nunca poderia** ter provado essa frase — não é uma lacuna de gate,
é ausência de artefato para gatear. Reportado como achado, não consertado (consertar seria mudança
de produto, fora de escopo): a menção a `check-harness-hooks-parity.sh` nessa frase foi removida, e
o header do próprio `check-harness-hooks-parity.sh` ganhou um parágrafo explicando por que ele nunca
vai cobrir os dois.

**Guard de vacuidade #2 ajustado como o ML-1A previu:** a segunda camada do gate
(`check-agent-hooks-parity.sh`) grepava `trackfw-credential-guard.sh` incondicionalmente — mas
Windsurf/Amazon Q só wireiam git-branch-guard, nunca credential-guard (confirmado por grep em
`agentfiles.go`/`hooks.js`/`hooks.py`). Adicionei `guard_marker_for()`: retorna
`trackfw-git-branch-guard.sh` para esses dois, `trackfw-credential-guard.sh` para os outros 6 —
continua sendo guard real (reprova se o marcador não aparecer), só deixou de assumir um marcador
único para os 8 CLIs.

**O que mudou, por arquivo:**
- `scripts/check-agent-hooks-parity.sh` — `CLIS` ganhou `windsurf amazonq`; `marker_for`/
  `hookfile_for` ganharam as duas entradas (`file:.windsurfrules`→`.windsurf/hooks.json`,
  `dir:.amazonq`→`.amazonq/cli-agents/q_cli_default.json`, confirmados contra
  `internal/generators/hooks.go:InjectHooksDetected`); comentários de cabeçalho atualizados de "6
  CLIs"/"18 invocações" para "8 CLIs"/"24 invocações". `compare_json` **não mudou uma linha** — como
  o ML-1A previu, o diff JSON recursivo genérico já cobre os dois formatos novos e o
  `toolsSettings.execute_bash.deniedCommands` do Amazon Q de graça.
- `scripts/check-harness-hooks-parity.sh` — só o header comentário, explicando a exclusão
  estrutural (nenhuma mudança em `CLIS`/lógica: não há artefato a comparar).
- `scripts/check-gates-falsify.sh` — Cenário 78 novo: corrompe `tools: ['*']` → `tools: ['read']`
  em `injectAmazonQHooks` (Node, único no arquivo) numa cópia isolada de `npm/`, prova que
  `check-agent-hooks-parity.sh` reprova em `agent-hooks-parity/amazonq/go-vs-node` com o path JSON
  `$.tools[0]` — sem essa prova, a extensão de `CLIS` poderia estar presente só de nome (ex.: um
  `marker_for`/`hookfile_for` errado comparando um arquivo vazio dos dois lados, "passando" por
  vacuidade mútua) sem o comparador jamais ser exercitado para os dois CLIs novos. Contagem: 146→147.
- `docs/cli-parity.md` — duas correções: (1) a anotação da seção "Git branch guard por runtime"
  (linha 4258) passou de `partial=` genérico para um `partial=` que nomeia exatamente o que
  `check-agent-hooks-parity.sh` cobre (8 CLIs, project-scope) vs. o que `check-harness-hooks-
  parity.sh` cobre (6 CLIs, global-scope) e por quê os outros 2 ficam fora por design; (2) a seção
  "Caminhos confirmados — Windsurf e Amazon Q" saiu de `gap` para `gate=scripts/check-agent-hooks-
  parity.sh partial=...` (identidade semântica, não byte-idêntica — nota cruzada com a seção
  ML-1A-bis) e a menção falsa a `check-harness-hooks-parity.sh` foi removida do texto.

**Saídas literais:**
- `go build ./...` / `go vet ./...` — sem erro
- `GO_BIN=bin/trackfw bash scripts/check-agent-hooks-parity.sh` — todos os 8 CLIs `OK` (16 linhas
  go-vs-node/go-vs-py), incluindo `amazonq/go-vs-node` e `amazonq/go-vs-py` — a real divergência de 6
  campos do Amazon Q que o ML-1A-bis fechou fica coberta e verde
- `bash scripts/check-harness-hooks-parity.sh` — inalterado, todos os 6 CLIs `OK`
- `bash scripts/check-parity-contract-coverage.sh` — exit 0, zero seções sem anotação, zero
  anotação inválida
- `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 bash scripts/check-gates-falsify.sh` — `Falsification checks
  passed (all 147 scenarios...)`, incluindo `OK [falsify/agent-hooks-parity/amazonq/go-vs-node-
  tools-drift-not-detected]`
- `make quality` — exit 0, zero `FAIL` no log completo (2804 linhas)
- `./bin/trackfw validate` — exit 0, 17 warnings pré-existentes (nenhum novo)
- `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make parity` — `REAL_EXIT=0`, zero `FAIL`

---

### Auditoria do ML-1A — aprovada, e **minha hipótese estava errada**

Escrevi na REQ que o formato divergente de Windsurf/Amazon Q *"provavelmente exigiria comparador
próprio"*. **Não exige.** O `compare_json` do `check-agent-hooks-parity.sh` é um diff JSON recursivo
genérico; a única premissa que ele faz é *"um arquivo, caminho fixo, por CLI"* — e os dois cumprem.
Basta acrescentar entradas em `CLIS`/`marker_for`/`hookfile_for`, na convenção que já existe.

O lote de investigação se pagou pelo motivo inverso do esperado: em vez de evitar um desenho errado,
**evitou um desenho desnecessário**. Se eu tivesse mandado implementar direto com a minha hipótese,
teríamos ganhado um comparador paralelo para nada.

**`deniedCommands` é coberto de graça** — o diff é do JSON inteiro, então
`toolsSettings.execute_bash.deniedCommands` entra sem caminho especial. Era a minha dúvida nº 5 e a
resposta é melhor que a esperada.

**Achado secundário, que teria custado uma rodada:** o guard de vacuidade nº 2 dos gates procura a
string `trackfw-credential-guard.sh` — e **nem Windsurf nem Amazon Q cabeiam credential-guard**, só
git-branch-guard. Acrescentá-los sem trocar essa string faria o gate reprovar por motivo errado.

---

### 🔴 ML-1A-bis — Divergência real de produto no Amazon Q (achado, decisão tomada)
**Status:** ✅ Concluído · **Agente:** `apolo-tf` · **Bloqueia o ML-1B.**

Medido: Node e Python escrevem **6 campos** no `q_cli_default.json` que o Go não escreve —
`prompt`, `mcpServers`, `toolAliases`, `allowedTools`, `resources`, `useLegacyMcpJson`.

**É a 6ª divergência real desta série**, e o gate do ML-1B reprovaria por causa dela — corretamente.

**Decisão: o Go é o canônico; Node e Python se alinham a ele.** O motivo está escrito no próprio
código (`agentfiles.go:1400-1415`), e é assimetria de risco:

> *"an extra field the real schema doesn't expect risks failing validation, whereas an absent
> optional field usually doesn't"*

Campo extra pode **quebrar** a validação do agente; campo opcional ausente normalmente não. Entre as
duas, escrever de menos é o lado seguro. Nada na implementação de Node/Python justifica os extras.

**Nota que herdamos:** o comentário do Go pede explicitamente *"verify this defaults set against the
live doc (or a real `q chat --agent` run) before treating it as final"* — e **ninguém verificou**.
Fica registrado como limite conhecido da decisão, não como verificação feita.

**Critérios de aceite:**
- [x] Node e Python param de escrever os 6 campos; `q_cli_default.json` byte-idêntico nos 3
- [x] Contrato de merge preservado: campo já presente em arquivo existente **não** é removido —
      só deixa de ser criado. Nunca clobbar customização do usuário
- [x] O `cli-parity.md` registra a decisão e o limite (a verificação contra a doc viva não foi feita)
- [x] `make quality` verde

#### Execução (apolo-tf, 2026-08-20)

`npm/src/generators/hooks.js` (`injectAmazonQHooks`) e `pypi/trackfw/generators/hooks.py`
(`inject_amazonq_hooks`) passaram a escrever só `name`, `description`, `tools` na criação do
`q_cli_default.json` — os mesmos 3 campos do Go, removendo `prompt`, `mcpServers`, `toolAliases`,
`allowedTools`, `resources`, `useLegacyMcpJson` dos dicts de default. O contrato "só define se
ausente" (`setdefault`/`hasOwnProperty`) não mudou — não há remoção de campo em arquivo existente,
só deixou de criar os 6 a mais numa instalação nova.

**Prova de byte-identidade (3 binários reais, fixture isolada em `$TMPDIR` com `$HOME` redirecionado,
fora do repo):** `bin/trackfw discover --init` (Go), `node npm/bin/trackfw discover --init`,
`PYTHONPATH=pypi python3 -m trackfw discover --init`, cada um contra seu próprio diretório de
trabalho isolado. `jq -S` normalizando ordem de chave + `diff` par a par
(go×node, go×py, node×py) sobre os 3 `.amazonq/cli-agents/q_cli_default.json` gerados: diff vazio
nos 3 pares — a única divergência bruta era ordem de chave (não-semântica em JSON).

**Prova de preservação de customização (mesmo método, arquivo pré-existente):** os 3 diretórios de
trabalho receberam previamente um `q_cli_default.json` com `mcpServers: {myserver: {command: foo}}`
e `useLegacyMcpJson: true` escritos manualmente; após rodar `discover --init` nos 3, os dois campos
sobreviveram intactos nos 3 arquivos (confirmado via `jq .`) — nada foi removido, só deixou de ser
criado do zero.

`docs/cli-parity.md` ganhou a seção "Campos mínimos do custom agent Amazon Q — Go como canônico
(2026-08-20, ML-1A-bis)", registrando a decisão por assimetria de risco **e** o limite explícito:
a escolha não foi verificada contra a doc viva da AWS nem contra um `q chat --agent` real — só
resolve a divergência entre os 3 CLIs, não confirma o schema real do Amazon Q. Testes ajustados:
Node (`npm/tests/git_branch_guard.test.js`) já não fixava os 6 campos extras, nenhuma mudança
necessária; Python (`pypi/tests/test_git_branch_guard.py::test_amazonq`) trocado de
`assertEqual(...)` positivo para `assertNotIn(...)` nos 6 campos.

**Saídas literais:**
- `go build ./...` — sem erro
- `node --test npm/tests/git_branch_guard.test.js` — `44 passed, 0 failed`
- `PYTHONPATH=pypi python3 -m pytest pypi/tests/test_git_branch_guard.py -q` — `34 passed, 6 subtests passed`
- `go test ./internal/generators/...` — `ok`
- `make quality` — `[exited with code 0]` (146 cenários de falsificação + gates de terceiros, todos `OK`)
- `./bin/trackfw validate` — exit 0, 17 warnings pré-existentes (nenhum novo introduzido por este ML)
- `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make parity` — `REAL_EXIT=0`


### Auditoria do ML-1A-bis — aprovada, com uma correção do **meu** critério de aceite

Gerei os artefatos com os três binários reais, em fixture isolado, e comparei:

```
chaves de topo:  go / node / py  ->  ['description','hooks','name','tools','toolsSettings']
                                     identicas, os 6 campos extras sumiram
comparacao semantica profunda:   go == node  True  ·  go == py  True
bytes brutos:                    DIFEREM  (ordem de chave / formatacao)
```

**Meu critério dizia "byte-idêntico", e estava errado.** O que se alcançou — e o que **deve** ser
alcançado — é **identidade semântica**. O comparador dos gates (`compare_json`) é um diff JSON
recursivo: ordem de chave não é contrato, e exigir byte-identidade acoplaria o teste a um detalhe de
serialização que nenhum dos três CLIs promete. Corrijo o critério, não o trabalho.

**A preservação de customização foi provada, não afirmada:** ele plantou `mcpServers` e
`useLegacyMcpJson` num arquivo pré-existente nos três diretórios antes de rodar, e os dois campos
sobreviveram intactos. O contrato *"deixa de criar, nunca remove"* está mantido — era o conflito que
eu tinha mandado ele parar e me consultar se aparecesse. Não apareceu.

**O limite ficou escrito** no `cli-parity.md`: o conjunto mínimo é decisão por assimetria de risco,
**não** verificação contra a documentação viva da AWS nem contra um `q chat --agent` real.

`make quality` (CI-exata) exit 0 · checker de cobertura exit 0 · `validate` exit 0.


### Auditoria do ML-1B — aprovada; a alegação era **pior** do que "não verificada"

```
CLIS agora: claude codex gemini copilot cursor kiro windsurf amazonq   (8/8)
sabotagem propria: "tools": ["*"] -> ["execute_bash"]  (literal unico, Go)
  -> EXIT=1, 2 FAIL: amazonq/go-vs-node e amazonq/go-vs-py, "structural drift"
restaurado -> todos passam
147 cenarios · make quality (CI-exata) exit 0 · cobertura exit 0 · validate exit 0
```

#### A descoberta central: a alegação era **impossível**, não apenas não-verificada

O texto afirmava byte-identidade confirmada por `check-agent-hooks-parity.sh` **e** por
`check-harness-hooks-parity.sh`. A segunda metade **nunca poderia ser verdade**: os caminhos de
Windsurf e Amazon Q são de **escopo de projeto**, e aquele gate só compara escopo **global**
(`~/.<tool>/...`). Não é lacuna de gate — é **ausência de artefato para gatear**.

Confirmado por ele na fonte, nos 3 CLIs: nenhum tem target de harness para os dois. Windsurf **não
tem mecanismo de hook global nativo** (decisão registrada no próprio comentário do gerador); Amazon
Q nunca recebeu o par.

Eu classifiquei isto como "alegação falsa de cobertura" na triagem. Estava certo, mas por um motivo
mais fraco do que o real: eu achava que faltava rodar o gate. **Faltava o gate ser capaz.**

#### O resultado ficou melhor que o meu critério de aceite

Eu escrevi *"a anotação vira `gate=`"*. Ela virou `gate=...partial=...`, e está **mais correta**:
declara os 8 CLIs cobertos em escopo de projeto, os 6 em escopo global, e **por que** os outros dois
nunca terão o segundo. Forçar `gate=` pleno teria reintroduzido a mesma classe de imprecisão que o
lote existia para eliminar — só que a meu favor desta vez, o que é pior.

**Guard de vacuidade ajustado como o parecer do ML-1A previu:** `guard_marker_for()` por CLI, para
que Windsurf e Amazon Q sejam checados contra `trackfw-git-branch-guard.sh` — o que de fato cabeiam
— e não contra `credential-guard`. Continua sendo guard: marcador ausente **reprova**.

Nota de vault registrada sobre a impossibilidade estrutural, para ninguém tentar "consertar" a
cobertura de harness dos dois no futuro.


## Wave 2 — `branch_has_wip_roadmap` com `done/`

### ML-2A — Cenário cross-CLI com roadmap em `done/`
**Status:** ✅ Concluído · **Agente:** `apolo-tf`
**Arquivos:** `scripts/check-branch-new-parity.sh` e/ou `check-validate-parity.sh`,
`scripts/check-gates-falsify.sh`, `docs/cli-parity.md`.

Medido: os fixtures dizem literalmente *"wip/ and done/ deliberately left empty"*, e o gate do
`validate` tem **zero** ocorrências da regra. O comportamento que define a `REQ-2026-07-26` nunca foi
exercitado entre os 3 CLIs.

**Critérios de aceite:**
- [ ] Fixture com roadmap correspondente em `done/` e branch de slug igual → **aceita**, nos 3
- [ ] Não-regressão: sem roadmap em lugar nenhum → **recusa**, nos 3
- [ ] Discriminante: roadmap em `done/` com slug **diferente** → recusa
- [ ] Cenário P4 sabotando a aceitação de `done/` e provando gate vermelho
- [ ] `make quality` verde

#### Execução (apolo-tf, 2026-08-20) — pronta para auditoria, status/checkboxes deixados para o `trackfw_architect`

**Onde o gate foi posto e por quê:** `scripts/check-validate-parity.sh`, não `check-branch-new-
parity.sh`. A própria evidência da REQ cita "`check-validate-parity.sh` não tem nenhuma ocorrência
da regra" — é o alvo mais literal. `TRACKFW_BRANCH` é suportado de forma idêntica pelos 3 CLIs
(`internal/validator/validator.go`, `npm/src/validator/index.js`,
`pypi/trackfw/validator.py:validate_branch_has_wip_roadmap`), o que permite exercitar `validate`
sem `git checkout` real — mais simples que `check-branch-new-parity.sh`, que já cobre wip/ (b) e
ausência (a/f) mas nunca precisou de `done/` porque `branch new` e `validate` chamam a mesma
`BranchSlugMatchesRoadmap`/`branchSlugMatchesRoadmap`/`branch_slug_matches_roadmap` (não é uma
segunda implementação).

**Os 3 casos, medidos com os binários reais:**
1. Roadmap em `done/` com slug igual → **zero violação** nos 3 (aceito — o caso central que a
   seção nunca provou).
2. Nenhum roadmap em lugar nenhum → bloqueia nos 3, mensagem "no roadmap is in wip/ nor done/"
   (não-regressão).
3. Roadmap em `done/` com slug **diferente** → bloqueia nos 3, mensagem "no matching roadmap in
   wip/ nor done/" (discriminante — sem ele um gate que aceitasse qualquer roadmap em `done/`
   passaria por acidente).

**Concordam os 3 CLIs de verdade no caso `done/`?** Sim, na semântica que importa (aceita/bloqueia,
texto da mensagem, exit code) — byte-idêntico nos 3. **Achado registrado, não corrigido:** Python's
`validate --json` tagueia esta UMA regra com `"rule": null, "file": null` —
`validate_branch_has_wip_roadmap` (pypi) retorna `list[str]`, não o formato dict que
`_enrich_items` sabe enriquecer; Go/Node tagueiam `"rule": "branch_has_wip_roadmap"` corretamente.
Isso é só a tag estruturada do `--json`, não a semântica de `done/`. O gate filtra por substring de
mensagem (`"wip/ nor done/"`, confirmado único nos 3 validators via grep antes de usar) em vez de
por `rule`, e pina a divergência de tag explicitamente para que uma mudança futura em qualquer lado
reprove. Detalhado em `vault/notes/validate-branch-has-wip-roadmap-done-python-rule-null-
2026-08-20.md`.

**Regressão própria pega e corrigida antes de fechar:** adicionar suporte a `GO_BIN` em
`check-validate-parity.sh` (necessário para o P4 sem cópia de script) quebrou o Cenário 4
pré-existente — `make parity` exporta `GO_BIN=bin/trackfw` (relativo) só para a linha do
`check-gates-falsify.sh` no Makefile, e essa variável vazava para o Cenário 4 (que nunca precisou
dela antes), resolvendo contra o `ROOT_DIR` errado. Corrigido com `env GO_BIN="$ROOT_DIR/bin/
trackfw"` explícito, mesma convenção já usada pelos Cenários 42/78.

**Cenário 79** (`check-gates-falsify.sh`, 147→148): baseline (binário real passa limpo) + detecção
(corrompe `BranchSlugMatchesRoadmap` dropando `doneDirs` do slice varrido — delta de literal único
— binário Go isolado, `check-validate-parity.sh` real apontado para ele via `GO_BIN`, Node.js/
Python reais) — prova que o bloco novo reprova se a aceitação de `done/` quebrar em qualquer um dos
3 runtimes.

`docs/cli-parity.md`: anotação da seção saiu de `gap` para `gate=scripts/check-validate-parity.sh
partial=...`, nomeando as 3 linhas cobertas, a redundância declarada com `check-branch-new-
parity.sh` e o achado do `rule: null`.

**Saídas literais:**
- `go build ./...` — sem erro
- `bash scripts/check-parity-contract-coverage.sh` — exit 0, zero anotação inválida/seção sem
  anotação
- `GO_BIN=bin/trackfw bash scripts/check-validate-parity.sh` — todos os blocos OK, incl. o novo
  "branch_has_wip_roadmap done/ acceptance parity checks passed (match/no-roadmap/diff-slug
  discriminant, byte-identical across 3 CLIs)"
- `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 bash scripts/check-gates-falsify.sh` — "Falsification checks
  passed (all 148 scenarios...)", incl. `OK [falsify/validate-parity/branch-has-wip-roadmap-done-
  acceptance-baseline]` e `OK [falsify/validate-parity/branch-has-wip-roadmap-done-acceptance-not-
  detected]`
- `make quality` — exit 0, zero FAIL
- `./bin/trackfw validate` — exit 0, 17 warnings pré-existentes (mesma contagem do ML-1B, nenhum
  novo — a nota nova do vault não ficou órfã)
- `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make parity` — zero FAIL, "all 148 scenarios" confirmado, e
  as duas linhas do Cenário 79 (baseline + detecção) presentes no log

---

### Auditoria do ML-2A — aprovada, e achou a **7ª divergência**

```
sabotagem propria: dirs := append(wipDirs, doneDirs...)  ->  append(wipDirs)   (literal unico)
  gate -> EXIT=1: "roadmap correspondente em done/ deveria ser aceito (zero violacoes),
                   mas go reportou [...no roadmap is in wip/ nor done/...]"
restaurado -> passa
148 cenarios · make quality (CI-exata) exit 0 · cobertura exit 0 · validate exit 0
```

O script usa convenção de mensagem própria em vez do prefixo `FAIL` — por isso meu `grep -c FAIL`
deu zero num run que saiu com `EXIT=1`. **Quase registrei como não-detecção.** Anoto para não repetir:
contar `FAIL` não substitui olhar o código de saída, e gates desta base não seguem uma convenção só.

**A escolha de onde pôr o gate é dele e está certa:** `check-validate-parity.sh`, não
`check-branch-new-parity.sh`. A evidência da REQ apontava aquele script como o de zero ocorrências
da regra, e o `TRACKFW_BRANCH` conduz o `validate` direto, sem precisar de `git checkout` real —
fixture mais simples e mais determinístico.

**Os três casos, com o discriminante:** `done/` com slug igual aceita; nenhum roadmap recusa; `done/`
com slug **diferente** recusa. Sem o terceiro, uma implementação que aceitasse *qualquer* roadmap
concluído passaria — e seria pior que o bug original.

#### 7ª divergência real, fixada e não corrigida

```
TRACKFW_BRANCH=feat/inexistente  validate --json
  Go      ->  rule: "branch_has_wip_roadmap"
  Python  ->  rule: None
```

Confirmei por medição. `validate_branch_has_wip_roadmap` (`pypi/trackfw/validator.py:1436`) devolve
`list[str]` em vez da forma que o `_enrich_items` enriquece. Mensagem e exit code idênticos; só o
`rule` do JSON diverge — e `--json` é o que integração de CI consome, então quem filtra por `rule`
perde esta regra **em silêncio**.

Ele **fixou a divergência no gate**, para que deriva em qualquer direção reprove alto, e abriu a
questão em vez de consertar. Correto: virou
`REQ-2026-08-20-validate-json-do-python-nao-rotula-a-regra-branch-has-wip-roadmap`, com um AC que
exige **varrer as demais regras** — se uma devolve a forma errada por descuido, é improvável que
esteja sozinha.

**Regressão que ele mesmo causou e consertou:** ao dar suporte a `GO_BIN` no
`check-validate-parity.sh`, o `GO_BIN=bin/trackfw` relativo do `make parity` vazou para a cópia
isolada do Cenário 4 e resolveu contra o `ROOT_DIR` errado. Corrigiu com override absoluto, na
convenção que os Cenários 42/78 já usavam. Achou porque rodou a **invocação CI-exata** — rodar só o
script direto não teria mostrado.


## Wave 3 — `credential_guard_hook_resolvable` cross-CLI

### ML-3A — Estender a prova para Node e Python
**Status:** ✅ Concluído · **Agente:** `apolo-tf`

O Cenário 47 declara no próprio comentário ser prova black-box da regra **Go**. É o controle que o
`ADR-2026-08-12` aponta como o que resta mitigando o fail-open — com prova em um terço dos runtimes.

**Critérios de aceite:**
- [ ] Regra exercitada nos 3 CLIs, com hook registrado apontando para script ausente
- [ ] Não-regressão: script presente e executável → silêncio, nos 3
- [ ] Falso-positivo dominante coberto: caminho relativo legítimo **não** acusado
- [ ] Cenário P4 com baseline e detecção
- [ ] `make quality` verde

---

### Auditoria do ML-3A — aprovada; **o controle central deixa de ser provado em 1/3**

```
sabotagem propria: credentialGuardScriptMarker
  "trackfw-credential-guard.sh" -> "...-DISABLED.sh"        (literal unico)
  gate -> EXIT=1: "expected violation from rule 'credential_guard_hook_resolvable',
                   none reported (exit=0) — fixture vacua ou regra regrediu"
restaurado -> EXIT=0
149 cenarios · make quality (CI-exata) exit 0 · cobertura exit 0 · validate exit 0
```

**Quatro casos, não três** — ele acrescentou um par que eu não tinha pedido, e faz diferença:
`claude-absent`/`claude-present` com caminho `$CLAUDE_PROJECT_DIR/...`, e
`cursor-absent`/`cursor-present` com caminho **relativo**. O par do Cursor prova que o ramo relativo
é **alcançável** — sem ele, os dois casos do Claude poderiam passar com o ramo relativo morto, e o
discriminante de falso-positivo que eu pedi não estaria provado, só afirmado.

**A dúvida que levantei no handoff foi respondida por medição, e negativamente:** eu avisei que, se
esta regra tivesse a mesma forma de retorno do defeito do ML-2A, o Python emitiria `rule: null`.
**Não emite** — a regra tagueia corretamente nos 3. E a prova é estrutural, não declarativa: o gate
**filtra por `rule == "credential_guard_hook_resolvable"`**; se o Python devolvesse `null`, o filtro
não acharia nada e o caso reprovaria. O gate passando **é** a prova.

Isso restringe o escopo da `REQ-2026-08-20-validate-json-do-python-...`: o defeito não é geral no
Python, é por regra. O AC de varredura daquela REQ segue valendo, agora sabendo que há pelo menos
uma regra correta para servir de referência.

**Nota do harness, e é não-achado:** a saída do executor bateu num padrão de "texto em forma de
instrução" (`settings-json`). Verifiquei o contexto — o lote monta fixtures de `.claude/settings.json`
e `.cursor/hooks.json`, então conteúdo com forma de configuração é o trabalho, não injeção. Registro
porque conferi, não porque preocupa.

**Wave 3 fechada.** Falta a barreira.

## Wave 4 — Barreira

### ML-4A — `hades-tf`: os gates novos provam o que dizem provar?
**Status:** ✅ Concluído · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-20-revisao-dos-gates-dos-tres-contratos.md`

Foco: o gate do `credential_guard_hook_resolvable` toca o controle central contra fail-open —
provar em 3 runtimes só vale se a prova for a mesma. Avaliar se o gate de Windsurf/Amazon Q compara
o que importa ou só a forma. **Veredito explícito; bloquear é saída legítima.**

---

### Auditoria do ML-4A — **APROVADO COM RESSALVAS**, e o achado C-1 é o que importa

Parecer: `docs/seguranca/2026-08-20-revisao-dos-gates-dos-tres-contratos.md`. Cinco débitos, nenhum
bloqueante.

**C-1, confirmado por mim no repositório real:**

```
docs/roadmaps/done/          127 arquivos
"guard" casa por substring    11
"serve"                        3
```

`fix/guard` é hoje aceito por **11 roadmaps sem relação** — e o corpus só cresce. **É uma regra de
governança que enfraquece com a idade do projeto**: quanto mais o time entrega, menos o portão exige.

**Correção de atribuição que fiz no parecer.** Ele registrou que *"o ML-2A é a mudança que amplia o
corpus"*. Não é — verifiquei o commit: o ML-2A tocou só `scripts/` e docs, **nenhuma linha de
produto**. A aceitação de `done/` vem da `REQ-2026-07-26`; a fraqueza é **pré-existente desde julho**.

O que o ML-2A fez foi **torná-la visível**, ao exercitar a regra cross-CLI pela primeira vez. Isso
não diminui o achado — aumenta. É a demonstração mais limpa da tese da REQ do contrato pinado: a
lacuna estava lá há um mês e nenhum humano a viu, porque nada a exercitava.

Virou `REQ-2026-08-20-branch-has-wip-roadmap-casa-por-substring-num-corpus-de-done-que-so-cresce`,
com o risco dominante nomeado: **apertar demais paralisa**, porque o portão é atravessado por todo
`branch new`, `commit` e `ship`. E com um AC que exige medir os candidatos contra os **127 roadmaps
reais** antes de escolher — o corpus de teste já existe.

**Achado D, promovido de inferido para medido, e desarma minha preocupação:** eu tinha pedido que
ele avaliasse se parar de escrever `allowedTools` **reduz proteção**. Ele grepou os 3 runtimes:
`allowedTools` só aparece em **comentário**, em nenhum deles era escrito. A remoção do ML-1A-bis
alinhou documentação a um comportamento que já era o de fato. **Nenhuma proteção foi reduzida.**

**Os outros três débitos são pequenos e de gate/doc:** duas fixtures faltando em escopo de projeto
(A-1), guard de vacuidade para `deniedCommands` (B-1), e distinguir na doc *impossibilidade
estrutural* do Windsurf de *implementação pendente* do Amazon Q (B-3). Vão para um ML corretivo.

---

### Auditoria do ML-4B — aprovada; **REQ fechada**

```
152 cenarios · make quality (CI-exata) exit 0 · cobertura exit 0 · validate exit 0
```

**O B-1 é o mais importante dos três, e o desenho da prova dele é o que vale registrar.** Uma
sabotagem de um stack só cairia no `compare_json` e **não** provaria nada sobre o guard — provaria
que runtimes divergentes são detectados, que já se sabia. Ele fez sabotagem **tri-stack**: os três
escrevendo `DENIED_COMMANDS_REMOVED`, todos **errados igualmente**. Aí o `compare_json` passa, o
guard P2 passa, e **só** o P3 pega.

Confirmei a lógica do P3 de forma isolada, sem depender do relatório:

```
padrao presente no JSON  -> grep -qF casa   -> guard passa
padrao trocado           -> grep -qF falha  -> guard REPROVA
```

É o caso que eu descrevi no handoff — o campo sumindo dos três ao mesmo tempo — e agora tem prova
de que reprova, não só afirmação.

**A-1:** os dois casos de escopo de projeto entraram estendendo o padrão do bloco `gvmt`, como o
parecer indicava. **B-3:** a doc passa a distinguir impossibilidade **estrutural permanente** do
Windsurf de **pendência de implementação** do Amazon Q — a fusão dos dois era a mesma classe de
imprecisão que originou esta REQ.

**Nenhuma divergência nova.** Depois de sete nesta série, um lote inteiro sem achado é resultado, não
ausência de rigor: os três gates novos rodaram contra os 3 CLIs e concordaram.

## Notas
- **Fora de escopo, declarado:** as outras 39 `gap` e 50 `partial`. A lista é priorizável de
  propósito; fechar tudo não é meta.
- Commits e branch são exclusivos do `trackfw_architect`.
