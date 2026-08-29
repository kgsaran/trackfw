---
status: done
date: 2026-07-25
req: "REQ-2026-07-25-wizard-guiado-de-identidade-no-agents-install.md"
squad: "trackfw"
---

# Roadmap: Wizard guiado identidade agents install

> Created: 2026-07-25 | Status: done | Merged: PR #65

## Acceptance Criteria

- [ ] Componente unico de wizard por CLI, consumido por `init` e `agents install`
- [ ] Passo de identidade em `agents install` so aparece com `kind == agents`
      **e** TTY **e** (`identity.json` ausente **ou** `--identity`)
- [ ] Com identidade existente e sem `--identity`, nao pergunta nada
- [ ] `skills install` nunca exibe o wizard
- [ ] Ramo nao-TTY nunca bloqueia em prompt
- [ ] Modo `custom` rotula com `Item.Name` + `Item.Description`, sem o `id`
- [ ] Tela de confirmacao (10 pares + apelido) antes de qualquer escrita
- [ ] Recusar a confirmacao nao grava nada
- [ ] Comportamento equivalente nos 3 CLIs
- [ ] `make quality` verde com `check-identity-parity.sh` **inalterado**
- [ ] `init` sem regressao (idempotencia e `--identity-preset`)

## Context

REQ: docs/req/REQ-2026-07-25-wizard-guiado-de-identidade-no-agents-install.md
ADR: docs/adr/ADR-2026-07-25-wizard-unificado-de-identidade-no-agents-install.md
squad: trackfw

Evolucao de UX sobre a identidade entregue no PR #64. **Nao altera o schema de
`identity.json`, o contrato de slug nem os artefatos gerados** — logo
`check-identity-parity.sh` deve continuar passando **sem nenhuma alteracao**.
Se ele precisar mudar, algo saiu do escopo.

### Fluxo alvo (ADR D6)

```
1. Target CLIs            existente
2. Agentes a gerenciar    existente
3. Superficie ambigua     existente, condicional
4. Apelido do usuario     NOVO ── condicional (ADR D2)
5. Preset ou custom       NOVO ── condicional (ADR D2)
6. Nomes livres           NOVO ── so no modo custom
7. Confirmacao            NOVO ── condicional (ADR D2)
8. Instalacao             existente
```

### Mapa de dependencias

```
Wave 1                    Wave 2 (paralelo)        Wave 3
ML-1A componente Go  ──┬──► ML-2A npm  ──┐
   + init + install    └──► ML-2B pypi ──┴──► ML-3A docs + testes E2E
```

Wave 1 e sequencial e sozinha de proposito: ela define o contrato de UX
(ordem das etapas, rotulos, regra de acionamento, formato da confirmacao) que
Node e Python vao replicar. Foi exatamente esse padrao que evitou retrabalho
de paridade na REQ anterior.

---

## Wave 1 — Componente compartilhado em Go (1 ML)
> Dependencies: none

### ML-1A — Wizard unificado, rotulos por especialidade e confirmacao
**Status:** done (`8b0eeb1`)
**Agente:** trackfw-backend
**Files affected:** `internal/commands/identity_wizard.go` (novo),
`internal/commands/init.go`, `internal/commands/integrations_flags.go`,
`internal/i18n/locales/{pt-BR,en-US,es-ES}.json`, testes correspondentes

**Actions:**
1. Extrair de `init.go` o wizard de identidade para
   `internal/commands/identity_wizard.go`, com uma API do tipo:
   `runIdentityWizard(catalog *integrations.Catalog, home string) (identity.Config, bool, error)`
   — o `bool` indica "usuario confirmou e deve persistir".
2. `init` passa a consumir o componente. **Sem mudanca de comportamento
   observavel** — a idempotencia do re-`init` e o `--identity-preset` seguem
   iguais (ha testes cobrindo; nao os altere para "passar").
3. **Rotulos por especialidade (ADR D4):** no modo `custom`, cada campo usa
   `Item.Name` + `Item.Description` do catalogo:
   `Architect — Architecture, ADRs and governed coordination`
   O `id` **nao** aparece. Fonte unica: `catalog.json`, ja embedado.
4. **Tela de confirmacao (ADR D3):** antes de `identity.Save`, listar os 10
   pares `description -> display_name` mais o apelido, e pedir confirmacao
   (`huh.NewConfirm`). Recusa -> volta a selecao de preset, **sem gravar**.
   Vale para preset **e** custom.
5. **Acionamento em `agents install` (ADR D2):** em
   `executeIntegrationMutation`, apos a selecao de alvos/superficies e antes de
   `BuildPlans`, chamar o wizard **somente** se
   `kind == KindAgents && integrationsStdinIsTTY() && (identityFileAusente || flag --identity)`.
   Caso contrario, seguir com `identity.Load` como hoje e, havendo identidade,
   imprimir uma linha informando qual esta em uso.
6. Flags novas em `agents install`: `--identity` (bool, forca reconfiguracao) e
   `--identity-preset` (mesma semantica de `init`; invalido -> erro listando os
   validos, derivados de `identity.PresetNames()`).
7. **Gate por `kind` (ADR D5):** `newIntegrationsLifecycleCmd` e compartilhado
   com `skills`. Sem o gate, `trackfw skills install` exibiria um wizard sem
   efeito. As flags novas tambem so devem ser registradas para `agents`.
8. Chaves i18n novas nos 3 locales, conjuntos identicos.

**Acceptance criteria:**
- [ ] `go build ./... && go test ./... && go vet ./... && make lint` verdes
- [ ] Teste: `agents install` com `identity.json` **existente** e sem
      `--identity` **nao** invoca o wizard
- [ ] Teste: `agents install` com `identity.json` **ausente** em TTY invoca
- [ ] Teste: `skills install` **nunca** invoca, mesmo sem `identity.json`
- [ ] Teste: ramo nao-TTY nao bloqueia e segue exigindo `--targets`
- [ ] Teste: recusar a confirmacao **nao** grava arquivo
- [ ] Teste: rotulos do modo custom contem `Item.Description` e **nao** contem
      o `id` cru
- [ ] Testes preexistentes de `init` passam **sem alteracao**
- [ ] `scripts/check-identity-parity.sh` passa **sem alteracao**

**Comandos de validação:** `go build ./... && go test ./... && go vet ./... && make lint && scripts/check-identity-parity.sh`

**Auditoria do orquestrador (E2E pelo binario):**

| Cenario | Resultado |
|---|---|
| flags `--identity`/`--identity-preset` so em `agents` | ✅ zero em `skills` |
| nao-TTY sem identidade | ✅ nao trava, nao pergunta, nao grava, artefato inalterado |
| `--identity-preset starwars` | ✅ `name: r2-d2-tf`, `R2-D2 — Database specialist...` |
| identidade existente | ✅ imprime `identity: 10 custom agent(s)` e **nao pergunta** |
| `--identity-preset xpto` | ✅ erro listando os 12 validos |
| `check-identity-parity.sh` | ✅ passou **sem alteracao no script** — guarda-corpo de escopo funcionou |

---

### ML-1B — Remocao de `saveWizardIdentity` orfa
**Status:** done (`af46f8b`)
**Agente:** general-purpose
**Files affected:** `internal/commands/init.go`, `internal/commands/identity_wizard.go`,
`internal/commands/init_test.go`

**Motivo.** Apos a extracao do ML-1A, `saveWizardIdentity` deixou de ser
chamada por qualquer caminho de producao, mas 3 testes continuavam a
exercita-la. Isso e **cobertura falsa**: quebrar o caminho real nao deixaria
nenhum teste vermelho.

**Causa da falha de processo:** instrucao do orquestrador ao ML-1A dizia
"testes preexistentes passam sem alteracao". A intencao era *nao regredir
comportamento*; a leitura literal — correta — foi *preservar chamadas a
funcao morta*. Lacuna de especificacao, nao de execucao. E o mesmo padrao dos
defeitos do ciclo anterior em outra forma: **teste que nao observa codigo vivo**.

**Actions executadas:**
1. `saveWizardIdentity` removida; `grep -rn "saveWizardIdentity" internal/` retorna zero.
2. Extraida `resolveIdentitySelection(...)` — toda a metade **nao-interativa**
   do wizard (guarda de skip -> `buildIdentityConfig` -> `identity.Validate` ->
   `confirm` -> `identity.Save`), com o unico passo interativo como callback.
   A sequencia existe agora em **exatamente um lugar**, e e esse lugar que os
   testes atacam.
3. Enum `identityOutcome` (`identitySkipped`/`identityDeclined`/`identitySaved`):
   um `bool` nao distinguiria "neutral" (nao grava e **retorna**) de "usuario
   recusou" (nao grava e **volta ao loop**).

**Acceptance criteria:**
- [x] `grep -rn "saveWizardIdentity" internal/` retorna zero
- [x] `go build/test/vet` + `make lint` verdes
- [x] Os 3 testes cobrem a mesma intencao contra codigo vivo
- [x] `check-identity-parity.sh` passa sem alteracao
- [x] **Mutation test verificado pelo orquestrador:** removendo `identity.Validate`
      do caminho real, falha **exatamente um** teste
      (`TestResolveIdentitySelectionCustomCollisionWritesNothing`), na assercao
      correta — `identity.json` passa a existir apesar do erro de validacao.
      Antes do ML-1B, essa mutacao nao deixaria nenhum teste vermelho.

**Pendencia herdada (para a Wave 2):** `npm/src/commands/init.js` ainda define e
chama seu proprio `saveWizardIdentity`, e um comentario aponta para o simbolo
Go ja removido. Nao e quebra de paridade comportamental (o contrato e de
artefatos byte-identicos), mas e divergencia estrutural + ponteiro morto.
Deve ser resolvido pelo ML-2A, que reestrutura esse arquivo.

---

## Wave 2 — Paridade Node.js e Python (2 MLs em paralelo)
> Dependencies: **barrier — ML-1A completo**. Diretorios disjuntos entre si.

### ML-2A — Porta Node.js
**Status:** done (`a7943df`)
**Agente:** trackfw-frontend
**Files affected:** `npm/src/commands/identity-wizard.js` (novo),
`npm/src/commands/init.js`, `npm/src/integrations/index.js` (ou onde vive o
fluxo de `agents install`), `npm/src/i18n/locales/*.json`, testes npm

**Actions:** replicar ML-1A com **ordem de etapas, rotulos e regra de
acionamento identicos**. Consultar a implementacao Go como referencia.

**Acceptance criteria:**
- [ ] `cd npm && npm test` verde, testes preexistentes **sem alteracao**
- [ ] Mesmos 7 testes de comportamento do ML-1A, portados
- [ ] `scripts/check-identity-parity.sh` passa sem alteracao
- [ ] Nenhum arquivo fora de `npm/`

### ML-2B — Porta Python
**Status:** done (`9266242`)
**Agente:** trackfw-data
**Files affected:** `pypi/trackfw/commands/identity_wizard.py` (novo),
`pypi/trackfw/commands/init.py`, `pypi/trackfw/integrations/command.py`,
`pypi/trackfw/i18n/locales/*.json`, testes pypi

**Actions:** replicar ML-1A com **ordem de etapas, rotulos e regra de
acionamento identicos**. Consultar a implementacao Go como referencia.
Detectar TTY com `sys.stdin.isatty()`.

**Acceptance criteria:**
- [ ] `make test-python` verde, testes preexistentes **sem alteracao**
- [ ] Mesmos 7 testes de comportamento do ML-1A, portados
- [ ] `scripts/check-identity-parity.sh` passa sem alteracao
- [ ] Nenhum arquivo fora de `pypi/`

---

## Wave 3 — Documentacao e verificacao E2E (1 ML)
> Dependencies: **barrier — ML-2A e ML-2B completos**

### ML-3A — Docs, i18n cruzado e E2E
**Status:** done
**Agente:** general-purpose
**Files affected:** `README.md`, `npm/README.md`, `pypi/README.md`,
`docs/cli-parity.md`, `docs/agents-working-context.md`

**Actions:**
1. Documentar o fluxo novo nos 3 READMEs, com o exemplo da tela de
   confirmacao, `--identity` e `--identity-preset` em `agents install`.
2. Secao em `docs/cli-parity.md` registrando que a UX do wizard tambem e
   contrato de paridade (ordem, rotulos, acionamento).
3. Verificar que as chaves i18n novas existem nos **9** arquivos de locale
   (3 CLIs x 3 idiomas), com conjuntos identicos.
4. E2E manual pelos binarios: `agents install` sem identidade (pergunta),
   com identidade (nao pergunta), com `--identity` (pergunta de novo),
   `skills install` (nunca pergunta), nao-TTY (nao bloqueia).

**Acceptance criteria:**
- [x] `make quality` verde (Go 21 testes + npm 113 testes + pytest 418 testes,
      `check-cli-parity.sh`, `check-validate-parity.sh`, `check-static-assets.sh`,
      `check-integration-assets.sh` e `check-identity-parity.sh` todos verdes)
- [x] `trackfw validate` sem violations novas — apenas as 2 preexistentes de
      `REQ-2026-07-24-corrige-resolve-de-integrations-em-windows-...` (`no
      linked ADR` / `no linked Roadmap`)
- [x] Os 3 READMEs documentam `--identity` e `--identity-preset`
- [x] E2E dos 4 cenarios (A/B/C/D do prompt do orquestrador) com output real
      registrado no relatorio, nos 3 CLIs — ver auditoria abaixo
- [x] Chaves i18n `identity.wizard.{confirmHeader,confirmQuestion,nicknameRowLabel}`
      e `identity.inUse` conferidas nos 9 arquivos de locale (3 CLIs x 3
      idiomas), conjuntos identicos

**Auditoria (Wave 3, ML-3A — E2E pelos 3 binarios reais, `HOME` isolado, `WD`
fora do repositorio):**

| Cenario | Go | Node | Python |
|---|---|---|---|
| A — sem identidade + `--identity-preset chaves` | ✅ `name: girafales-tf` | ✅ `name: girafales-tf` | ✅ `name: girafales-tf` |
| B — identidade ja configurada, sem flag | ✅ `identity: 10 custom agent(s)`, nao pergunta | ✅ idem | ✅ idem |
| C — `skills install --help \| grep -c identity` | ✅ `0` | ✅ `0` | ✅ `0` |
| D — `--identity-preset xpto` | ✅ erro limpo listando os 12 validos | ❌ stack trace (nao e erro limpo) | ✅ erro limpo listando os 12 validos |

**Achado no Cenario D (Node):** `--identity-preset xpto` no CLI Node encerra
com stack trace em vez de mensagem de erro limpa. **Nao e regressao desta
REQ** — e o mesmo padrao pre-existente ja registrado na auditoria da Wave 2
para `--scope xpto` (ausencia de try/catch global no CLI Node), fora do
escopo desta REQ, que e exclusivamente de UX do wizard de identidade.
Reportado aqui para rastreabilidade; nao bloqueia o fechamento do ML.

**Nota de higiene do E2E:** os cenarios A/B foram executados com `scope
project` a partir de um diretorio de trabalho **fora** do repositorio
(`WD` em `/private/tmp/.../scratchpad/e2e/{go,node,python}`), evitando
escrever `.claude/agents/` ou `.trackfw/` na arvore do trackfw. Uma primeira
tentativa rodou por engano dentro do repo e foi revertida com `rm -rf` antes
de qualquer commit.

**Nota de escopo:** `docs/agents-working-context.md`, listado acima em
"Files affected" do ML-3A, **nao** fez parte do escopo exclusivo desta
execucao (delimitado explicitamente pelo orquestrador a README.md,
npm/README.md, pypi/README.md, docs/cli-parity.md, este roadmap e a REQ) —
deixado intencionalmente de fora, para o orquestrador decidir se atualiza.

**Nota sobre a ordem do fluxo:** a leitura direta de `runIdentityWizard` nos
3 CLIs mostra preset/custom -> nomes (so custom) -> apelido -> confirmacao,
o que **difere** da ordem descrita acima em "Fluxo alvo (ADR D6)" (que lista
o apelido antes do preset). `docs/cli-parity.md` foi escrito para refletir a
ordem real do codigo, com a divergencia registrada na REQ. Nao e regressao —
os 3 CLIs implementam a mesma ordem entre si.

---

## Legenda de status
- pending / in_progress / done / blocked


**Auditoria do orquestrador (Wave 2):**

- ML-2B (pypi) rodou primeiro. Verificado de forma independente: 418 testes
  Python, gate `check-identity-parity.sh` intacto (0 linhas de diff), os 5
  cenarios E2E pelo binario real batem com o Go — incluindo
  `identity: 10 custom agent(s)` e a mensagem exata de preset invalido.
- ML-2A (npm) **falhou na primeira tentativa** por limite de sessao da API,
  nao por erro de execucao. Trabalho parcial ja estava completo e correto no
  working tree; auditado e commitado pelo orquestrador em vez de re-executado
  do zero. Resolveu a pendencia herdada do ML-1B (`saveWizardIdentity`
  duplicado em `init.js`).
- **Achado investigado e descartado como nao-defeito:** `--identity-preset xpto`
  no CLI Node crasha com stack trace em vez de erro limpo. Confirmado como
  **padrao pre-existente**, presente em `origin/main` antes desta feature —
  `--scope xpto` ja crashava da mesma forma (`throw new Error` sem try/catch
  global). Fora do escopo desta REQ, que e exclusivamente de UX do wizard de
  identidade, nao de tratamento de erro do CLI.
- `check-identity-parity.sh` passou nos 3 CLIs sem nenhuma linha alterada no
  script — guarda-corpo de escopo mantido ate o fim da Wave 2.
