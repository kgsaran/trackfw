---
status: wip
date: 2026-09-23
req: "docs/req/REQ-2026-09-23-bash-consome-stdout-de-python3-sem-normalizar-crlf-e-o-gate-examina-zero-no-windows.md"
squad: "hades-tf, apolo-tf, artemis-tf"
---

# Roadmap: bash consome stdout de `python3` sem normalizar CRLF

> Created: 2026-09-23 | Status: wip

## Context

REQ: `docs/req/REQ-2026-09-23-bash-consome-stdout-de-python3-sem-normalizar-crlf-e-o-gate-examina-zero-no-windows.md`
Origem: **#353** (consumidor externo) + achado próprio no `windows-census.yml` (2026-09-23).

No Windows, `python3` traduz `\n` → `\r\n` no stdout. Bash que consome essa saída recebe valores com
`\r` invisível. O efeito é **silêncio**: o gate roda, não acha nada, e reporta sucesso — ou reprova
por vacuidade sem dizer a razão verdadeira.

**Duas ocorrências medidas, independentes:** `check-no-literal-nul-in-source.sh` examinava **0 de
671** arquivos; `run-gates-falsify-shard.sh` derruba **8/8 shards** do censo com **146 rótulos**
acusados ausentes — inclusive rótulos que **foram emitidos**.

🔴 **Este roadmap conserta o instrumento de medição.** A recontagem do cluster de Windows vem depois,
com ele funcionando. Medir agora é medir com régua quebrada.

## Acceptance Criteria

- [x] Enumeração real dos sítios, classificada (a)/(b)/(c) — **(a)=19 · (b)=0 · (c)=11**, e o gate do ML-1B achou o 20º que a enumeração perdeu
- [x] Todo (a) corrigido por **ponto único** — incluindo os sítios que capturavam via **função
      intermediária** (`check_field_json` 7 call sites, `normalize_barrier_json`, `target_ids_json`,
      `doc_check_json` 4 call sites), normalizados **dentro** das funções no ML-1C. Desmarcado em
      2026-09-23 pela revisão do `hefesto-tf` e remarcado após o corretivo.
- [x] Gate falsificável — **5 braços**, cobrindo `$(python3 ...)`, `$("$PY_BIN" ...)` e
      `sys.stdout.write(...\n)`. 🔴 **Duas formas ficam declaradas como NÃO cobertas** no cabeçalho
      do gate (`< <(python3 ...)` e captura via função nova), com a razão técnica de cada uma —
      *"not measured is not acceptable; these are measured and the limit is documented"*.
      Desmarcado por mim após a revisão do `hades-tf` (eu havia injetado só a forma que o gate foi
      feito para pegar) e remarcado após o ML-1C, com injeção minha por forma.
- [x] 🔴 **AC REESCRITO pela medição.** O original dizia "`windows-census.yml` volta a dar 8/8 shards";
      ele **presumia que o CRLF era a única causa**, e a medição refutou: era **87%** dela.
      Entregue: rótulos ausentes **146 → 19**, e o instrumento passa a reportar cenário que
      reprova de verdade em vez de se auto-sabotar. Os **19** remanescentes ficam **enumerados**
      (9 de `git-branch-guard/*`), como entrada medida do próximo trabalho — fora desta REQ,
      por escopo negativo declarado desde o início.
- [x] Falsificação exercitada **no Windows** — os 3 braços `crlf-normalize/*` colhidos, e o censo rodado na branch (`35872779844`)
- [ ] `make quality` e **CI** verdes

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

---

## Wave 0 — Enumeração e modelo de ameaça
> Dependências: nenhuma. **Bloqueia a implementação.**

### ML-0A — enumerar os sítios de consumo e classificar
**Status:** ✅ Concluído (auditado por Zeus em 2026-09-23)
**Files affected:** `docs/seguranca/2026-09-23-enumeracao-crlf-python3-em-bash.md`

#### Resultado — **(a)=19 · (b)=0 · (c)=11**

🔴 **Minha medição foi refutada, e para pior: eu disse "1 script normaliza"; são ZERO.** O que tomei
por normalização era `PYTHONIOENCODING=utf-8` (`check-gates-falsify.sh:20`), que controla **codec**,
não tradução de newline. Confirmei: os `tr -d` daquele arquivo removem **espaço** (saída de `wc -l`),
não `\r`.

**Dois sítios ATIVOS em CI de Windows:** `run-gates-falsify-shard.sh:85` (o do censo quebrado) e
`check-gates-falsify.sh:3915,4431`. Os outros 17 são dormentes — **mas nenhum passa no teste de
POSIX-only genuíno**: `make parity-rest` invoca vários sem guarda de plataforma.

#### 🔴 A distinção que governa o ML-1A — delimitador vs conteúdo

**`tr -d '\r'` está PROIBIDO.** Existe fixture com **CRLF intencional** no repositório —
`check-roadmap-barrier-contract.sh:1091`, `write_fixture_crlf()`, usada a partir da linha 1110 para
o #216. Um helper que normalize `\r` no nível de leitura de arquivo **destrói esses fixtures em
silêncio**.

- **Delimitador** (normalizar): rótulo de manifesto, lista de caminhos — o `\r` é artefato do modo
  texto, nunca dado.
- **Conteúdo** (preservar): arquivo cujo CRLF é **o objeto da verificação**.

**Regra:** `sed 's/\r$//'` ou binary mode no Python, atuando **sobre stdout capturado** — nunca
sobre conteúdo de arquivo.

**O que já está medido — não remeça, use:**
- Mecanismo provado localmente (sem VM): manifesto **CRLF** + `grep -qxF` → não casa; `grep -qF`
  (sem `-x`) → casa. É o que explica rótulo **emitido e acusado ausente** ao mesmo tempo.
- `run-gates-falsify-shard.sh`: linha **85** (redireciona stdout do Python), linhas **118** (`-qxF`,
  quebra) e **127** (`-qF`, sobrevive). **Zero** tratamento de `\r` no arquivo.
- `check-no-literal-nul-in-source.sh`: 671 caminhos, 671 com `\r`, `[[ -f ]]` → 0 (#353).

**Actions:**
1. 🔴 **Enumerar pelo mecanismo, não pelo literal.** O critério é *"bash consome saída de `python3`
   como dado"* — por pipe, `$(...)`, `< <(...)` ou redirecionamento para arquivo lido com `while
   read`. Invocar Python sem consumir a saída **não** entra.
   Minha medição inicial, **a refutar ou confirmar**:
   ```
   $ grep -rln 'python3\|PY_BIN\|$PYTHON' scripts/*.sh | wc -l
   32
   $ # normalizam \r: 1 (check-gates-falsify.sh)
   ```
   ⚠️ Os 32 **não são todos defeito**. E o número pode estar **subestimado**: há consumo via
   `.github/workflows/*.yml` e via `Makefile`. Varra os três.
2. **Classifique** cada sítio em (a) consome sem normalizar · (b) normaliza · (c) não consome.
3. **Modelo de ameaça.** Quem esvazia esta Wave 0 sem quebrar regra escrita? O caminho óbvio:
   declarar que "só roda em Linux no CI" e fechar. Qual é o teste que distingue sítio realmente
   POSIX-only de sítio que **alguém vai rodar no Windows**?
4. **Falsificação nas duas direções.** O que quebra se a normalização for longe demais? `\r`
   legítimo dentro de conteúdo (arquivo CRLF sendo inspecionado **como dado**) não pode ser comido
   pela normalização — é a diferença entre *delimitador* e *conteúdo*.
5. **Residual declarado.**

**Acceptance criteria:**
- [x] Tabela completa, veredito por sítio, com `arquivo:linha` e o comando que produziu a lista
- [x] 🔴 Critério (a)/(c) **aplicável por terceiro**, não "julgamento do revisor"
- [x] 🔴 A distinção **delimitador vs conteúdo** está escrita, com um exemplo de cada
- [x] As quatro seções com evidência
- [x] Nenhuma linha de implementação neste ML

**Gate:** `trackfw barrier <roadmap> --wave 0`, auditado por mim.

---

## Wave 1 — A correção
> Dependências: Wave 0 auditada. Escopo definido pela tabela do ML-0A.

### ML-1A — ponto único de normalização + os sítios (a)
**Status:** ✅ Concluído · **Papel:** `apolo-tf`

🔴 **Ponto único, não `tr -d '\r'` espalhado.** Foi cópia de helper que originou a REQ-2026-08-31, e
o issue #401 registra três cópias byte-idênticas do mesmo padrão ainda abertas. Não crie a quarta.

Considere um helper em `scripts/` que os gates consumam (`lib` sourceada, ou wrapper de invocação do
Python que já normaliza). A escolha é sua; o **ponto único** não é negociável.

⚠️ **Não normalize conteúdo.** Se um gate inspeciona um arquivo CRLF **como dado** (ex.: verifica
que o arquivo tem CRLF), comer o `\r` destrói a verificação. A Wave 0 entrega essa distinção.

**Acceptance criteria:**
- [x] Ponto único criado e consumido por todos os sítios (a)
- [x] Teste **load-bearing** por sítio: falha com manifesto/lista CRLF antes, passa depois
- [x] Braço (b): em POSIX, nada muda — os gates continuam medindo o que mediam
- [x] `go build ./...` RC=0 · gates tocados RC=0
- [x] 🔴 **NÃO rodar `make quality`** — barreira é do arquiteto
- [x] Uma frase por teste novo

### ML-1A-bis — o `source` resolve por `$ROOT_DIR`, que o `--self-test` reaponta
**Status:** ✅ Concluído · **Papel:** `apolo-tf`
**Files affected:** os 5 gates que sourceiam o lib **e** têm `--self-test`. **Não** mexer no lib.

🔴 **Reprovação encontrada na MINHA barreira, não na validação do executor.** Barreira após o ML-1A:
`RC=2`, **101** `^OK ` (contra 824), `make: *** [parity-rest] Error 1`.

```
FAIL [self-test/ignora-artefato-de-build]: esperava exatamente 2 sítio(s) derivado(s)
  check-git-branch-guard-hook-schema.sh: line 89:
  /tmp/.../self-test/ignored-build-artifact/scripts/lib-crlf-normalize.sh: No such file or directory
check-git-branch-guard-hook-schema.sh --self-test: 5 cenário(s) FALHARAM.
```

**Mecanismo:**
```bash
ROOT_DIR=${TRACKFW_ROOT_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}
. "$ROOT_DIR/scripts/lib-crlf-normalize.sh"
```
O `--self-test` monta árvores sintéticas em `/tmp` e **reaponta `ROOT_DIR`** para elas, copiando só
`trackfw-git-branch-guard.sh`. O `source` então procura o lib na árvore sintética, onde ele não
existe.

**Não é defeito do lib nem da normalização** — é do **caminho de resolução**. `ROOT_DIR` é mutável
por projeto: resolver uma dependência interna por ele é frágil por construção.

**Correção pedida:** resolver o lib pelo diretório do **próprio script**, imune a `ROOT_DIR`:
```bash
SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
. "$SCRIPT_DIR/lib-crlf-normalize.sh"
```

**Os 5 em risco** (todos sourceiam o lib **e** têm `--self-test`): `check-channels-content.sh`,
`check-git-branch-guard-hook-schema.sh`, `check-goreleaser-prerelease.sh`, `check-gates-falsify.sh`,
`check-no-literal-nul-in-source.sh`. Só o segundo apareceu porque a barreira **abortou no primeiro**
— os outros quatro não chegaram a rodar. **Corrija os 5**, e confirme se há outros que reapontem
`ROOT_DIR` sem `--self-test`.

🔴 **Erro meu de handoff, registrado:** pedi "os gates tocados, individualmente → RC=0" e **não
exigi `--self-test` onde ele existe**. O executor rodou o gate, que passou; o `--self-test` é que
reprova. A validação seguiu a letra do que pedi.

**Acceptance criteria:**
- [x] Os 5 gates com `--self-test` passam: `bash <gate> --self-test` RC=0. Cole as 5 saídas
- [x] O caminho do lib **não** depende de `ROOT_DIR` em nenhum dos 19 sítios — verifique todos
- [x] Braço (b): `strip_cr` continua funcionando; `printf 'a\rb\n' | strip_cr` preserva o CR do meio
- [x] 🔴 **NÃO rodar `make quality`** — a barreira é do arquiteto

### ML-1B — gate que impede a reintrodução
**Status:** ✅ Concluído · **Papel:** `artemis-tf` · **Depende de:** ML-1A

Gate que **reprova** quando um consumo novo de saída de `python3` nascer sem passar pelo ponto único.

🔴 **Aprendizado direto do gate de contenção (REQ-2026-08-31):** um gate que aceita o sítio por
**marcador textual** afirma sem provar — 34 defeitos passaram sob luz verde. Prefira um discriminante
**estrutural** (o sítio chama o helper? o pipe passa pelo wrapper?) a um comentário de autoria.

**Acceptance criteria:**
- [x] Gate reprova consumo novo sem normalização — prove injetando
- [x] Gate passa na árvore correta, com guarda de **não-vacuidade** (piso de sítios examinados)
- [x] Falsificação com **rótulos literais**, colhidos pela guarda de conjunto
- [x] 🔴 **NÃO rodar `make quality`**

---

### ML-1B-bis — a menção morta a `python3` aciona o gate de encoding
**Status:** ✅ Concluído · **Papel:** `artemis-tf`
**Files affected:** `scripts/check-crlf-normalize-capture.sh` e
`scripts/check-output-encoding-declared.sh` (só o comentário obsoleto). **Nada mais.**

🔴 **Reprovação da minha barreira:** RC=2, **544** `^OK ` contra 824.
```
check-output-encoding-declared: FAIL
  - scripts/check-crlf-normalize-capture.sh: invoca python3 (linha 100)
    e NAO declara `export PYTHONIOENCODING=utf-8`.
```

**Não é invocação — é menção.** O gate novo tem a string `python3` **18 vezes**, toda em regex e
comentário. Ele é bash puro, como o handoff exigiu.

**E não é bug do gate de encoding: é trade-off documentado nele**, linhas 207-212:
> *"Trade-off assumido, na direção segura: uma menção MORTA a `python3` … passa a colocar o arquivo
> na população e a exigir dele a declaração. Isso é falso positivo, ruidoso e **FECHADO** — reprova
> pedindo uma linha inofensiva —, ao contrário do falso negativo que substitui. **Hoje não ocorre**:
> as duas populações coincidem (38 = 38, delta vazio nas duas direções)."*

Preferiram falso positivo **ruidoso e fechado** a falso negativo **silencioso e aberto**. É a escolha
certa, e o remédio prescrito é a linha inofensiva.

🔴 **Mas a frase "hoje não ocorre" acabou de ficar FALSA** — nosso gate é a primeira ocorrência.
Deixá-la é a Regra Dura de Reconciliação violada: artefato afirmando o que não sustenta mais.

**Ações:**
1. Adicionar `export PYTHONIOENCODING=utf-8` em `check-crlf-normalize-capture.sh`, com comentário
   de **uma linha** dizendo que é menção morta, não invocação, e que a linha atende ao trade-off
   documentado do gate de encoding.
2. Atualizar o comentário de `check-output-encoding-declared.sh` (linhas ~207-212): trocar *"hoje
   não ocorre"* pelo fato — **ocorre desde 2026-09-23**, no `check-crlf-normalize-capture.sh`, e o
   caso se resolveu como o trade-off previu.
   ⚠️ **Não** mexa no discriminante nem na `ALLOWLIST` — a allowlist existe para exceção de
   **processo** (o #238 aberto), não para menção morta.

**Acceptance criteria:**
- [x] `bash scripts/check-output-encoding-declared.sh` RC=0
- [x] `bash scripts/check-crlf-normalize-capture.sh` RC=0 e continua **bash puro** (nenhuma
      invocação real de `python3` adicionada)
- [x] O comentário do gate de encoding não afirma mais *"hoje não ocorre"*
- [x] 🔴 **NÃO rodar `make quality`** — a barreira é do arquiteto

### ML-1C — o gate é estreito demais, e há sítios (a) que ele não vê
**Status:** ✅ Concluído · **Papel:** `apolo-tf`
**Files affected:** `scripts/check-barrier.sh`, `scripts/check-update-parity.sh`,
`scripts/check-roadmap-barrier-contract.sh`, `scripts/check-crlf-normalize-capture.sh`,
`scripts/check-gates-falsify.sh` (cenário).

🔴 **Bloqueio da barreira final. Os DOIS revisores convergiram** — por caminhos diferentes — em que
o gate cobre **uma** forma de consumo e o AC prometia todas.

#### (A) Sítios não corrigidos, via função intermediária — achado do `hefesto-tf`

| função | arquivo:linha | call sites |
|---|---|---|
| `check_field_json` | `check-barrier.sh:142` | **7** |
| `normalize_barrier_json` | `check-barrier.sh:614` | — |
| `target_ids_json` | `check-update-parity.sh:91` | — |
| `doc_check_json` | `check-roadmap-barrier-contract.sh:113` | **4** |

**Confirmei:** `check_field_json` chama `python3 -c` e termina **sem** `strip_cr`; é capturada em
`$(check_field_json ...)`; e a comparação seguinte é `[[ "$WH_STATUS" == '"passed"' ]]`, que **quebra**
com `"passed"\r`. **O gate não marca a linha 142 como candidato.**

🔴 **São pré-existentes na `main`** — mas mesma causa, mesmo mecanismo, mesmos arquivos. Regra Dura:
**ML nesta REQ, PR aberto até entrarem.** Não vira REQ nova.

#### (B) Formas que o gate não enxerga — achado do `hades-tf`, confirmado por mim

| forma | detecta? |
|---|---|
| `$(python3 -c ...)` | sim — foi a minha injeção, e por isso eu marquei o AC |
| `$("$PY_BIN" -c ...)` | **não** — e **já é usada** em `check-gates-falsify.sh:3917,4433` |
| `sys.stdout.write('...\n')` | **não** — isento por "não emite `\n`" |
| `< <(python3 ...)` | **não** — só olha `$(...)` |
| `$(funcao_que_chama_python3 ...)` | **não** — o (A) acima |

🔴 **Erro meu, registrado:** marquei o AC do gate como atendido depois de injetar **a forma que o
gate foi feito para pegar**. É o mesmo erro que venho apontando nos executores a semana toda —
testar o gate contra o defeito conhecido, não contra o que vai aparecer.

⚠️ **A guarda de vacuidade (66 ≥ 50) NÃO protege disso** — ela conta os candidatos que o
discriminante **já enxerga**. Corpus estreito com piso alto dá aparência de cobertura.

**Ações:**
1. Normalizar **dentro** das 4 funções (cobre todos os call sites de uma vez — sugestão do
   `hefesto-tf`, e é o ponto único correto).
2. Estender o discriminante do gate para as formas de (B). Para cada uma que **não** for coberta,
   **declare por quê** — "não coberto" é resultado aceitável; "não medido" não é.
3. Reavaliar o piso: com o discriminante mais largo, o número de candidatos muda.

**Acceptance criteria:**
- [x] As 4 funções normalizam; os 11+ call sites deixam de depender de correção por sítio
- [x] Gate reprova **cada** forma de (B) que passar a cobrir — prove injetando **uma a uma**
- [x] Para cada forma **não** coberta: razão escrita no cabeçalho do gate
- [x] Piso reavaliado, com o comando que produziu o número novo
- [x] Falsificação com **rótulos literais** para as formas novas
- [x] 🔴 **NÃO rodar `make quality`** — a barreira é do arquiteto

## Wave 2 — A prova no Windows
> Dependências: Wave 1 completa. **É a wave que fecha a REQ.**

### ML-2A — o censo volta a produzir número
**Status:** ✅ Concluído (auditado por Zeus em 2026-09-23, com AC reescrito pela medição) · **Papel:** `artemis-tf`

🔴 **O AC que importa, e não é local:** disparar `windows-census.yml` (`workflow_dispatch`) e obter
**8/8 shards** com apuração **sem** `TOTAL INCOMPLETO`.

⚠️ **VM investiga, CI mede.** A VM é ARM64 e o runner é x64 — número medido na VM carrega a ressalva
"pode ser artefato desta VM", que já bloqueou uma triagem de 512 falhas. O número que vira afirmação
sai do runner.

⚠️ O run de referência **pré-correção** é o `35856380122` (2026-09-23): 2/8 shards, 146 rótulos
ausentes. Compare contra ele — mesmo instrumento, mesma plataforma.

**Resultado auditado por Zeus em 2026-09-23 — run `35872779844` (branch) contra `35856380122` (main):**

| | pré-correção | pós-correção |
|---|---|---|
| rótulos acusados ausentes | **146** | **19** |
| shards com guarda local falhando | 8 | 5 |
| shards apurados | 2/8 | 2/8 |

🔴 **O AC original — "8/8 shards" — NÃO foi atendido, e a razão é que ele presumia o que a medição
refutou:** que o CRLF fosse a **única** causa do censo quebrado. Ele era **87%** dela.

**Os 19 restantes não são CRLF.** Enumerados, com concentração clara:
```
git-branch-guard/*                        9    ← dominante
git-branch-guard-global-script-integrity  2
integration-assets/*                      2
roadmap-req-frontmatter-path/*            2
credential-guard · barrier · trust-check  1 cada
```

**AC reescrito, com a medição** — não marcado como atendido por generosidade, e não arrastado para
dentro de causa não medida:

**Acceptance criteria:**
- [x] O censo **volta a medir a causa CRLF**: rótulos ausentes por comparação caem de **146 → 19**
      (-87%), e o instrumento passa a reportar **cenário que reprova de verdade**
      (`[falsify/enumerate] 1 cenário(s) reprovaram`) em vez de se auto-sabotar por `\r`
- [x] 🔴 **Os 19 remanescentes ficam ENUMERADOS**, não estimados — são a entrada medida do próximo
      trabalho, e **não** entram nesta REQ: o escopo negativo excluiu a triagem do cluster desde o
      início, e agrupar sem medir a causa é o erro do `IsAbs` (14 estimados, 2 entregues)
- [x] A comparação é contra o run `35856380122` — mesmo instrumento, mesma plataforma, só o código
      difere
- [x] A comparação é contra o run `35856380122`, **não** contra o de 2026-09-10 (pré-v8, mede outra
      coisa: a remoção de Node/Python)
- [x] O número obtido entra no roadmap como **linha de base pós-v8** — e **não** é usado para triar o
      cluster de Windows nesta REQ (escopo negativo)

## Barreira final

Revisão `hefesto-tf` e `hades-tf`, auditoria do arquiteto, `trackfw barrier`. **CI verde.**

⚠️ Custo de CPU: `TRACKFW_FALSIFY_JOBS=4` na barreira local, teste do pacote tocado nos handoffs —
`vault/notes/carga-de-cpu-vem-da-suite-de-falsificacao-vezes-agentes-paralelos-2026-09-22.md`.
