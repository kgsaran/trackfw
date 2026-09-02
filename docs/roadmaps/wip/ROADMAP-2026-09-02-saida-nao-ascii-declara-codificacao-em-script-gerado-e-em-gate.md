---
status: wip
date: 2026-09-02
req: "docs/req/REQ-2026-09-02-heredoc-python3-com-nao-ascii-morre-em-cp1252-em-40-scripts-e-um-deles-e-instalado-no-usuario.md"
squad: "hades-tf, apolo-tf, artemis-tf"
---

# Roadmap: Saída não-ASCII declara codificação, em script gerado e em gate

> Created: 2026-09-02 | Status: wip

## Context

REQ: `docs/req/REQ-2026-09-02-heredoc-python3-com-nao-ascii-morre-em-cp1252-em-40-scripts-...`

**Item 4 do issue #216 — o último dos onze.** O issue nomeia **um** gate; medi e são **40 scripts**
com heredoc `python3` + não-ASCII e **zero** com `reconfigure`.

**Um deles é produto:** o `attentionSignalScript` (`internal/generators/scaffold.go:757`) é gerado e
escrito em `scripts/trackfw-attention-signal.sh` de quem adota — com `ã ç é ê ó ú — ✓`. **É o
`trackfw init` entregando script quebrado numa máquina Windows.**

## Acceptance Criteria

- [ ] Produto separado de ferramenta, e o produto tratado primeiro
- [ ] Varredura pelo **sintoma de saída**, não pelo heredoc
- [ ] Correção uniforme e verificável por gate
- [ ] Falsificação nas duas direções, **incluindo o controle** de que a saída UTF-8 não muda
- [ ] Gate contra reintrodução
- [ ] Item 4 sai de `REPRODUCED` (camada 2 de 4 → 3)
- [ ] `make quality` e **CI** verdes

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — A varredura pelo sintoma, não pelo mecanismo
> Dependências: nenhuma. Bloqueia a correção.

### ML-0A — Enumerar toda saída não-ASCII, e separar produto de ferramenta
**Status:** ✅ Concluído
**Agente:** `hades-tf`
**Files affected:** nenhum (documento em `docs/seguranca/`)

**Por que a minha lista de 40 é ponto de partida e não escopo:** encontrei-a procurando `python3` +
não-ASCII + ausência de `reconfigure`. **Isso acha quem já usa heredoc Python.** Não acha quem
imprime não-ASCII por `echo`, `printf`, `awk`, `sed`, ou Python invocado de outra forma.

**A varredura tem de ser pelo sintoma — saída não-ASCII — não pelo mecanismo que eu presumi.** Nesta
sessão isso se repetiu: minhas enumerações erraram por uma ordem de grandeza duas vezes, e nas duas
foi você quem achou a população real.

**Actions:**
1. Varrer **toda** saída não-ASCII em `scripts/` e em **conteúdo gerado** pelos 3 CLIs — não só
   heredoc Python.
2. 🔴 **Classificar em duas populações**, porque as urgências são diferentes:
   **(a) produto** — gerado e instalado no usuário; quebra quem adota.
   **(b) ferramenta** — gates nossos; quebra nós.
   Confirme se o `attentionSignalScript` é o **único** de (a), ou se há outros geradores emitindo
   conteúdo com não-ASCII que roda na máquina do usuário.
3. **Modelo de ameaça leve:** o defeito é de disponibilidade, não de confidencialidade. Mas há um
   caso que merece olhar: **saída que alimenta hash ou comparação** — o `CORPUS_HASH` do
   `check-roadmap-barrier-contract.sh:542` faz a codificação **fazer parte do dado**, e um hash que
   varia por SO é **pior que um crash**, porque parece *"o corpus mudou"*. Procure outros pontos
   assim.
4. 🔴 **Falsificação nas duas direções, e a simétrica importa:** `reconfigure(errors="replace")`
   corrige o crash, mas **troca-o por substituição silenciosa** se a saída não for de fato UTF-8.
   Nomeie onde `replace` é aceitável e onde esconderia dado.
5. **Residual declarado.**

**Critérios de aceite:**
- [x] Varredura pelo sintoma, com o método mostrado — não só a minha lista de 40
- [x] Classificação produto × ferramenta, item a item
- [x] Veredito sobre saída que alimenta hash/comparação
- [x] Veredito sobre onde `errors="replace"` esconderia dado
- [x] Nenhuma linha de correção escrita
- [x] Parecer em `docs/seguranca/2026-09-02-modelo-de-ameaca-da-saida-nao-ascii.md`

**Gates da wave:**
```bash
test -f docs/seguranca/2026-09-02-modelo-de-ameaca-da-saida-nao-ascii.md
! grep -qi "placeholder" docs/seguranca/2026-09-02-modelo-de-ameaca-da-saida-nao-ascii.md
grep -q "Residual" docs/seguranca/2026-09-02-modelo-de-ameaca-da-saida-nao-ascii.md
```

#### Resultado do ML-0A (hades-tf, 2026-09-02) — auditado pelo arquiteto

**O escopo real é 2, não 40 — e uma medição minha estava errada.**

### 1. Eu medi a vizinhança e reportei como conteúdo

Afirmei **12** caracteres não-ASCII no `attentionSignalScript`. **É 1**, num comentário morto.
Peguei uma **janela arbitrária de 4000 chars** a partir do índice do nome da variável, capturando o
código Go em volta — e o `scaffold.go` tem comentários acentuados. Extraindo o literal:
`1535 chars, 1 não-ASCII`. **Confirmei.**

Mesma classe que auditei nos outros a sessão inteira: **medir algo próximo do que se quer.**

**A urgência cai:** o risco no produto não é texto estático, é **texto dinâmico do agente** — e já
está amortecido por `2>/dev/null || echo "Agent needs attention"`, que **degrada a mensagem sem
matar o script**.

### 2. `echo` e `printf` do bash nunca estouram por codificação

Verificado sob `LC_ALL=C`. **Só `python3 print()` faz encoding estrito.** Então a varredura ampla por
sintoma — ~700 linhas de `echo` não-ASCII — **converge de volta para os mesmos 40**, mas por razão
melhor que coincidência: **`python3` é o único primitivo capaz de crashar neste código.**

Minha AC2 supunha que a varredura por sintoma acharia mais. Achou **o mesmo**, e explicou **por quê**.

### 3. Dos 40, só 2 têm risco real

| script | classe | situação |
|---|---|---|
| `check-roadmap-barrier-contract.sh` | ferramenta | 🔴 **risco real** — alimenta o `CORPUS_HASH` |
| `check-atomic-write-anti-divergence.sh` | ferramenta | seguro — só stderr de diagnóstico |
| os outros 38 | ferramenta | não-ASCII está no `echo`, não no corpo Python |

**Segundo artefato de produto que a REQ não previu:** `scripts/trackfw-git-branch-guard.sh`, com
**534 bytes** não-ASCII — mas **seguro**, por não invocar `python3`.

### 4. O remédio óbvio é inseguro em dois dos três casos

`errors="replace"` falsificado nas duas direções:

| alvo | veredito |
|---|---|
| os 39 gates de diagnóstico | **seguro** — degradar é melhor que abortar |
| `CORPUS_HASH` | 🔴 **inseguro** — não corrige a não-determinação do hash, **só a torna silenciosa** |
| `attentionSignalScript` | 🔴 **pioraria** — troca um fallback limpo por corrupção calada |

E ele nomeou uma terceira opção que eu não tinha considerado: **`PYTHONUTF8=1`**, cobrindo os 39
**sem editar nenhum**.

### 🔴 Colisão com o PR #238, levantada e não resolvida por mim

**O item de maior risco — o `CORPUS_HASH` — é exatamente o que o PR #238 de
`lourivalgarciajunior` corrige**, e ele está **bloqueado aguardando a governança que pedimos**.

**A Wave 1 não toca aquele arquivo.** Corrigi-lo aqui seria tomar trabalho já feito por ele, enquanto
o seguramos por processo. **Decisão do KG**, não minha.

## Wave 1 — O que NÃO colide com o PR #238
> Dependências: Wave 0. **Escopo reordenado pelo achado:** o alvo de maior risco (`CORPUS_HASH`) é o
> que o PR #238 do reporter corrige, e está bloqueado por decisão de processo. Esta wave cobre o
> resto.

### ML-1A — `attentionSignalScript`: o caminho dinâmico
**Status:** ✅ Concluído
**Agente:** `apolo-tf`
**Files affected:** `internal/generators/scaffold.go`, `npm/src/generators/hooks.js`,
`pypi/trackfw/generators/init_gen.py` (o literal é **byte-idêntico** nos 3)
**Diagnóstico:** o texto estático tem 1 caractere não-ASCII. O risco é **texto dinâmico do agente**,
já amortecido por `2>/dev/null || echo`.
🔴 **Não trocar o fallback por `errors="replace"`** — o ML-0A mediu que isso **pioraria**: troca
degradação limpa por corrupção silenciosa.
**Critérios de aceite:**
- [x] O caminho dinâmico não estoura sob `PYTHONIOENCODING=cp1252`
- [x] 🔴 **Controle:** o fallback atual **continua funcionando** — não pode ser substituído
- [x] Literal segue **byte-idêntico** nos 3 CLIs
- [x] `make quality` verde — `QUALITY_EXIT=0`, re-executado pelo arquiteto (não herdado do relatório)

**Solução:** prefixo `PYTHONIOENCODING=utf-8` nas duas invocações `python3 -c`, por invocação
(`VAR=valor comando`, sem `export` — não vaza para o resto do script). `PYTHONUTF8=1`/`-X utf8` foi
falsificado e **perde**: é ignorado quando `PYTHONIOENCODING` já vem setado no ambiente — que é
exatamente o método de simulação de console cp1252 adotado no projeto (`TestCliEmConsoleCp1252`,
\#223). O fallback `2>/dev/null || echo` foi preservado; `errors="replace"` não foi usado.

**Evidência de aceite — auditoria do arquiteto (2026-09-02), medida com o script realmente gerado
por `trackfw init` e o ramo `jq` desativado por `if false`:**

```
antes   cp1252  "Área crítica"  ->  Agent needs attention   ← perde a mensagem
antes   utf-8   "Área crítica"  ->  Área crítica
depois  cp1252  "Área crítica"  ->  Área crítica            ← corrige
depois  utf-8   "Área crítica"  ->  Área crítica            ← controle: saída UTF-8 não muda
depois  cp1252  JSON malformado ->  Agent needs attention   ← controle: fallback intacto
```

Paridade: `scripts/check-attention-scripts-parity.sh` com `GO_BIN` recompilado da árvore → exit 0,
8/8 `OK` (compara Node e Python de verdade, não regex sobre a fonte).

🔴 **Correção de evidência — o cenário do relatório do agente não discrimina.** O relatório provou a
direção 1 com `confirmação ✓`; **isso não reproduz**: antes e depois devolvem `confirmação ✓`
idêntico. Os 3 bytes de `✓` (`E2 9C 93`) são todos definidos em cp1252, então decodificar UTF-8 como
cp1252 e re-codificar é **round-trip byte-a-byte**. O gargalo não é o encode do stdout (isso só
valeria para um literal no código) — é o **decode do stdin**, e só quebra com um byte **indefinido**
em cp1252: `Á` = `C3 81`. O fix é correto; a evidência escolhida é que não media o que se pensava.
Registrado em `vault/notes/cp1252-roundtrip-mascara-o-defeito-o-discriminante-e-decode-de-stdin-2026-09-02.md`.

🔴 **O residual declarado pelo agente está invertido.** Ele registrou que entrada genuinamente
cp1252 "agora falha para o fallback em vez de imprimir algo". Medido: **antes**, `tr` morre com
`Illegal byte sequence` e o `set -euo pipefail` mata o script — **nenhum** `.trackfw-attention.json`
é escrito; **depois**, grava `"Agent needs attention"`. É melhora, não regressão.

### ML-1B — `PYTHONUTF8=1` para os gates de diagnóstico
**Status:** ✅ Concluído
**Agente:** `artemis-tf`
**Diagnóstico:** o ML-0A propôs cobrir os 39 **sem editar nenhum**. Avaliar onde declarar —
`Makefile`, wrapper, ou workflow — e **justificar**.
🔴 **Não aplicar ao `check-roadmap-barrier-contract.sh`** enquanto o #238 estiver aberto.
🔴 **E `PYTHONUTF8=1` não conserta o `CORPUS_HASH`** — o problema lá é o hash depender da
codificação, não a saída estourar.
**Critérios de aceite:**
- [x] Os gates de diagnóstico não estouram sob console cp1252, verificado por execução — **42 de 43**
- [x] 🔴 **Controle:** a saída em terminal UTF-8 **continua idêntica**
- [x] O `check-roadmap-barrier-contract.sh` **não** é tocado
- [x] `make quality` verde — `QUALITY_EXIT=0`, zero `FAIL` em 3.439 linhas, re-executado pelo arquiteto

**Mecanismo — o do roadmap foi falsificado e trocado.** `export PYTHONIOENCODING=utf-8` declarado
**dentro de cada gate** (37 arquivos), não `PYTHONUTF8=1`. Medido em Python 3.14.7:

```
PYTHONIOENCODING=cp1252 python3           -> stdout=cp1252
PYTHONIOENCODING=cp1252 python3 -X utf8   -> stdout=cp1252   <- PYTHONUTF8 NAO muda o stdio
PYTHONIOENCODING=utf-8  python3           -> stdout=utf-8
```

`PYTHONUTF8` governa `locale.getpreferredencoding()` (o `open()` sem `encoding=`); é **ignorado no
stdio** quando `PYTHONIOENCODING` vem do ambiente — que é o método de simulação de console cp1252
adotado no projeto (#223). Com ele, "verificado por execução" seria inalcançável. **As duas variáveis
cobrem superfícies diferentes; não são alternativas.** Eu escrevi a errada no roadmap antes de o
ML-1A medir.

**Por que dentro de cada gate e não no `Makefile`:** o `Makefile` cobre só `make parity`. Ficariam de
fora a invocação direta pelos workflows (`quality.yml:25,26,54`, `release.yml:57-59`), a invocação
manual de um gate isolado, e a invocação de gate por gate — inclusive as **cópias sandboxadas** que o
`check-gates-falsify.sh` cria, onde um `source scripts/lib/…` relativo quebraria.

**Evidência de aceite — auditoria do arquiteto (2026-09-02), reproduzida de forma independente com
cópias "antes" mantidas dentro de `scripts/` para não quebrar a resolução de `ROOT_DIR`:**

```
antes  cp1252  check-parity-contract-coverage  -> rc=1  UnicodeEncodeError '\u2192' (→), <stdin> linha 332
antes  utf-8   check-parity-contract-coverage  -> rc=0                                  ← controle
depois cp1252  check-parity-contract-coverage  -> rc=0                                  ← corrige

antes  cp1252  check-barrier                   -> rc=1  failure message mismatch
                                                  want [... origin/main — pass ...]
                                                  got  [... origin/main 0x97 pass ...]
depois cp1252  check-barrier                   -> rc=0  todos os cenários passam
```

Controle byte-a-byte em UTF-8, antes × depois: `check-parity-contract-coverage` idêntico;
`check-barrier` diverge **apenas** na ordem das linhas `go: downloading …` (não-determinismo do
módulo), idêntico após ordenar.

🔴 **O segundo modo de falha é o que não estava no meu diagnóstico.** O `check-barrier.sh` **não
crasha**: `—` (U+2014) **é definido** em cp1252 e sai como o byte `0x97`; o bash compara com o
literal UTF-8 e reprova com *"failure message mismatch"* — uma mensagem plausível **sobre o produto**,
quando a causa é a codificação do canal entre o `python3` do gate e o `bash`. Registrado em
`vault/notes/gate-em-cp1252-tem-duas-falhas-distintas-crash-de-print-e-mismatch-por-transcodificacao-2026-09-02.md`.

🔴 **Achado que corrige o AC: o `check-roadmap-barrier-contract.sh` também estoura.** Não foi tocado
(proibição do #238 respeitada — `grep PYTHONIOENCODING` → 0), mas foi **executado**:

```
utf-8   -> rc=0, 53 cenários OK
cp1252  -> rc=1, UnicodeEncodeError '\u2705' (✅), <stdin> linha 7 = linha 523 do arquivo
```

A linha 523 está no heredoc que escreve o `$CORPUS_LINES_FILE` — o mesmo arquivo cujo sha vira o
`CORPUS_HASH` da linha 542. **É o mesmo sítio do defeito do #238, com sintoma diferente:** sob cp1252
o gate morre *antes* de o hash divergir. Forçar a codificação mataria o crash e **não** tornaria o
hash independente do SO. Por isso o AC lê **42 de 43**, não "`make parity` inteiro".

**Trade-off assumido, escrito no comentário de cada gate:** console cp1252 real passa a exibir
**mojibake em vez de crashar**. Para um gate de diagnóstico é a troca certa — acento ilegível com
exit code correto vale mais que reprovação falsa.

**Residuais declarados:** (1) `open()` sem `encoding=` em 5 gates não é coberto nem simulável aqui;
(2) **nenhum job de CI roda `scripts/check-*.sh` no Windows** (`parity` é `ubuntu-latest`,
`quality.yml:409`) — a exposição real é a invocação manual; (3) o
`windows-repro/.../cmd_cp1252_print` mede `print('→')` **em isolamento** e não invoca o `.sh`, então
o veredito daquele instrumento **não muda** — verificar o que o check mede antes de fixar
"camada 2 de 4 → 3"; (4) sem gate anti-reintrodução — é a Wave 2.

**Paridade 3 CLIs:** infra de gate, exceção explícita do contrato. Nada em `internal/`, `npm/src/`,
`pypi/trackfw/`; `check-parity-contract-coverage.sh` verde.

## Wave 2 — Gate contra reintrodução
> Dependências: Wave 1 (fechada — ML-1A `5b5391e`, ML-1B `6721078`, barreira da Wave 1 `passed`).

**Escopo revisado depois da Wave 1.** O título original dizia "correção uniforme **e** gate contra
reintrodução". A correção uniforme já saiu inteira no ML-1B (37 gates). Sobra o gate — e a Wave 1
mostrou que ele tem **dois alvos**, não um.

### ML-2A — O gate que impede a regressão, nos dois alvos
**Status:** ✅ Concluído
**Agente:** `artemis-tf`
**Files affected:** `scripts/check-output-encoding-declared.sh` (novo), `Makefile`,
`.github/workflows/quality.yml`, `docs/cli-parity.md` (registrar a exceção de infra de gate)

**Alvo 1 — ferramenta.** Todo `scripts/check-*.sh` que invoca `python3` declara
`export PYTHONIOENCODING=utf-8`. Exceção **única e nomeada**: `check-roadmap-barrier-contract.sh`,
com o motivo escrito (PR #238 aberto sobre o mesmo sítio). Allowlist explícita, nunca skip silencioso.

**Alvo 2 — produto, e é o que a paridade NÃO cobre.** O literal `attentionSignalScript` dos 3 CLIs
tem de conter o prefixo `PYTHONIOENCODING=utf-8`. O `check-attention-scripts-parity.sh` compara os 3
entre si: se alguém remover o prefixo **dos três**, ele continua verde. Paridade mede se as
implementações concordam, não se o contrato está correto — o mesmo cego já registrado em
`vault/notes/barrier-so-casa-cabecalho-de-aceite-em-portugues-2026-08-28.md`.

🔴 **Regex literal é evadível — leia `vault/notes/gate-literal-regex-syntax-equivalent-bypass-2026-09-01.md`
antes de escrever a asserção.** `PYTHONIOENCODING="utf-8"`, `='UTF-8'`, `export  PYTHONIOENCODING=utf-8`
e `PYTHONIOENCODING=utf8` são semanticamente equivalentes ou próximas. Decida e **documente** quais
formas aceita e quais recusa; uma regex literal ingênua reprova quem está certo e passa quem está
errado.

🔴 **Guarda de vacuidade obrigatória.** Se a varredura enumerar **zero** gates, o check **falha** —
não passa em silêncio. Um `glob` que deixa de casar é a forma mais comum de um gate virar decorativo.

🔴 **O próprio gate novo invoca `python3`?** Se sim, ele se aplica a si mesmo — e isso tem de ser
verdade por execução, não por boa intenção.

**Critérios de aceite:**
- [x] Alvo 1 falsificado nas duas direções: remover a declaração de um gate qualquer → check **reprova**
      nomeando o arquivo; árvore íntegra → check **passa**
- [x] Alvo 2 falsificado nas duas direções: remover o prefixo do literal **nos 3 CLIs** → check
      **reprova**; árvore íntegra → check **passa**
- [x] 🔴 Guarda de vacuidade falsificada: varredura vazia → check **falha**, verificado por execução
- [x] `check-roadmap-barrier-contract.sh` na allowlist **com motivo escrito**, e o arquivo segue intocado
- [x] O check está ligado ao `make quality` **e** ao workflow — verificado, não presumido
- [x] `make quality` verde


**Entregue:** `scripts/check-output-encoding-declared.sh`, ligado ao alvo `parity:` do `Makefile`
(que o `quality.yml:445` executa via `make parity`). Sem `- run:` avulsa no workflow: a
`REQ-2026-08-04` removeu deliberadamente a lista manual parcial, e reintroduzi-la desfaria aquela
decisão.

**Evidência de aceite — auditoria do arquiteto (2026-09-02), reproduzida de forma independente:**

*Alvo 2, o cego da paridade — prefixo removido dos **3** geradores, `GO_BIN` recompilado da árvore
modificada:*

```
check-attention-scripts-parity     rc=0   <- 8/8 OK: VERDE sobre a regressao
check-output-encoding-declared     rc=1   <- nomeia os 3 arquivos e as 6 linhas
```

A mensagem de falha explica o próprio cego em linha. Geradores restaurados; `git status` confirma
idênticos ao HEAD.

*Alvo 1, duas direções:* declaração removida de `check-attention-scripts-parity.sh` → reprova com
`invoca python3 (linha 117) e NAO declara ...`; restaurado → `rc=0`.

*Vacuidade, por execução:* glob apontado para padrão inexistente numa cópia →
`ALVO 1 vacuo: ... enumerou ZERO arquivos (cwd=...). Recuso passar em silencio.` São **três**
guardas — glob vazio, população de invocadores vazia, e **auto-aplicação** (o gate reprova se ele
próprio sumir da população).

*Baseline:* `45 scripts/check-*.sh enumerados, 38 invocam python3, 1 na allowlist, 37 checados` +
`2/2 invocações com prefixo` em cada um dos 3 geradores. `make quality` → exit 0, zero `FAIL`.
`grep -c PYTHONIOENCODING scripts/check-roadmap-barrier-contract.sh` → 0, intocado.

**Cobertura declarada como `partial=`, não `gate=`:** a asserção é **estática**. Prova que a
declaração existe, é exportada, tem valor alias de `utf_8` e precede a primeira invocação; **não**
prova por runtime que aquele `python3` enxergou UTF-8. Provar comportamentalmente exigiria executar
os 38 gates com `python3` instrumentado, e dois deles inviabilizam isso dentro de `make parity`
(`check-gates-falsify` ~3m05s; `check-barrier` executa git). O mecanismo foi provado por execução
uma vez, com stub. Honestidade preferida a um `gate=` que promete mais do que mede.

**Correção aplicada pelo arquiteto no fecho da wave:** `scripts/trackfw-attention-signal.sh` — a
cópia versionada e *dogfooded* neste repo — estava sem o prefixo desde o ML-1A. Regenerada pelo
binário da árvore atual; o diff contra o gerador é agora **vazio**, e a divergência era exatamente
as 2 linhas do ML-1A. Estávamos entregando o conserto para os adotantes com o nosso próprio harness
ainda quebrado sob cp1252.

🔴 **O que continua descoberto — vira REQ, não este ML:** *nada* compara essa cópia versionada com o
literal do gerador. O `check-attention-scripts-parity.sh` roda em `mktemp -d` e o
`scaffold_parity_test.go` faz `os.Chdir(t.TempDir())` antes do `ReadFile` — os dois olham para uma
cópia efêmera, nunca para o arquivo no repo. Auditar pelo arquivo de nome mais óbvio dá o veredito
errado. Registrado em
`vault/notes/copia-versionada-do-attention-signal-esta-obsoleta-e-sem-guarda-2026-09-02.md`.

**Comandos de validação:** `bash scripts/check-output-encoding-declared.sh`, `make quality`

## Verificação que só o CI fecha

O item 4 saindo de `REPRODUCED`: camada 2 de **4 para 3**. 🔴 **Verificar o que o check mede antes de
fixar o número** — errei isso duas vezes nesta sessão, e na segunda o check media uma **réplica
dentro do harness**, não o produto.

## Barreira final

`hefesto-tf` e `hades-tf`, auditoria do arquiteto, `barrier`. **CI verde.**
