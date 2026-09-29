---
status: wip
date: 2026-09-29
req: "docs/req/REQ-2026-09-29-o-ratchet-de-windows-aperta-numa-direcao-so-e-a-lista-de-falhas-conhecidas-nunca-colhe-a-melhoria.md"
squad: ""
---

# Roadmap: o ratchet de Windows aperta numa direção só

> Created: 2026-09-29 | Status: wip

## Context
<!-- Derived from REQ -->
REQ: docs/req/REQ-2026-09-29-o-ratchet-de-windows-aperta-numa-direcao-so-e-a-lista-de-falhas-conhecidas-nunca-colhe-a-melhoria.md
ADR: docs/adr/ADR-2026-09-05-o-ci-de-windows-bloqueia-por-conjunto-de-nomes-e-por-tipo-de-evento-nunca-por-contagem.md (**Emenda 1**, D6 e D7)
Origem: **#364**. Medido: 1677 PASS · 14 FAIL · **0 obsoletas** — a lista está exata, e é isso que
torna a adoção do D6 gratuita agora.

## Acceptance Criteria
<!-- Consolidados; detalhe por ML nas waves. -->
- [ ] D6: falha declarada que resolve **reprova**, nomeando a entrada e o protocolo do D4
- [ ] Falsificação nas duas direções + contra-braço: regressão nova **continua** reprovando
- [ ] D7: as 14 entradas ganham a razão, e o checker **valida** o campo
- [ ] Triagem das 14 por causa-raiz, **medida**
- [ ] `make quality` e CI verdes, **incluindo o `windows-full-suites`**

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat model: por que cada uma das 14 falha, e o que o checker já garante
> Dependências: nenhuma. 🔴 **Bloqueia a implementação.**

**Gates da wave:**

```bash
n=$(jq -r '.entries[]|.name' .github/windows-known-failures.json 2>/dev/null | wc -l | tr -d ' '); test "$n" = "14" && echo "Gate W0: $n entradas na lista — a triagem parte deste universo" || { echo "GATE FALHOU: esperava 14 entradas, contou $n — a lista mudou, remeça a medicao antes de triar" >&2; exit 1; }
```

### ML-0A — triar as 14 por causa-raiz e mapear o checker
**Owner:** `hades-tf`
**Status:** ⬜ Pendente
**Arquivos:** leitura de `.github/`, `scripts/`; escrita em `docs/seguranca/2026-09-29-wave0-ratchet.md`

**Tarefa:**
1. 🔴 **Para CADA uma das 14, a razão MEDIDA da falha** — não presumida. O log do
   `windows-full-suites` da `main` tem a mensagem de cada uma; extraia e classifique por mecanismo
   (permissão POSIX · CRLF · `bash` ausente · outro). **Agrupe:** é provável que 14 nomes tenham
   poucas causas.
2. **Mapeie o checker** que implementa o ratchet: onde compara declarado × observado, onde emite
   `[-1 resolvido]`, e o que já valida do esquema (`_meta.d4_note` descreve o protocolo de remoção).
3. 🔴 **Refute ou confirme a premissa do D6:** que dá para reprovar por "entrada resolveu" **sem**
   falso positivo. Ataque: teste que **não executa** (skip) conta como resolvido? E teste cujo
   **nome mudou**? O **D3** trata suíte que não executa como falha de classe própria e o **D5**
   declara nomes instáveis — verifique se esses dois já cobrem os buracos, ou se o D6 abre um novo.
4. **Onde o campo de razão cabe** no esquema sem quebrar o checker nem o `removed[]`.

**Critérios de aceite:**
- [ ] As 14 com razão **medida** e citação da mensagem de falha, nenhuma sem evidência
- [ ] Agrupamento por causa-raiz, com a contagem por grupo
- [ ] Mapa do checker: onde comparar, onde emitir, o que já é validado
- [ ] 🔴 Veredito sobre o D6 **antes** de alguém escrever código: skip e rename produzem falso
      positivo? Se sim, o D6 precisa de recorte — e ele é escrito **agora**, não depois
- [ ] Proposta de esquema para o campo de razão, compatível com `entries[]` e `removed[]`

## Wave 1 — o gatilho e a razão
> Dependências: **Wave 0 auditada.** O desenho sai dela — se o D6 precisar de recorte, este bloco muda.

### ML-1A — (a definir pela Wave 0)
**Status:** ⬜ Pendente
Placeholder consciente. 🔴 **Não despachar antes da Wave 0 auditada.**

## Wave 2 — auditoria independente
> Dependências: Wave 1 completa.

### ML-2A — revisão por reimplementação
**Owner:** `hades-tf`
**Status:** ⬜ Pendente
**Método:** 🔴 **não conferir o diff.** Ler ADR e REQ, derivar o esperado, medir o comportamento.
**Critérios de aceite:**
- [ ] Entrada que resolve → reprova · lista fiel → passa · **regressão nova continua reprovando**
- [ ] Skip e rename **não** produzem falso positivo
- [ ] Veredito: sobrou caminho pelo qual uma melhoria passe despercebida?
