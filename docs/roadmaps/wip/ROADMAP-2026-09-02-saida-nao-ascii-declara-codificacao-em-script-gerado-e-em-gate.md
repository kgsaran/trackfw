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
**Status:** ⬜ Pendente
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
- [ ] Varredura pelo sintoma, com o método mostrado — não só a minha lista de 40
- [ ] Classificação produto × ferramenta, item a item
- [ ] Veredito sobre saída que alimenta hash/comparação
- [ ] Veredito sobre onde `errors="replace"` esconderia dado
- [ ] Nenhuma linha de correção escrita
- [ ] Parecer em `docs/seguranca/2026-09-02-modelo-de-ameaca-da-saida-nao-ascii.md`

**Gates da wave:**
```bash
test -f docs/seguranca/2026-09-02-modelo-de-ameaca-da-saida-nao-ascii.md
! grep -qi "placeholder" docs/seguranca/2026-09-02-modelo-de-ameaca-da-saida-nao-ascii.md
grep -q "Residual" docs/seguranca/2026-09-02-modelo-de-ameaca-da-saida-nao-ascii.md
```

## Wave 1 — O produto primeiro
> Dependências: Wave 0. **Só o conteúdo gerado e instalado no usuário.** Os gates ficam para a Wave 2:
> misturar faria a correção que atinge usuário esperar pela varredura de ~40 arquivos nossos.

## Wave 2 — Os gates, com correção uniforme e gate contra reintrodução
> Dependências: Wave 1.

## Verificação que só o CI fecha

O item 4 saindo de `REPRODUCED`: camada 2 de **4 para 3**. 🔴 **Verificar o que o check mede antes de
fixar o número** — errei isso duas vezes nesta sessão, e na segunda o check media uma **réplica
dentro do harness**, não o produto.

## Barreira final

`hefesto-tf` e `hades-tf`, auditoria do arquiteto, `barrier`. **CI verde.**
