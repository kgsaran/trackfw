---
status: backlog
date: 2026-09-09
squad: ares-tf
req: "docs/req/REQ-2026-09-01-gate-anti-divergencia-prova-que-as-copias-concordam-mas-nao-que-a-lista-esta-completa-quarta-copia-futura-passa-silenciosa.md"
---

# Roadmap: Gates de paridade provam concordância, mas não completude

> Criado em: 2026-09-09 | Status: backlog

## Context

REQ: `docs/req/REQ-2026-09-01-gate-anti-divergencia-prova-que-as-copias-concordam-mas-nao-que-a-lista-esta-completa-quarta-copia-futura-passa-silenciosa.md`

## A causa, uma frase

**Um gate que compara um conjunto fixo prova que os itens que ele conhece concordam — e nada sobre os
que ele não conhece.** O item novo passa em silêncio, **para sempre**, e o gate segue verde com toda a
confiança.

## Sítios conhecidos

### 1. `check-atomic-write-anti-divergence.sh` — lista fixa de arquivos (da REQ)

```bash
FILES=(pypi/trackfw/identity/__init__.py
       pypi/trackfw/thirdparty/quarantine.py
       pypi/trackfw/integrations/manager.py)
```

Uma **quarta cópia** de `_atomic_write` passaria silenciosamente. Achado do `hades-tf` na barreira da
REQ do `fchmod`.

### 2. `check-cli-parity.sh` — compara só o primeiro nível (issue #298)

**Medido pelo arquiteto em 2026-09-09:**

```
menções a subcomando em check-cli-parity.sh:  ZERO
gate de subcomando:                            não existe

adr      list · new                    2
req      list · move · new             3
roadmap  list · move · new · show      4
                                      ---
                                       9 subcomandos sem gate entre os 3 runtimes
```

🔴 **O autor do issue reportou 12; são 9.** A diferença provável é o `help` que o cobra injeta em cada
grupo — não é subcomando nosso e não deve entrar num gate de paridade. **Corrigir isso no gate importa:**
contar o `help` faria o gate exigir paridade de algo que o framework gera.

**Falsificação dele, com controle:** removeu `req list` só do Node ⇒ `check-cli-parity.sh` saiu
**exit 0** (cego); com a árvore intacta, o gate dele saiu 0 também. O controle é o que separa *"pegou o
defeito"* de *"reprova sempre"*.

**E já nos mordeu:** `req move` faltou nos três e `req list` faltou no Python, **sem nenhum gate
avisar**.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 1 — Completude, não só concordância

### ML-1A — `check-cli-parity` desce um nível, com guarda de reconciliação
**Status:** ⬜ Pendente · **Agente:** `ares-tf`

💡 **O remédio já existe neste repositório.** O `ML-2A` do roadmap do hook
(`scripts/check-git-branch-guard-hook-schema.sh`) resolveu exatamente esta classe: **deriva** a lista
de sítios e **reprova se um sítio derivado não estiver contabilizado**. Use o mesmo padrão — não
invente outro.

🔴 **A lista de comandos-com-subcomando tem de ser DERIVADA**, não literal. O autor do issue declara
essa limitação no gate dele: *"se aparecer um quarto comando, a lista é literal e não descobre
sozinha"*. Herdar a limitação seria fechar o buraco reproduzindo a causa.

**Compare nos dois sentidos** — faltando **e** sobrando. Falsifique **os dois**: o autor falsificou só
"faltando", e declarou isso.

**Excluir o `help` injetado pelo framework**, com o motivo escrito.

### ML-1B — `check-atomic-write-anti-divergence` deriva a lista
**Status:** ⬜ Pendente · **Agente:** `ares-tf`

Mesmo padrão: derivar por assinatura da função em vez de enumerar caminhos.

## Wave 2 — A varredura que fecha a classe
> Dependências: Wave 1.

### ML-2A — Quais outros gates comparam conjunto fixo?
**Status:** ⬜ Pendente · **Agente:** `ares-tf`

São ~48 gates em `scripts/`. **Quantos comparam lista literal?** Levantamento medido, com veredito
por gate: derivável, ou literal-com-motivo.

🔴 **"Literal com motivo escrito" é resultado válido** — o `check-parity-call-site-pins.sh` congelou a
lista **de propósito**, porque a derivação ingênua dava falso positivo num sítio legítimo. O que não
vale é literal **sem** motivo.

## Critérios de Aceite

- [ ] os 9 subcomandos comparados entre os 3 runtimes, nos dois sentidos
- [ ] lista de comandos-com-subcomando **derivada**; quarto comando futuro **não** passa em silêncio
- [ ] `help` do framework excluído, com motivo escrito
- [ ] `check-atomic-write-anti-divergence` deriva a lista
- [ ] levantamento dos ~48 gates: derivável ou literal-com-motivo
- [ ] falsificação nas duas direções em cada gate alterado + guarda de vacuidade
