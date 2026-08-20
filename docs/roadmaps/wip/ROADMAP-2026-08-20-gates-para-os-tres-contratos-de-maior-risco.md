---
status: wip
date: 2026-08-20
req: "docs/req/REQ-2026-08-20-tres-contratos-afirmados-no-cli-parity-sem-gate-cross-cli.md"
adr: ""
squad: "apolo-tf, hades-tf"
---

# Roadmap: gates para os três contratos de maior risco

> Created: 2026-08-20 | Status: wip

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
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` · **Dependência:** ML-1A
**Critérios de aceite:**
- [ ] Windsurf e Amazon Q cobertos, nos 3 CLIs, comparando saídas/artefatos reais
- [ ] Cenário P4 com baseline e detecção
- [ ] A anotação da seção deixa de afirmar cobertura inexistente
- [ ] `make quality` verde

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


## Wave 2 — `branch_has_wip_roadmap` com `done/`

### ML-2A — Cenário cross-CLI com roadmap em `done/`
**Status:** ⬜ Pendente · **Agente:** `apolo-tf`
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

---

## Wave 3 — `credential_guard_hook_resolvable` cross-CLI

### ML-3A — Estender a prova para Node e Python
**Status:** ⬜ Pendente · **Agente:** `apolo-tf`

O Cenário 47 declara no próprio comentário ser prova black-box da regra **Go**. É o controle que o
`ADR-2026-08-12` aponta como o que resta mitigando o fail-open — com prova em um terço dos runtimes.

**Critérios de aceite:**
- [ ] Regra exercitada nos 3 CLIs, com hook registrado apontando para script ausente
- [ ] Não-regressão: script presente e executável → silêncio, nos 3
- [ ] Falso-positivo dominante coberto: caminho relativo legítimo **não** acusado
- [ ] Cenário P4 com baseline e detecção
- [ ] `make quality` verde

---

## Wave 4 — Barreira

### ML-4A — `hades-tf`: os gates novos provam o que dizem provar?
**Status:** ⬜ Pendente · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-20-revisao-dos-gates-dos-tres-contratos.md`

Foco: o gate do `credential_guard_hook_resolvable` toca o controle central contra fail-open —
provar em 3 runtimes só vale se a prova for a mesma. Avaliar se o gate de Windsurf/Amazon Q compara
o que importa ou só a forma. **Veredito explícito; bloquear é saída legítima.**

---

## Notas
- **Fora de escopo, declarado:** as outras 39 `gap` e 50 `partial`. A lista é priorizável de
  propósito; fechar tudo não é meta.
- Commits e branch são exclusivos do `trackfw_architect`.
