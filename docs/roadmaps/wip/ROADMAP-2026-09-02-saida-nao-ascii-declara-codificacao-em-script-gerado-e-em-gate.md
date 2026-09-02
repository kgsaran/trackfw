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
**Status:** ⬜ Pendente
**Agente:** `apolo-tf`
**Files affected:** `internal/generators/scaffold.go`, `npm/src/generators/hooks.js`,
`pypi/trackfw/generators/init_gen.py` (o literal é **byte-idêntico** nos 3)
**Diagnóstico:** o texto estático tem 1 caractere não-ASCII. O risco é **texto dinâmico do agente**,
já amortecido por `2>/dev/null || echo`.
🔴 **Não trocar o fallback por `errors="replace"`** — o ML-0A mediu que isso **pioraria**: troca
degradação limpa por corrupção silenciosa.
**Critérios de aceite:**
- [ ] O caminho dinâmico não estoura sob `PYTHONIOENCODING=cp1252`
- [ ] 🔴 **Controle:** o fallback atual **continua funcionando** — não pode ser substituído
- [ ] Literal segue **byte-idêntico** nos 3 CLIs
- [ ] `make quality` verde

### ML-1B — `PYTHONUTF8=1` para os gates de diagnóstico
**Status:** ⬜ Pendente
**Agente:** `artemis-tf`
**Diagnóstico:** o ML-0A propôs cobrir os 39 **sem editar nenhum**. Avaliar onde declarar —
`Makefile`, wrapper, ou workflow — e **justificar**.
🔴 **Não aplicar ao `check-roadmap-barrier-contract.sh`** enquanto o #238 estiver aberto.
🔴 **E `PYTHONUTF8=1` não conserta o `CORPUS_HASH`** — o problema lá é o hash depender da
codificação, não a saída estourar.
**Critérios de aceite:**
- [ ] Os gates de diagnóstico não estouram sob console cp1252, verificado por execução
- [ ] 🔴 **Controle:** a saída em terminal UTF-8 **continua idêntica**
- [ ] O `check-roadmap-barrier-contract.sh` **não** é tocado
- [ ] `make quality` verde

## Wave 2 — Os gates, com correção uniforme e gate contra reintrodução
> Dependências: Wave 1.

## Verificação que só o CI fecha

O item 4 saindo de `REPRODUCED`: camada 2 de **4 para 3**. 🔴 **Verificar o que o check mede antes de
fixar o número** — errei isso duas vezes nesta sessão, e na segunda o check media uma **réplica
dentro do harness**, não o produto.

## Barreira final

`hefesto-tf` e `hades-tf`, auditoria do arquiteto, `barrier`. **CI verde.**
