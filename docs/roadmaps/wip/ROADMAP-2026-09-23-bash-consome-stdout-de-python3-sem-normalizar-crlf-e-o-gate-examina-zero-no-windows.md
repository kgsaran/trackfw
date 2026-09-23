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

- [ ] Enumeração real dos sítios, classificada (a) não normaliza · (b) normaliza · (c) não consome
- [ ] Todo (a) corrigido por **ponto único**, não por `tr -d '\r'` espalhado
- [ ] Gate falsificável que reprova consumo novo sem normalização
- [ ] `windows-census.yml` volta a dar **8/8 shards** e apuração sem `TOTAL INCOMPLETO`
- [ ] Falsificação exercitada **no Windows**
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
**Status:** ⬜ Pendente · **Papel:** `apolo-tf`

🔴 **Ponto único, não `tr -d '\r'` espalhado.** Foi cópia de helper que originou a REQ-2026-08-31, e
o issue #401 registra três cópias byte-idênticas do mesmo padrão ainda abertas. Não crie a quarta.

Considere um helper em `scripts/` que os gates consumam (`lib` sourceada, ou wrapper de invocação do
Python que já normaliza). A escolha é sua; o **ponto único** não é negociável.

⚠️ **Não normalize conteúdo.** Se um gate inspeciona um arquivo CRLF **como dado** (ex.: verifica
que o arquivo tem CRLF), comer o `\r` destrói a verificação. A Wave 0 entrega essa distinção.

**Acceptance criteria:**
- [ ] Ponto único criado e consumido por todos os sítios (a)
- [ ] Teste **load-bearing** por sítio: falha com manifesto/lista CRLF antes, passa depois
- [ ] Braço (b): em POSIX, nada muda — os gates continuam medindo o que mediam
- [ ] `go build ./...` RC=0 · gates tocados RC=0
- [ ] 🔴 **NÃO rodar `make quality`** — barreira é do arquiteto
- [ ] Uma frase por teste novo

### ML-1B — gate que impede a reintrodução
**Status:** ⬜ Pendente · **Papel:** `artemis-tf` · **Depende de:** ML-1A

Gate que **reprova** quando um consumo novo de saída de `python3` nascer sem passar pelo ponto único.

🔴 **Aprendizado direto do gate de contenção (REQ-2026-08-31):** um gate que aceita o sítio por
**marcador textual** afirma sem provar — 34 defeitos passaram sob luz verde. Prefira um discriminante
**estrutural** (o sítio chama o helper? o pipe passa pelo wrapper?) a um comentário de autoria.

**Acceptance criteria:**
- [ ] Gate reprova consumo novo sem normalização — prove injetando
- [ ] Gate passa na árvore correta, com guarda de **não-vacuidade** (piso de sítios examinados)
- [ ] Falsificação com **rótulos literais**, colhidos pela guarda de conjunto
- [ ] 🔴 **NÃO rodar `make quality`**

---

## Wave 2 — A prova no Windows
> Dependências: Wave 1 completa. **É a wave que fecha a REQ.**

### ML-2A — o censo volta a produzir número
**Status:** ⬜ Pendente · **Papel:** `artemis-tf`

🔴 **O AC que importa, e não é local:** disparar `windows-census.yml` (`workflow_dispatch`) e obter
**8/8 shards** com apuração **sem** `TOTAL INCOMPLETO`.

⚠️ **VM investiga, CI mede.** A VM é ARM64 e o runner é x64 — número medido na VM carrega a ressalva
"pode ser artefato desta VM", que já bloqueou uma triagem de 512 falhas. O número que vira afirmação
sai do runner.

⚠️ O run de referência **pré-correção** é o `35856380122` (2026-09-23): 2/8 shards, 146 rótulos
ausentes. Compare contra ele — mesmo instrumento, mesma plataforma.

**Acceptance criteria:**
- [ ] `windows-census.yml` com **8/8 shards** e total apurado
- [ ] A comparação é contra o run `35856380122`, **não** contra o de 2026-09-10 (pré-v8, mede outra
      coisa: a remoção de Node/Python)
- [ ] O número obtido entra no roadmap como **linha de base pós-v8** — e **não** é usado para triar o
      cluster de Windows nesta REQ (escopo negativo)

## Barreira final

Revisão `hefesto-tf` e `hades-tf`, auditoria do arquiteto, `trackfw barrier`. **CI verde.**

⚠️ Custo de CPU: `TRACKFW_FALSIFY_JOBS=4` na barreira local, teste do pacote tocado nos handoffs —
`vault/notes/carga-de-cpu-vem-da-suite-de-falsificacao-vezes-agentes-paralelos-2026-09-22.md`.
