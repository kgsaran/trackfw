---
status: done
date: 2026-08-02
req: "docs/req/REQ-2026-08-02-substituir-os-parsers-artesanais-de-config-por-biblioteca-yaml-nos-tres-clis.md"
squad: ""
---

# Roadmap: Substituir os parsers artesanais de config por biblioteca YAML nos tres CLIs

> Created: 2026-08-02 | Status: done

## Context

REQ: docs/req/REQ-2026-08-02-substituir-os-parsers-artesanais-de-config-por-biblioteca-yaml-nos-tres-clis.md
ADR: docs/adr/ADR-2026-08-02-parsing-de-config-por-biblioteca-yaml-com-normalizacao-para-string-na-fronteira.md

Quatro defeitos silenciosos em dois dias saíram dos parsers artesanais (~1085 linhas somadas).
Cada um foi corrigido pontualmente, mas o parser segue sendo **subconjunto** de YAML. KG
determinou que não se pode seguir sabendo do defeito.

### A medição que redefine o trabalho

Adotar bibliotecas **sem mais nada** troca a divergência artesanal por divergência de **schema**:

| Entrada | Go `yaml.v3` | Python `PyYAML` |
|---|---|---|
| `yes` | `"yes"` string | **`True` bool** |
| `010` | **`8` int** | **`8` int** |
| `2026-08-02` | **`time.Time`** | **`datetime.date`** |

`lenient_until` é `string // date string YYYY-MM-DD` → quebra no dia 1.
`wip_limit: 010` → viraria 8.

**A decisão central é a normalização para string na fronteira**, não a adoção da biblioteca.

### Node não foi medido

Sem rede para instalar a lib. Está declarado como lacuna, não presumido — é o **AC4**, e o ML-1A
deve medir antes de escolher entre `js-yaml` e `yaml`.

### Estrutura

**Executor único nos 3 CLIs.** No ciclo anterior foi o primeiro trabalho multi-CLI sem divergência
nem ML de reconciliação. Aqui o alvo é mais difícil: semântica idêntica entre três bibliotecas
**diferentes**, com schemas diferentes.

**O ML-0 existe por causa do AC4.** Escolher a biblioteca do Node depende de medição que ainda não
foi feita; fazer isso dentro do ML de implementação misturaria decisão com execução.

## Critérios de Aceite

- [ ] Os 3 carregam com biblioteca YAML; escalares chegam aos consumidores como **string**
- [ ] Fidelidade textual: `2026-08-02` volta `"2026-08-02"`; `010` volta `"010"` → 10, não 8
- [ ] `yes` idêntico nos 3 (hoje Go e Python divergem)
- [ ] As ~20 chaves seguem funcionando — teste **por chave**
- [ ] Nada dos 4 ciclos anteriores regride
- [ ] Formas antes não suportadas (mapa inline, lista aninhada, âncora) passam a funcionar
- [ ] `validate` e `status` byte-idênticos nos 3
- [ ] Falsificação com fixture contendo `yes`, octal e data nua
- [ ] `make build`, `make test`, `make lint`, `make parity`, `make quality` verdes

---

## Wave 0 — Medir o Node e escolher a biblioteca (1 ML)
> Dependências: nenhuma

### ML-0A — Medição de schema do Node
**Status:** ✅ concluído (auditado 2026-08-02)
**Agente:** Apolo
**Arquivos afetados:** nenhum de produto — é investigação

**Ações:** instalar `js-yaml` e `yaml` num diretório temporário e rodar a tabela de tipos
(`yes`, `no`, `on`, `010`, `1.0`, `null`, `~`, `2026-08-02`, `true`) em ambas. Comparar com as
medições de Go e Python já registradas no ADR.

**Acceptance criteria:**
- [x] Tabela executada nas duas bibliotecas (`js-yaml` 5.2.3, `yaml` 2.9.0)
- [x] Recomendação: **`yaml` 2.x**, com justificativa
- [x] Normalização resolve nas duas — critério de parada não acionado
- [x] `git status --porcelain` vazio

**Três achados que a tabela do ADR não previa:**

1. **Octal é divisão de TRÊS vias.** Go e Python → `8`; **as duas libs Node → `10`**.
   Nenhum par concorda por padrão.
2. **Node não converte data** — `2026-08-02` volta string. Logo **um teste de `lenient_until`
   passaria no Node sem normalização alguma**. É o **octal** que discrimina o Node, não a data.
   Isso vai direto para o AC11: a fixture precisa do octal, senão é vacuosa no Node.
3. **Âncoras corrompem normalização ingênua.** Em `yaml` 2.x, `b: *x` produz um `Alias`, cujo
   `.source` é o **nome da âncora**, não o valor.

**Escolha da biblioteca:** `yaml` 2.x sobre `js-yaml` — `Scalar.source` é API pública e
documentada (em `js-yaml` seria `parseEvents`+`eventsToAst`, não documentado, que já falhou na
primeira tentativa dele), e `yaml` tem zero deps de runtime enquanto `js-yaml` arrasta `argparse`.

---

## Wave 1 — Implementação (1 ML, executor único)
> Dependências: **ML-0A concluído**

### ML-1A — Biblioteca + normalização nos três CLIs
**Status:** ✅ concluído (auditado 2026-08-02)
**Agente:** Apolo (executor **único**)
**Arquivos afetados:** `internal/config/config.go`, `npm/src/config/index.js`,
`pypi/trackfw/config.py`, manifestos (`go.mod`, `npm/package.json`, `pypi/pyproject.toml`) + testes

**Riscos confirmados pelo ML-0A, além do AC3:**
- **Âncoras/aliases:** `b: *x` vira `Alias`, não `Scalar`; `.source` devolve o nome da âncora.
  Resolver o alias antes de ler, ou o campo sai corrompido. A lib aceita em silêncio.
- **Octal diverge em três direções** — reforça que a normalização deve ler o **nó bruto**, não
  reverter o valor tipado.

**Ponto de maior risco — AC3, fidelidade textual.** Ler com a biblioteca e depois "des-tipar" não
é trivial: `time.Time` de volta a `2026-08-02` e `8` de volta a `010` são **irreversíveis** depois
da coerção. Se a biblioteca perder a forma original, a normalização precisa acontecer **antes** da
coerção — lendo o nó bruto em vez do valor tipado. É decisão de implementação, mas o resultado é
contrato.

**Acceptance criteria:**
- [ ] Tabela de fidelidade do AC3 reproduzida nos 3, saída lado a lado
- [ ] Teste **por chave** (~20 chaves)
- [ ] Regressão dos 4 ciclos anteriores coberta
- [ ] Mapa inline, lista aninhada e âncora funcionam
- [ ] Config ausente/vazio cai nos defaults sem erro
- [ ] Contrato de `ProjectConfig` e equivalentes **inalterado**
- [ ] **Não** tocar no parser de frontmatter
- [ ] Suítes verdes nos 3

---

## Wave 2 — Barreira (1 ML)
> Dependências: **Wave 1 completa**

### ML-2A — Paridade e seam de schema
**Status:** ✅ concluído (auditado 2026-08-02)
**Agente:** Ártemis

**Ações:**
1. Gates de paridade passam; `make quality` exit 0; `validate` e `status` byte-idênticos nos 3.
2. Confirmar que os **82** cenários existentes seguem passando. **Rodar, não presumir** — a
   troca do parser é a mudança mais ampla da série, e há cenários que escrevem `trackfw.yaml`.
3. **Cenário novo com fixture que discrimina schema:** precisa conter `yes`, valor com cara de
   octal e data nua. Fixture de strings simples passa sob qualquer schema e não prova nada.
4. Braço de detecção determinístico, isolando o que está sob teste.
5. Contador e linha final atualizados.

**Acceptance criteria:**
- [ ] Gates passam; `make quality` exit 0
- [ ] 82 cenários herdados confirmados; qualquer um quebrado pela troca, reparado
- [ ] Cenário novo com fixture discriminante de schema; não vacuoso
- [ ] Contador atualizado
- [ ] `git status --porcelain` sem resíduo


---

## MLs corretivos, todos vindos de auditoria

### ML-1B — Config malformada falhava em SILÊNCIO (regressão nossa)
**Status:** ✅ concluído (auditado 2026-08-02)

O ML-1A introduziu parser **all-or-nothing**: uma vírgula errada descartava a config inteira sem
avisar. **Medido contra a `main`**, é regressão, não preservação:

```
trackfw.yaml:  agents: [zeus, apolo      ← colchete não fechado
               wip_limit: 3

parser antigo (linha a linha) → wip_limit = 3   (leu a linha válida)
parser novo (ML-1A)           → wip_limit = 1   (descartou tudo, exit 0, sem mensagem)
```

Virou a **pior** instância da classe que o ciclo existe para eliminar. Agora falha alto:
mensagem e exit **idênticos** nos 3, verificado byte a byte.

Ele ainda fechou **três divergências** que só apareceram ao auditar o caminho de erro:
chaves duplicadas (só o Node rejeitava), multi-documento (só o Go aceitava, lendo o primeiro em
silêncio) e alias referenciado antes da âncora (só o Node aceitava, virando string vazia).

### ML-3A — O `validate` contornava a biblioteca
**Status:** ✅ concluído (auditado 2026-08-02)

A barreira descobriu que o objetivo do ciclo estava **metade cumprido**: a config passava por
biblioteca YAML, mas o `validate` — o caminho que mais importa — relia `trackfw.yaml` com
leitores artesanais próprios.

```
wip_limit: "3"      config.Load() → 3        validate → limit: 1
```

`readWIPConfig` e `readGovernanceMode` eliminados nos 3; nenhum ponto do validator lê o arquivo
diretamente. Correção majoritariamente por **deleção**.

**Ele parou onde devia:** `update.go`, `sync/linear.go` e `sync/jira.go` também leem
`trackfw.yaml`, mas para campos que **não existem** em `ProjectConfig` (`hooks`, `ci`,
`linear_api_key`, `jira_base_url`). Corrigi-los exigiria ampliar o contrato — decisão de ADR.
Reportou em vez de decidir. **Fica na fila.**

### ML-3B — A regressão não tinha teste
**Status:** ✅ concluído (auditado 2026-08-02)

O executor do ML-3A sinalizou honestamente: nenhum teste fixava o bug. O existente usa
`wip_limit` **sem aspas**, que passava antes e depois.

A fixture precisa do **escalar citado** para discriminar:

| `trackfw.yaml` | binário correto | com leitor artesanal de volta |
|---|---|---|
| `wip_limit: "3"` | `limit: 3` | `limit: 1` ← **discrimina** |
| `wip_limit: 3` | `limit: 3` | `limit: 3` ← **vácuo** |

Contador **88 → 92**.

---

## Fechamento

Concluído e auditado em 2026-08-02. `make quality` exit 0; falsificação **82 → 92**.

**O que muda:** ~1085 linhas de parser artesanal saem; entram `yaml.v3` (Go), `yaml` 2.x (Node) e
`PyYAML` (Python — **primeira dependência de runtime** do pacote). Qualquer YAML válido passa a
ser aceito no config, e o que não parseia **falha alto** em vez de sumir.

**A lição do ciclo:** a decisão que importava não era "adotar biblioteca" — era **normalizar para
string na fronteira**. As três bibliotecas divergem em três direções (`yes`, octal, data), e
adotá-las sem normalizar teria trocado a divergência artesanal por divergência de schema. Medir
**antes** de escrever código foi o que revelou isso.

**Fica na fila:** `update.go`, `sync/linear.go` e `sync/jira.go` ainda leem `trackfw.yaml`
diretamente, para campos fora do `ProjectConfig`. Ampliar o contrato é decisão de ADR.
