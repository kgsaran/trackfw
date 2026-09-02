---
status: Open
date: 2026-09-02
author: "zeus-tf"
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-02-saida-nao-ascii-declara-codificacao-em-script-gerado-e-em-gate.md"
---

# REQ: Heredoc `python3` com não-ASCII morre em cp1252 — em 40 scripts, e um deles é instalado no usuário

> Date: 2026-09-02 | Status: Open

## Motivation

**Item 4 do issue #216** — o último dos onze. O issue nomeia **um** gate
(`scripts/check-parity-contract-coverage.sh`). **Medi, e a população é outra:**

```
40 scripts com heredoc python3 + caracteres não-ASCII
 0 com sys.stdout.reconfigure
```

Em console cp1252 — o padrão do Windows —, `print()` de `→`, `✓`, `relatório` ou `seções` estoura
com `UnicodeEncodeError` e o script morre.

### O que reenquadra a REQ: um deles é produto, não ferramenta nossa

```
internal/generators/scaffold.go:757   attentionSignalScript
                                      "canonical content of the attention-signal hook"
        ↓ escrito em scripts/trackfw-attention-signal.sh de quem adota
        12 caracteres não-ASCII: ã ç é ê ó ú — ✓
        usa python3, sem reconfigure
```

**Não é chateação da nossa suíte de gates: é `trackfw init` entregando um script quebrado na máquina
de um usuário Windows.** Os outros 39 são gates de desenvolvimento — incômodo real, mas de outra
classe.

### O reporter achou uma segunda instância sem saber que havia 38

O **PR #238** de `lourivalgarciajunior` corrige o `check-roadmap-barrier-contract.sh` — **1 dos 40**.
O comentário dele traz uma segunda razão que eu não teria visto: além do crash, a codificação
**entraria no `CORPUS_HASH`**, fazendo o mesmo corpus dar hash diferente por SO. Confirmei na linha
542.

## Acceptance Criteria

- [ ] **AC1** — 🔴 **Separar produto de ferramenta, e tratar primeiro o produto.** O
      `attentionSignalScript` é gerado e instalado; os 39 gates são nossos. **São urgências
      diferentes** — um quebra usuário, os outros quebram nós. Misturar faria a correção do usuário
      esperar pela varredura dos 39.
- [ ] **AC2** — 🔴 **A varredura é pelo primitivo, não pelo sintoma.** Encontrei os 40 procurando
      `python3` + não-ASCII + ausência de `reconfigure`. **Isso acha quem já usa heredoc; não acha
      quem imprime não-ASCII por outro caminho** (`echo`, `printf`, `awk`, ou Python invocado de
      outra forma). A Wave 0 varre por **saída não-ASCII**, não por heredoc.
- [ ] **AC3** — Correção uniforme e **verificável por gate**, não script a script na mão. 40
      correções manuais divergem na primeira manutenção.
- [ ] **AC4** — 🔴 **Falsificação nas duas direções.** (a) sob `PYTHONIOENCODING=cp1252`, o script
      corrigido **não** estoura — reproduzível em **qualquer SO**, como o
      `TestCliEmConsoleCp1252` do #223 já demonstrou; (b) **controle:** a saída em terminal UTF-8
      **continua idêntica**. `errors="replace"` sem o controle trocaria crash por mojibake silencioso.
- [ ] **AC5** — Gate impedindo reintrodução. **Nasce ligado ao `Makefile`, com guarda de vacuidade
      ancorada no mesmo cwd, `python3` nunca `python`, e `assert_count` onde a assinatura repetir** —
      são ~40 pontos, e o gate precisa reprovar se **um** regredir.
- [ ] **AC6** — O achado do `CORPUS_HASH` endereçado: onde a saída alimenta hash, a codificação
      **faz parte do dado**. Um hash que varia por SO é pior que um crash — crash é barulhento, hash
      divergente parece *"o corpus mudou"*.
- [ ] **AC7** — 🔴 **O item 4 sai de `REPRODUCED` na camada 2** (de 4 para 3). **Verificar o que o
      check mede antes de fixar o número** — errei isso duas vezes nesta sessão, e na segunda o check
      media uma réplica dentro do harness, não o produto.
- [ ] **AC8** — `make quality` e **CI** verdes.

## Negative Scope

- **Não** mergear o PR #238 dentro desta REQ. Ele está **bloqueado aguardando governança**, a pedido
  do KG, e corrige 1 dos 40 — a decisão sobre ele é separada.
- **Não** alterar as mensagens em si. O defeito é a **codificação da saída**, não o texto; remover os
  acentos seria tratar o sintoma e empobrecer as mensagens.
- **Não** tocar o `_force_utf8_output()` do CLI Python (item 1, já corrigido). É o caminho do
  `main()`; estes scripts nunca passam por lá — foi por isso que o item 4 sobreviveu ao #223.

## Linked ADR

ADR: <!-- avaliar na Wave 0: se a conclusão for que todo script gerado ou versionado deve declarar
codificação de saída, isso é contrato de projeto e merece registro em docs/cli-parity.md. -->

## Linked Roadmap

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-09-02-saida-nao-ascii-declara-codificacao-em-script-gerado-e-em-gate.md`
