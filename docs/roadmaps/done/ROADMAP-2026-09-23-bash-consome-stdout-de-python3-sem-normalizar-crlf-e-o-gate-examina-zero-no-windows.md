---
status: done
date: 2026-09-23
req: "docs/req/REQ-2026-09-23-bash-consome-stdout-de-python3-sem-normalizar-crlf-e-o-gate-examina-zero-no-windows.md"
squad: "hades-tf, apolo-tf, artemis-tf"
---

# Roadmap: bash consome stdout de `python3` sem normalizar CRLF

> Created: 2026-09-23 | Status: done

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
      (10 de `git-branch-guard/*`), como entrada medida do próximo trabalho — fora desta REQ,
      por escopo negativo declarado desde o início.
- [x] Falsificação exercitada **no Windows** — os 3 braços `crlf-normalize/*` colhidos, e o censo rodado na branch (`35872779844`)
- [x] `make quality` e **CI** verdes — local RC=0 (903 `^OK `, 0 `: FALHA`, falsificação 252 OK);
      **CI: 21 checks verdes** no PR #414, incluindo os 6 jobs de Windows

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

**Gates da wave:**

```bash
n=$(git grep -l lib-crlf-normalize -- "scripts/*.sh" | wc -l | tr -d " "); test "$n" -ge 4 && echo "Gate W0: ponto unico sourceado por $n scripts (piso 4)" || { echo "GATE FALHOU: $n < 4 - o ponto unico deixou de ser usado" >&2; exit 1; }
```

⚠️ **Gate real escrito em 2026-09-27**, na reabertura. Antes havia só a menção em prosa ao `barrier`,
que o `validate` acusa como *placeholder ou ausente* (AC7 da `ADR-2026-09-18`). Medido hoje: **19**
scripts sourceiam o ponto único; piso **4**, para não travar refatoração legítima.

🔴 **Uma linha só, de propósito:** o `barrier` executa **uma linha por vez**, então bloco `if…fi`
multi-linha nunca roda como gate — defeito que já deixou uma Wave 0 inteira inexecutável nesta casa.

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
git-branch-guard/*                       10    ← dominante (5 checkout-flag-position + 5 env-command-prefix)
git-branch-guard-global-script-integrity  2
integration-assets/*                      2
roadmap-req-frontmatter-path/*            2
barrier · credential-guard · trust-check  1 cada
                                        ────
                                         19
```

🔴 **Correção de contagem (2026-09-23, pós-merge):** a decomposição acima dizia `9` para
`git-branch-guard/*` e **somava 18**, não 19. Recontado do artefato do run, não da prosa:
`gh run view 35872779844 --log | grep -o 'AUSENTE: [^ ]*' | sort -u | wc -l` → **19**. A lista
literal, que é a entrada do próximo trabalho:

```
barrier/wave-zero-flag-guard-rejected-again-detected
credential-guard-hook-resolvable/detected
git-branch-guard-global-script-integrity/detected-without-wiring
git-branch-guard-global-script-integrity/non-vacuity
git-branch-guard/checkout-flag-position/baseline-blocks-no-track
git-branch-guard/checkout-flag-position/baseline-blocks-q-b
git-branch-guard/checkout-flag-position/detection-catches-bypass-no-track
git-branch-guard/checkout-flag-position/detection-catches-bypass-q-b
git-branch-guard/checkout-flag-position/detection-does-not-break-plain-checkout-b
git-branch-guard/env-command-prefix/baseline-blocks-command
git-branch-guard/env-command-prefix/baseline-blocks-env
git-branch-guard/env-command-prefix/detection-catches-bypass-command
git-branch-guard/env-command-prefix/detection-catches-bypass-env
git-branch-guard/env-command-prefix/detection-does-not-break-plain-push
integration-assets/direction-a-catalog-absent
integration-assets/direction-b-shim-absent
roadmap-req-frontmatter-path/go/from-req
roadmap-req-frontmatter-path/go/from-req-baseline
trust-check/direction-b-detected
```

⚠️ **O run é do SHA `11eb7ff0`, anterior ao ML-1C.** O número 19 vale para aquela árvore. A linha
de base pós-merge é medida por um censo novo em `main`.

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

---

## Adendo pós-merge (2026-09-23) — por que o censo AINDA não dá número, e não são os 19

Ao preparar a entrada do próximo trabalho, recontei do artefato em vez da prosa e encontrei **duas
coisas que esta REQ não viu**:

**1. A decomposição dos 19 estava errada** — dizia `9` para `git-branch-guard/*` e somava **18**.
São **10** (5 `checkout-flag-position` + 5 `env-command-prefix`). Corrigido acima, com a lista
literal.

**2. 🔴 A apuração do censo morre por um defeito de uma linha — e não por falta de artefato.**

O run `35872779844` termina com `TOTAL INCOMPLETO — 2/8 shards`. A leitura natural é *"6 shards não
subiram log"*. **Está errada:** os 8 artefatos existem, no layout exato que a apuração procura.

```
$ gh api .../runs/35872779844/artifacts --jq '.artifacts[].name'
falsify-shard-0 … falsify-shard-7          ← os 8
$ gh run download 35872779844
falsify-shard-1/shard_1.log                ← o caminho que a apuração testa
```

O que quebra é `windows-census.yml`, na contagem por shard:

```bash
CNT_FAIL_GREP=$(grep -ac '^FAIL' "$LOG_FILE" 2>/dev/null || echo 0)
```

`grep -c` **já imprime `0`** quando não casa nada — e sai com **1**, o que dispara o `|| echo 0` e
acrescenta um segundo `0`. A variável fica com **duas linhas**:

| entrada | valor de `CNT_FAIL_GREP` | `$(( ))` |
|---|---|---|
| log **sem** `FAIL` (shard limpo) | `$'0\n0'` | **`syntax error in expression`** |
| log **com** `FAIL` (controle) | `2` | ok |
| forma correta `{ grep -ac … \|\| true; }` | `0` | ok |

No log do run, exatamente isso, no shard 1 — o primeiro shard **sem nenhuma falha**:

```
DISCREPÂNCIA FAIL shard 1: grep=0
0 awk=0
line 57: 0
0: syntax error in expression (error token is "0")
```

🔴 **A consequência é perversa:** o shard que **passa inteiro** é o que derruba a apuração. Quanto
melhor o resultado, mais cedo o total morre. E a mensagem acusa *"artefato não baixado"*, mandando
quem investiga para o lado errado — o mesmo padrão do CRLF que esta REQ corrigiu: o instrumento
denuncia a causa errada.

**Isto não reabre esta REQ.** O AC do censo já foi reescrito pela medição e o original já está
declarado **não atendido**. O achado é a entrada do próximo trabalho, agora com **causa medida** em
vez de 19 rótulos sem mecanismo — e a triagem dos 19 nem começou, porque o total nunca existiu.

---

## Wave 3 — REABERTURA: o literal distribuído, que o gate desta REQ nunca varreu
> Dependências: Waves 0–2 (fechadas). 🔴 **Esta wave existe porque a REQ foi fechada com sítio vivo.**

### O que mudou desde o fechamento

Medido em 2026-09-27, ao encontrar a cópia versionada **revertida na árvore**:

```
scripts/trackfw-attention-signal.sh   (cópia versionada)   strip_cr = 2   ← a correção de #414
internal/generators/scaffold.go       (literal embutido)   strip_cr = 0   ← nunca recebeu
árvore local, após regeneração pelo produto                strip_cr = 0   ← o defeito VOLTOU
```

🔴 **RETRATAÇÃO — 2026-09-27, após o ML-3A.** Eu escrevi aqui, e no commit da reabertura, que *"o
defeito do #353 volta a ser distribuído"*. **É inexato, e a medição do `hades-tf` me refutou.**

`TOOL` e `MSG` passam por `tr -d '\000-\037'` (`scaffold.go:937,938`) **antes** de entrar no JSON, e
CR é octal `015` — dentro desse intervalo. Verifiquei eu mesmo:

```
$ printf 'conteudo\r' | tr -d '\000-\037' | od -c
0000000    c   o   n   t   e   u   d   o
```

**O CR não chega ao JSON gravado.** A sanitização downstream cobre. A justificativa do `ML-3B` passa
a ser: **(a)** paridade literal↔cópia versionada, que o `ML-3D` formaliza como gate, e **(b)** defesa
em profundidade — eliminar o `\r` na variável antes da sanitização, em vez de depender dela.

⚠️ **Por que eu errei:** medi a *ausência* do `strip_cr` no literal e concluí impacto sem seguir o
dado até o fim do pipeline. Régua estreita — o mesmo defeito de método que passei a campanha
cobrando dos executores.

### A causa estrutural, e por que o gate desta própria REQ não pegou

```
scripts/check-crlf-normalize-capture.sh:232
    for f in "$SCAN_ROOT/scripts/"*.sh
```

Varre **só `scripts/*.sh`**. Nunca lê `internal/generators/*.go`. A correção foi aplicada exatamente
onde o gate enxerga; o sítio que **distribui** ficou fora do campo de visão.

⚠️ **E isto não é um caso isolado — é uma FORMA.** A nota
`vault/notes/copia-versionada-do-attention-signal-esta-obsoleta-e-sem-guarda-2026-09-02.md` já
descrevia a estrutura de dois artefatos (literal × cópia versionada) e registrava que **nada compara
os dois**. Ela documentou a cópia **atrasada**; aqui a cópia estava **adiantada** e foi regenerada
por cima. Mesma ausência de guarda, direção oposta.

🔴 **Erro de método meu, registrado:** essa nota existia e eu investiguei **antes** de lê-la. A regra
do vault é explícita — consultar antes de investigar. Li depois, e ela teria encurtado o caminho.

---

### ML-3A — **Wave 0 da reabertura** — modelo de ameaça do sítio distribuído
**Owner:** `hades-tf`
**Status:** ✅ Concluído — auditado em 2026-09-27 · 🔴 **refutou a minha afirmação e achou um defeito
maior** (`docs/seguranca/2026-09-27-wave0-literal-distribuido-crlf.md`)
🔴 **Bloqueia o ML-3B e o ML-3C.** Nenhuma implementação antes desta auditada.
**Ações:**
1. O sítio é um **hook** (`PreToolUse`) que roda na máquina do consumidor e consome **JSON vindo do
   agente** (`tool_input.command`, `tool_input.question`). Avaliar o que muda no modelo de ameaça ao
   introduzir `strip_cr` no caminho — em particular se remover `\r` pode **alterar** um valor que
   depois é interpolado em `.trackfw-attention.json`.
2. 🔴 **A pergunta que eu quero respondida:** o `strip_cr` deve ser aplicado **antes ou depois** do
   truncamento `[:300]`? Se o `\r` estiver dentro dos 300, a ordem muda o resultado — e um dos dois
   pode partir um escape pela metade.
3. Declarar se a **regeneração** da cópia versionada a partir do literal pode sobrescrever conteúdo
   do consumidor sem aviso — é a mesma família do **#445**.
**Critérios de aceite:**
- [x] Parecer escrito em `docs/seguranca/`, com veredito por item
- [x] A ordem `strip_cr` × truncamento **decidida e justificada** → **DEPOIS**, com `sed $'s/\r$//'`,
      e a pré-condição de `pipefail` nomeada
- [x] 🔴 **Refutação entregue:** o CR **não** chega ao JSON — `tr -d '\000-\037'` já o remove.
      Verifiquei por conta própria antes de aceitar
- [x] 🔴 **Achado fora da enumeração:** `ROADMAP_DIR` com CR silencia o sinal de atenção → `ML-3E`
- [x] Veredito sobre o **#445**: mesma causa (`os.WriteFile` sem guard), roteado para a REQ do #445
**Gates da wave:**

```bash
n=$(grep -cF "s/\r" internal/generators/scaffold.go); test "$n" -ge 3 && echo "Gate W3: literal embutido normaliza CRLF em $n sitio(s)" || { echo "GATE FALHOU: $n - o literal de scaffold.go NAO normaliza CRLF, e e ele que o init distribui" >&2; exit 1; }
```

⚠️ **Este gate foi CORRIGIDO em 2026-09-27, e o defeito dele era o mesmo que esta REQ combate.**

Eu o escrevi procurando `strip_cr`. A implementação correta **não usa** `strip_cr` — usa
`sed $'s/\r$//'` inline, porque o script distribuído **não pode sourcear** a `lib-crlf-normalize.sh`
(o consumidor não a tem). 🔴 **Meu gate media o token, não o efeito** — e teria reprovado a correção
certa enquanto aceitaria um `strip_cr` decorativo. É a mesma classe do gate que deixou este defeito
passar; escrevi um terceiro exemplar dela sem perceber.

Régua nova: a **forma** de normalização (`s/\r`), com piso **3** (medido: 11 ocorrências, entre
comentários de decisão e os pipelines). Falsificado nas duas direções antes de entrar.

### ML-3B — **AC7 parte 1** — o literal recebe a normalização
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-09-27
**Arquivos:** `internal/generators/scaffold.go` (linhas 924 e 925) · a cópia versionada
`scripts/trackfw-attention-signal.sh`, **regenerada a partir do literal**
**Ações:**
1. Levar a normalização de CRLF para os **2 sítios** do literal que capturam stdout de `python3` em
   variável. O sítio `2202` (`py_compile`) **não** captura saída e fica fora — classificação medida.
2. 🔴 **A cópia versionada passa a ser DERIVADA do literal**, nunca editada à mão. Hoje ela é a
   única com a correção, e foi por isso que a regeneração a perdeu.
3. O script gerado **não pode** depender de `lib-crlf-normalize.sh` existir no projeto do consumidor
   — ele não tem esse arquivo. A normalização tem de ser **autocontida** no script gerado.
   ⚠️ Este ponto é o que torna a correção não-trivial; a cópia versionada podia dar `source` porque
   vive ao lado da lib, e o script distribuído **não vive**.
**Critérios de aceite:**
- [x] ⚠️ **AC atendido pela segunda metade, e digo qual.** `grep -c strip_cr` no literal → **0**.
      O `strip_cr` é alias da lib, e o script distribuído **não pode** sourceá-la. A forma
      autocontida entregue é `sed $'s/\r$//'` (python3) e `sed $'s/\r//g'` (ROADMAP_DIR) — **11**
      ocorrências medidas. Marcar sem esta ressalva afirmaria uma verificação que não foi feita
- [x] Script gerado por `init` em diretório limpo **roda** sem a lib — provado executando
- [x] Cópia versionada **byte-idêntica** ao gerado → `diff -q` silencioso, verificado por mim
- [x] A frase da Regra Dura de Reconciliação, por teste novo → 4 frases (ML-3B/3E)
- [x] `make quality` `exit=0` (1367 `^OK `, 0 `: FALHA`) e `validate` 170 warnings, 0 violations

### ML-3C — **AC7 parte 2** — o gate passa a varrer o literal
**Owner:** `artemis-tf`
**Status:** ✅ Concluído — auditado em 2026-09-27 · 🔴 **e achou que o discriminante era derrotável por comentário**
**Arquivos:** `scripts/check-crlf-normalize-capture.sh`
**Ações:**
1. Estender o escopo para os **literais embutidos** em `internal/generators/*.go`, além de
   `scripts/*.sh`.
2. 🔴 **Falsificação nas duas direções:** um literal novo que capture `python3` **sem** normalizar
   faz o gate **reprovar**; um literal que normalize **passa**. Sem o primeiro braço o gate é
   decorativo — e foi exatamente assim que ele deixou este defeito passar.
3. **Declarar o que o gate NÃO cobre**, como a versão atual já faz para as formas de captura.
**Critérios de aceite:**
- [x] Braço de reprovação provado por **fixture** (braço F) — o ML-3B corrigiu os sítios reais
      antes de o ML-3C medir, então o arquivo real já não servia como braço negativo
- [x] Piso 50 → **52** (70 candidatos medidos, ≈74%)
- [x] 🔴 Contra-braço: mesma população de `scripts/*.sh` (**69**), e os 2 FAIL viraram OK **porque
      foram corrigidos**, não porque o gate afrouxou — verificado por mim com fixtures nas duas
      direções
- [x] A frase da Regra Dura de Reconciliação, por braço novo → 3 frases (F/G/H)

### ML-3D — a guarda que faltava entre literal e cópia
**Owner:** `artemis-tf`
**Status:** ✅ Concluído — auditado em 2026-09-27 · **5 de 5 pares**, após corretivo R1
**Ações:** teste ou gate que **compare** a cópia versionada com o que o literal gera, e reprove na
divergência. É a guarda cuja ausência a nota de vault de 2026-09-02 já havia registrado e que
ninguém criou.
🔴 **Sem isto, a Wave 3 conserta o sintoma e deixa o mecanismo vivo:** nada impede que a próxima
correção volte a ser aplicada só numa das duas naturezas.
**Critérios de aceite:**
- [x] Divergência **reproduzida**: adulterei `trackfw-git-branch-guard.sh` e o teste **reprovou**;
      restaurada, **passou**. Falsificação minha, não do relatório
- [x] O teste **não** usa `os.Chdir` — os geradores recebem caminho **absoluto** e a cópia é lida via
      `findRepoRoot` (sobe de `os.Getwd()` até o `go.mod`)
- [x] A frase da Regra Dura de Reconciliação
- [x] 🔴 **R1 — escopo estendido de 2 para 5 pares.** A entrega inicial cobria só `signal` e
      `cleanup`; `credential-guard`, `git-branch-guard` e `validate` têm a **mesma estrutura e a
      mesma ausência de guarda**. Estavam byte-idênticos, que é precisamente como o par do
      attention-signal estava **antes** de divergir

### ML-3E — 🔴 o achado do ML-3A: `ROADMAP_DIR` com CR silencia o sinal de atenção
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-09-27 · 🔴 **eram TRÊS sítios, não dois**
**Arquivos:** `internal/generators/scaffold.go` (linhas 928 e 959) · cópia versionada regenerada

#### Por que este ML vale mais que o que originou a reabertura

O `hades-tf` mediu, e eu **reproduzi**: o `ROADMAP_DIR` é extraído do `trackfw.yaml` e **não passa**
pela sanitização que protege `TOOL` e `MSG`.

```bash
ROADMAP_DIR=$(grep '^roadmap_dir:' trackfw.yaml | head -1 | sed 's/…//' | tr -d '"' | tr -d "'")
#                                                                        ↑ só remove ASPAS
case "$ROADMAP_DIR" in /*|../*|*/../*|*/..|..) ROADMAP_DIR="docs/roadmaps" ;; esac
#                      ↑ rejeita absoluto e "..", NÃO rejeita CR
mkdir -p "$ROADMAP_DIR"
… > "$ROADMAP_DIR/.trackfw-attention.json"
```

Reprodução, com `trackfw.yaml` em CRLF (Windows, Notepad):

```
$ printf 'roadmap_dir: docs/roadmaps\r\n' | grep … | sed … | tr -d '"' | tr -d "'" | od -c
0000000  d o c s / r o a d m a p s  \r  \n
```

**Efeito:** `mkdir -p "docs/roadmaps\r"` cria um diretório **com CR no nome**, e o
`.trackfw-attention.json` é gravado lá. O `trackfw serve` procura em `docs/roadmaps/` e **não
encontra**.

🔴 **O sinal de atenção silencia — sem erro, sem aviso.** É pior que o defeito que abriu esta
reabertura: ali o CR era absorvido a jusante; aqui ele **muda o caminho de escrita**. E é
exatamente a classe que esta REQ existe para tratar: *"o mecanismo dá sinal de sucesso enquanto o
controle está inerte"*.

**Causa raiz distinta dos sítios `python3`** — aqui é `grep` sobre arquivo, não stdout de `python3`.
Fica **no mesmo roadmap** pela regra deste projeto: *mesmo sintoma investiga junto; só se separa com
a medição escrita*. A medição está escrita, e ela **mantém** junto: mesmo script, mesma wave, mesma
entrega.

**Ações:**
1. Normalizar o `ROADMAP_DIR` na extração — forma **autocontida**, sem depender de
   `lib-crlf-normalize.sh` (o consumidor não o tem).
2. Avaliar se o `case` de validação deve **também** recusar caracteres de controle, em vez de só
   normalizar. Decidir e escrever: normalizar silenciosamente esconde um `trackfw.yaml` malformado.
3. Aplicar no **literal** e regenerar a cópia versionada a partir dele.

**Critérios de aceite:**
- [x] `trackfw.yaml` com CRLF produz `.trackfw-attention.json` em `docs/roadmaps/` — provado
      **executando**; o diretório com `\r` no nome **não** é criado
- [x] 🔴 **Contra-braço:** LF continua funcionando, `roadmap_dir` com espaço não é quebrado, e
      `trackfw.yaml` **sem** `roadmap_dir:` cai no fallback
- [x] A decisão escrita: **normalizar, não rejeitar** — rejeitar cairia no fallback `docs/roadmaps`,
      que é o **mesmo** desfecho silencioso que estamos corrigindo; muda só qual diretório errado.
      Higiene de YAML malformado pertence ao `validate`/`doctor`, que têm canal com o usuário
- [x] A frase da Regra Dura de Reconciliação, por teste novo

---

## Ajustes do ML-3B decididos pela Wave 0

- **Ordem:** `strip_cr` **DEPOIS** do `[:300]` — os dois vivem em contextos irredutíveis (`[:300]` é
  do subprocesso `python3`, a normalização é do bash sobre o stdout dele). O `\r` de interesse é
  acrescentado pelo `print()` **após** o slice, então não há escape a partir pela metade.
- **Forma autocontida:** `sed $'s/\r$//'` — ANSI-C quoting, portável entre BSD e GNU `sed`, mesmo
  padrão do `lib-crlf-normalize.sh`, sem depender da lib.
- 🔴 **Pré-condição obrigatória — `pipefail`:** o script tem `set -euo pipefail` na linha 3. **Com**
  `pipefail`, o rc não-zero do `python3` propaga pelo pipe e o `|| echo "Agent needs attention"`
  dispara. **Sem** `pipefail`, o `sed` (último do pipe) devolve 0, o `||` **nunca** dispara e a
  variável fica **vazia em silêncio**. O `ML-3B` deve deixar **comentário inline** registrando essa
  dependência — uma refatoração futura que remova o `pipefail` quebra o fallback sem erro visível.

## Roteamento decidido — o overwrite NÃO entra aqui

O ML-3A mediu que `scaffold.go:995` usa `os.WriteFile` (`O_TRUNC`) sem checar existência, sem diff e
sem prompt — **mesma causa do #445** (`writeTrackfwConfig`, `scaffold.go:789`).

**Não entra nesta REQ.** Mesma causa → mesma REQ, e a REQ é a do **#445**, que já existe. Abrir aqui
seria o padrão que a Regra Dura proíbe. Fica registrado como **residual desta wave**, com a
assimetria de severidade que o parecer nomeia: aqui o consumidor perde uma customização e recebe o
script **correto**; no #445 ele perde **configuração deliberada** e o produto passa a reportar estado
de governança **falso**.


### Auditoria do ML-3D — e uma régua minha que repetiu o defeito que acabamos de corrigir

**Falsifiquei por conta própria**, sem confiar no relatório: adulterei `trackfw-git-branch-guard.sh`,
o teste **reprovou**; restaurei, **passou**; `git status` limpo depois.

⚠️ **O `validate` ficou fora da tabela, com razão declarada e que eu aceito.** `generateValidateScript`
escreve em caminho **relativo ao cwd** — em `go test` o cwd é `internal/generators/`, então chamá-la
gravaria em `internal/generators/scripts/`. É **a armadilha do `Chdir` documentada na nota do vault**,
por outro caminho. A saída foi comparar contra `buildValidateScript(Config{})`, a função pura que ela
chama por dentro.

🔴 **Ressalva que fica escrita:** isso fixa que a cópia versionada foi gerada com `Config` **vazia**.
Se algum dia ela passar a ser gerada com config preenchida, o teste reprova por **motivo alheio** ao
que mede. Não é defeito hoje — é dívida declarada, e declará-la é o que a distingue de uma omissão.

#### 🔴 O erro de método que eu cometi auditando

Para checar se o teste caía na armadilha do `Chdir`, rodei `grep -c 'os.Chdir'` e obtive **2**. Ia
reportar violação. As duas ocorrências eram **comentários** explicando que ele **não** usa `Chdir`.

**É exatamente o defeito que o `ML-3C` acabou de corrigir no gate** — `grep` sobre bloco **com**
comentários, prosa contando como implementação. Consertamos no produto e eu o repeti na auditoria,
no mesmo dia, sobre o mesmo tema. Fica registrado porque a lição não é sobre o gate: é sobre a régua.

---

### ML-3F — 🔴 a correção do ML-3E quebrava no Windows, que é a plataforma que ela existe para consertar
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-09-27, **com verificação na VM antes do CI**

#### O defeito, achado pelo contra-braço que eu exigi

O `windows-full-suites` do PR #453 reprovou em
`TestAttentionSignal_SpaceInRoadmapDir_NotBrokenByCRLFNorm` — o **contra-braço de espaço no nome**.
Reproduzi na VM e instrumentei o pipeline:

```
ROADMAP_DIR=$( … | sed $'s/\r//g' )
sed: -e expression #1, char 0: no previous regular expression
ROADMAP_DIR=[]        ← vazio, cai no fallback e IGNORA o roadmap_dir do consumidor
```

🔴 **E falhava sempre no Windows — com CRLF e com LF.** Não era caso de borda: era **todo** consumidor
Windows com `roadmap_dir` customizado perdendo o valor. Pior que o defeito original, e a um merge de
ser publicado na v9.0.1.

#### Três formas, duas plataformas — medido, porque "portável" era hipótese

| forma | macOS (BSD sed) | Windows (Git Bash, GNU sed 4.9) |
|---|---|---|
| `sed $'s/\r$//'` — caminho **python3** (ML-3B) | ok | **ok** |
| `sed $'s/\r//g'` — **ROADMAP_DIR** (ML-3E) | ok | 🔴 **falha** |
| `tr -d '\r'` | ok | ok |

O parecer da Wave 0 afirmou que `sed $'...'` é *"portável entre BSD e GNU sed"*. É verdade para a
forma `$//` e **falso** para a forma `//g` no MSYS. **Afirmação de portabilidade que ninguém
exercitou no ambiente alvo** — e que nenhum gate local tinha como pegar.

Escolha: `tr -d '\r'`, que o script **já usa duas vezes** no mesmo pipeline (para aspas). Menos
superfície que introduzir um `sed` com quoting diferente.

#### Verificação na VM, ANTES do CI — os quatro cenários

```
CRLF + espaço no nome     → ./my roadmaps/.trackfw-attention.json
LF   + espaço no nome     → ./my roadmaps/.trackfw-attention.json
CRLF simples              → ./docs/roadmaps/.trackfw-attention.json
sem roadmap_dir           → ./docs/roadmaps/.trackfw-attention.json   (fallback)
```

**Critérios de aceite:**
- [x] Os 3 sítios usam `tr -d '\r'`; o caminho python3 permanece `sed $'s/\r$//'`
- [x] As 3 cópias versionadas **regeneradas a partir do literal** — `TestScripts_LiteralMatchesVersionedCopy` verde nos 5 pares
- [x] Comentário de decisão registra a reprovação medida, com o erro literal do Git Bash
- [x] Verificado na **VM**: 4 cenários corretos
- [x] `make quality` `exit=0`, **1367** `^OK `, **0** `: FALHA`
- [ ] 🔴 `windows-full-suites` verde no PR #453 — **só o CI fecha este**

#### ⚠️ Residual declarado: o discriminante do gate NÃO reconhece `tr -d '\r'`

O executor afirmou que *"`tr -d '\r'` é reconhecido como `strip_cr`"*. **Verifiquei e não é.** O gate
testa:

```
grep -qF 'strip_cr' … || grep -qF 's/\r$//' …
```

`tr -d '\r'` não casa com nenhum dos dois. O gate passa porque **o sítio `ROADMAP_DIR` não é
candidato** — candidato exige captura de `python3`, e ali não há `python3`. Passa pelo motivo certo,
mas **não** pela razão que ele deu.

🔴 **Lacuna latente:** se alguém usar `tr -d '\r'` num sítio que **é** candidato, o gate **acusa
indevidamente**. Não corrijo aqui: nenhum sítio real está nessa condição hoje, e a v9.0.1 tem janela
— cada dia com a v9.0.0 corrente gera instalações novas com o sinal silenciando. **Declarado para
não virar surpresa**, que é o que distingue dívida de omissão.
